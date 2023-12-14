package wwauth

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"github.com/weavingwebs/wwgo/wwhttp"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Turnstile struct {
	siteKey        string
	secretKey      string
	trustedDomains []string
	log            zerolog.Logger
}

func NewTurnstileFromEnv(log zerolog.Logger) *Turnstile {
	siteKey := os.Getenv("TURNSTILE_SITE_KEY")
	if siteKey == "" {
		panic("TURNSTILE_SITE_KEY not set")
	}
	secretKey := os.Getenv("TURNSTILE_SECRET_KEY")
	if secretKey == "" {
		panic("TURNSTILE_SECRET_KEY not set")
	}
	trustedDomains := wwgo.ArrayFilterStr(wwgo.MapSlice(strings.Split(os.Getenv("TURNSTILE_TRUSTED_DOMAINS"), ","), strings.TrimSpace))
	if len(trustedDomains) == 0 {
		panic("TURNSTILE_TRUSTED_DOMAINS not set")
	}

	return &Turnstile{
		siteKey:        siteKey,
		secretKey:      secretKey,
		trustedDomains: trustedDomains,
		log:            log,
	}
}

func (t *Turnstile) SiteKey() string {
	return t.siteKey
}

type TurnstileVerifyResponse struct {
	Success     bool       `json:"success"`
	ChallengeTs *time.Time `json:"challenge_ts"`
	Hostname    *string    `json:"hostname"`
	ErrorCodes  []string   `json:"error-codes"`
	Action      *string    `json:"action"`
	CData       *string    `json:"cdata"`
}

func (t *Turnstile) VerifyToken(ctx context.Context, token string, expectedAction string) (bool, error) {
	// https://developers.cloudflare.com/turnstile/get-started/server-side-validation/
	postData := url.Values{
		"secret":   {t.secretKey},
		"response": {token},
		"remoteip": {wwhttp.IpForContext(ctx).String()},
	}

	// POST.
	req, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", postData)
	if err != nil {
		return false, errors.Wrapf(err, "error verifying turnstile token")
	}
	defer func() { _ = req.Body.Close() }()

	// Read response.
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return false, errors.Wrapf(err, "error reading turnstile response")
	}

	// Check status code.
	if req.StatusCode != http.StatusOK {
		return false, errors.Errorf("%d error verifying turnstile token: %s", req.StatusCode, body)
	}

	// Parse response.
	var resp TurnstileVerifyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, errors.Wrapf(err, "error parsing turnstile response: %s", body)
	}

	// Check success.
	if !resp.Success {
		t.log.Warn().Strs("error_codes", resp.ErrorCodes).Msg("turnstile token verification failed")
		return false, nil
	}

	// Check action.
	if resp.Action == nil || *resp.Action != expectedAction {
		t.log.Warn().Msgf("turnstile token is valid but has wrong action (expected %s, got %s)", expectedAction, *resp.Action)
		return false, nil
	}

	// Check time.
	if resp.ChallengeTs == nil || time.Since(*resp.ChallengeTs) > 1*time.Minute {
		t.log.Warn().Msg("turnstile token is valid but is too old")
		return false, nil
	}

	// Check hostname.
	if resp.Hostname == nil || !wwgo.SliceIncludes(t.trustedDomains, *resp.Hostname) {
		t.log.Warn().Msgf("turnstile token is valid but has wrong hostname (expected %s, got %s)", t.trustedDomains, *resp.Hostname)
		return false, nil
	}

	// Success.
	return true, nil
}
