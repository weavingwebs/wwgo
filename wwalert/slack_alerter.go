package wwalert

import (
	"context"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"strings"
)

type SlackAlertConfig struct {
	// Required if WebhookAwsSecret is not set.
	Webhook string `yaml:"webhook"`
	// Required if Webhook is not set, otherwise ignored.
	WebhookAwsSecret *AwsSecret `yaml:"webhook_aws_secret"`
	// Optional.
	Channel string `yaml:"channel"`
	// Optional.
	Username string `yaml:"username"`
	// Optional.
	IconEmoji string `yaml:"icon_emoji"`
}

type SlackAlerter struct {
	Log             zerolog.Logger
	Configs         []SlackAlertConfig
	DefaultUsername string
}

func (sa SlackAlerter) SendAlert(ctx context.Context, msg string) error {
	// Send alerts.
	sent := 0
	for i, c := range sa.Configs {
		// Default username.
		username := strings.TrimSpace(c.Username)
		if username == "" {
			username = sa.DefaultUsername
		}

		// Resolve webhook url.
		webhook := c.Webhook
		if webhook == "" {
			if c.WebhookAwsSecret == nil {
				return errors.Errorf("[%d].webhook is not defined", i)
			}
			var err error
			webhook, err = c.WebhookAwsSecret.Resolve(ctx)
			if err != nil {
				return err
			}
		}

		// Send.
		slackClient := wwgo.NewSlackClient(sa.Log, webhook, c.Channel)
		err := slackClient.TrySend(ctx, wwgo.SlackMessagePayload{
			Channel:   wwgo.StrNilIfEmpty(strings.TrimSpace(c.Channel)),
			Username:  wwgo.StrNilIfEmpty(username),
			Text:      msg,
			IconEmoji: wwgo.StrNilIfEmpty(strings.TrimSpace(c.IconEmoji)),
		})
		if err != nil {
			sa.Log.Err(err).Msgf("Failed to send slack alert")
			continue
		}
		sent++
	}

	// Check if all succeeded.
	total := len(sa.Configs)
	if sent < total {
		return errors.Errorf("%d/%d slack alerts succeeded", sent, total)
	}
	return nil
}
