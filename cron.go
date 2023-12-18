package wwgo

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"time"
)

type CronFn func(ctx context.Context, log zerolog.Logger) error

type CronTab struct {
	log      zerolog.Logger
	slack    *SlackWebhookClient
	siteName string
	crons    map[string]CronFn
	timeZone *time.Location
}

func NewCronTab(
	log zerolog.Logger,
	slack *SlackWebhookClient,
	siteName string,
	// i.e. "Europe/London", "UTC" or "Local" (not recommended).
	// See https://golang.org/pkg/time/#LoadLocation for more info.
	timeZoneName string,
	// i.e. "0 1 * * *" for 1am every day.
	crons map[string]CronFn,
) (*CronTab, error) {
	var timeZone *time.Location
	var err error
	if timeZone, err = time.LoadLocation(timeZoneName); err != nil {
		return nil, errors.Wrapf(err, "failed to load timezone '%s'", timeZoneName)
	}

	return &CronTab{
		log:      log,
		slack:    slack,
		siteName: siteName,
		crons:    crons,
		timeZone: timeZone,
	}, nil
}

// Start the crons in the background.
func (c *CronTab) Start(ctx context.Context) {
	log := c.log

	crons := cron.New(
		cron.WithLocation(c.timeZone),
		cron.WithLogger(zerologCronLogger{c.log}),
	)

	// Add crons.
	for spec, fn := range c.crons {
		if _, err := crons.AddFunc(spec, func() {
			if err := fn(ctx, log); err != nil {
				log.Err(err).Send()
				c.slack.Send(ctx, SlackMessagePayload{
					Channel:   nil,
					Username:  ToPtr(c.siteName),
					Text:      fmt.Sprintf("Cron job failed with error: %s", err),
					IconEmoji: ToPtr(":face_with_symbols_on_mouth:"),
				})
			}
		}); err != nil {
			log.Fatal().Err(err).Msgf("Failed to add cron %s", spec)
		}
	}

	// Run crons in background.
	crons.Start()

	// Stop crons on context cancel.
	go func() {
		<-ctx.Done()
		crons.Stop()
	}()
}

type zerologCronLogger struct {
	log zerolog.Logger
}

func (l zerologCronLogger) Info(msg string, keysAndValues ...interface{}) {
	zerologCronLog(l.log.Info(), msg, keysAndValues)
}

func (l zerologCronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	zerologCronLog(l.log.Err(err), msg, keysAndValues)
}

func zerologCronLog(logEntry *zerolog.Event, msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) != 0 {
		logEntry.Interface("cronContext", keysAndValues)
	}
	logEntry.Msgf(msg)
}
