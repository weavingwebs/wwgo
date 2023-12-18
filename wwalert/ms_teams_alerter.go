package wwalert

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"strings"
)

type MsTeamsAlertConfig struct {
	// Required if WebhookAwsSecret is not set.
	Webhook string `yaml:"webhook"`
	// Required if Webhook is not set, otherwise ignored.
	WebhookAwsSecret *AwsSecret `yaml:"webhook_aws_secret"`
	// Optional
	Prefix string
}

type MsTeamsAlerter struct {
	Log           zerolog.Logger
	Configs       []MsTeamsAlertConfig
	DefaultPrefix string
}

func (sa MsTeamsAlerter) SendAlert(ctx context.Context, msg string) error {
	// Send alerts.
	sent := 0
	for i, c := range sa.Configs {
		// Default prefix.
		prefix := strings.TrimSpace(c.Prefix)
		if prefix == "" {
			prefix = fmt.Sprintf("**[%s]**", sa.DefaultPrefix)
		}

		// Ensure the prefix has a space.
		prefix = strings.TrimSuffix(prefix, " ") + " "

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
		err := SendMsTeams(ctx, webhook, MsTeamsMessagePayload{
			Text: prefix + msg,
		})
		if err != nil {
			sa.Log.Err(err).Msgf("Failed to send MS Teams alert")
			continue
		}
		sent++
	}

	// Check if all succeeded.
	total := len(sa.Configs)
	if sent < total {
		return errors.Errorf("%d/%d MS Teams alerts succeeded", sent, total)
	}
	return nil
}
