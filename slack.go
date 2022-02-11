package wwgo

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

type SlackWebhookClient struct {
	webhookUrl string
	channel    string
	log        zerolog.Logger
}

func NewSlackWebhookClientFromEnv(log zerolog.Logger) *SlackWebhookClient {
	webhookUrl := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookUrl == "" {
		panic("SLACK_WEBHOOK_URL is not set")
	}
	return &SlackWebhookClient{webhookUrl: webhookUrl, channel: os.Getenv("SLACK_WEBHOOK_CHANNEL"), log: log}
}

func (s *SlackWebhookClient) Send(ctx context.Context, message SlackMessagePayload) {
	if message.Channel == nil && s.channel != "" {
		message.Channel = StrRef(s.channel)
	}
	if err := SendSlackWebhook(ctx, s.webhookUrl, message); err != nil {
		s.log.Err(errors.Wrapf(err, "Failed to send slack")).Send()
	}
}

type SlackMessagePayload struct {
	Channel   *string `json:"channel"`
	Username  *string `json:"username"`
	Text      string  `json:"text"`
	IconEmoji *string `json:"icon_emoji"`
}

func SendSlackWebhook(ctx context.Context, webhookUrl string, message SlackMessagePayload) error {
	firstAttemptAt := time.Now()
	maxAttemptDuration := 2 * time.Minute
	for {
		resp, err := doSendSlack(ctx, webhookUrl, message)
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
			body, _ := ioutil.ReadAll(resp.Body)
			return errors.Errorf("slack returned %d: %s", resp.StatusCode, body)
		}

		return nil
	}
}

func doSendSlack(ctx context.Context, webhookUrl string, message SlackMessagePayload) (*http.Response, error) {
	encoded, err := json.Marshal(&message)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode slack message")
	}
	body := bytes.NewBuffer(encoded)

	client := http.Client{Timeout: time.Second * 30}
	req, err := http.NewRequestWithContext(ctx, "POST", webhookUrl, body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create slack request")
	}
	req.Header.Set("Content-Type", "application/json; charset: utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to send slack")
	}
	defer func() { _ = resp.Body.Close() }()
	return resp, err
}
