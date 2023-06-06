package wwhttp

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"net/http"
	"sync"
	"time"
)

// Getter is a helper for retrieving & caching an ETagged http response.
type Getter struct {
	log           zerolog.Logger
	url           string
	checkInterval time.Duration
	etag          string
	lastChecked   time.Time
	mutex         *sync.Mutex
	cache         interface{}
}

type GetterConfig struct {
	Log           zerolog.Logger
	Url           string
	CheckInterval time.Duration
}

func NewGetter(config GetterConfig) *Getter {
	res := &Getter{
		log:           config.Log,
		url:           config.Url,
		checkInterval: time.Second * 30,
		etag:          "",
		lastChecked:   time.Time{},
		mutex:         &sync.Mutex{},
		cache:         nil,
	}
	if config.CheckInterval != 0 {
		res.checkInterval = config.CheckInterval
	}
	return res
}

func (pg *Getter) GetJson(ctx context.Context, v interface{}) error {
	now := time.Now()
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	// If we have only recently retrieved it, just return it.
	if pg.cache != nil && pg.lastChecked.Add(pg.checkInterval).After(now) {
		pg.log.Debug().Msgf("Not time to get fresh %s yet", pg.url)
		v = pg.cache
		return nil
	}

	// Build Request.
	req, err := http.NewRequestWithContext(ctx, "GET", pg.url, nil)
	if err != nil {
		return err
	}

	// If we have an ETag & a cache, send the If-None-Match.
	if pg.cache != nil && pg.etag != "" {
		req.Header.Set("If-None-Match", pg.etag)
		pg.log.Trace().Msgf("Sending ETag for %s: %s", pg.url, pg.etag)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: 30 * time.Second,
		},
	}

	// Get response.
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check response.
	if resp.StatusCode == 304 {
		// Etag matches, just return the cached version.
		pg.log.Debug().Msgf("304 not modified for %s, using cached", pg.url)
		pg.lastChecked = now
		return nil
	}

	// If there are any issues from here, fallback to cached if possible instead
	// of dying.
	err = func() error {
		if resp.StatusCode != 200 {
			return errors.Errorf("HTTP Error %d", resp.StatusCode)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			return errors.Errorf("Invalid content-type %s", resp.Header.Get("Content-Type"))
		}

		// Decode json.
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return errors.Wrapf(err, "failed to decode %s", pg.url)
		}

		// Store with ETag & time so we can save some time & bandwidth next time.
		pg.cache = v
		pg.etag = resp.Header.Get("ETag")
		pg.lastChecked = now

		// Done.
		pg.log.Info().Msgf("Downloaded fresh %s", pg.url)
		pg.log.Debug().Msgf("%s ETag: %s", pg.url, pg.etag)
		return nil
	}()
	if err != nil {
		if v != nil {
			pg.log.Warn().Msgf("Caught error getting %s, falling back to cached: %s", pg.url, err)
		} else {
			return err
		}
	}

	return nil
}
