package wwalert

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

type MsTeamsMessagePayload struct {
	Text string `json:"text"`
}

func SendMsTeams(ctx context.Context, webhookUrl string, message MsTeamsMessagePayload) error {
	firstAttemptAt := time.Now()
	maxAttemptDuration := 2 * time.Minute
	for {
		resp, err := doSendMsTeams(ctx, webhookUrl, message)
		if err != nil {
			return err
		}

		// Handle error response.
		if resp.StatusCode == 429 {
			retryAfterStr := resp.Header.Get("Retry-After")
			retryAfter, _ := strconv.Atoi(retryAfterStr)
			if retryAfter == 0 {
				retryAfter = 30
			}

			// If the next retry time will not exceed the max try time, try again.
			retryDelay := time.Duration(retryAfter) * time.Second
			if time.Since(firstAttemptAt)+retryDelay <= maxAttemptDuration {
				time.Sleep(retryDelay)
				continue
			}
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return errors.Errorf("MS Teams returned %d: %s", resp.StatusCode, body)
		}

		return nil
	}
}

func doSendMsTeams(ctx context.Context, webhookUrl string, message MsTeamsMessagePayload) (*http.Response, error) {
	encoded, err := json.Marshal(&message)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode MS Teams message")
	}
	body := bytes.NewBuffer(encoded)

	client := http.Client{Timeout: time.Second * 30}
	req, err := http.NewRequestWithContext(ctx, "POST", webhookUrl, body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MS Teams request")
	}
	req.Header.Set("Content-Type", "application/json; charset: utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send to MS Teams")
	}
	defer func() { _ = resp.Body.Close() }()
	return resp, err
}
