package wwalert

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"os"
)

type CategoryConfig struct {
	Slack   []SlackAlertConfig   `yaml:"slack"`
	MsTeams []MsTeamsAlertConfig `yaml:"ms_teams"`
}

type AlertsConfig struct {
	Default    CategoryConfig            `yaml:"default"`
	Categories map[string]CategoryConfig `yaml:"categories"`
}

func ReadConfig(filePath string) (*AlertsConfig, error) {
	// Read file.
	configYaml, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read '%s'", filePath)
	}

	// Decode.
	config := &AlertsConfig{}
	if err := yaml.UnmarshalStrict(configYaml, config); err != nil {
		return nil, errors.Wrapf(err, "failed to decode '%s'", filePath)
	}
	return config, nil
}

type ConfigAlerter struct {
	log     zerolog.Logger
	config  *AlertsConfig
	appName string
}

func NewConfigAlerter(log zerolog.Logger, config *AlertsConfig, appName string) *ConfigAlerter {
	return &ConfigAlerter{
		log:     log,
		config:  config,
		appName: appName,
	}
}

func NewConfigAlerterFromEnv(log zerolog.Logger, appName string) (*ConfigAlerter, error) {
	filePath := os.Getenv("WWALERTS_CONFIG")
	if filePath == "" {
		return nil, fmt.Errorf("WWALERTS_CONFIG is not set")
	}
	config, err := ReadConfig(filePath)
	if err != nil {
		return nil, err
	}
	return NewConfigAlerter(log, config, appName), nil
}

func (a *ConfigAlerter) SendAlert(ctx context.Context, category string, msg string) error {
	// Figure out what config to use.
	alertConfig := a.config.Default
	if category != "" {
		tmpConfig, ok := a.config.Categories[category]
		if !ok {
			a.log.Warn().Msgf("[Warning] Unknown category '%s', using default", category)
		} else {
			alertConfig = tmpConfig
		}
	}

	// Send alerts.
	gos := errgroup.Group{}

	if alertConfig.Slack != nil && len(alertConfig.Slack) != 0 {
		gos.Go(func() error {
			alerter := SlackAlerter{
				Log:             a.log,
				Configs:         alertConfig.Slack,
				DefaultUsername: a.appName,
			}
			if err := alerter.SendAlert(ctx, msg); err != nil {
				return err
			}
			return nil
		})
	}

	if alertConfig.MsTeams != nil && len(alertConfig.MsTeams) != 0 {
		gos.Go(func() error {
			alerter := MsTeamsAlerter{
				Log:           a.log,
				Configs:       alertConfig.MsTeams,
				DefaultPrefix: a.appName,
			}
			if err := alerter.SendAlert(ctx, msg); err != nil {
				return err
			}
			return nil
		})
	}

	return gos.Wait()
}
