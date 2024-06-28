package wwgo

import (
	"context"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"time"
)

type CronFn func(ctx context.Context, log zerolog.Logger) error

type CronTab struct {
	log      zerolog.Logger
	alerter  CategoryAlerter
	siteName string
	crons    map[string]CronFn
	timeZone *time.Location
}

func NewCronTab(
	log zerolog.Logger,
	alerter CategoryAlerter,
	siteName string,
	timeZone *time.Location,
	// i.e. "0 1 * * *" for 1am every day.
	// See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format for more info.
	crons map[string]CronFn,
) (*CronTab, error) {
	return &CronTab{
		log:      log,
		alerter:  alerter,
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
			// Catch panics.
			defer func() {
				if r := recover(); r != nil {
					log.Error().Str("panic", fmt.Sprintf("%+v", r)).Str("cron", spec).Msg("Panic in cron")
					if innerErr := c.alerter.SendAlert(ctx, "panic", fmt.Sprintf("Panic in cron '%s': %+v", spec, r)); innerErr != nil {
						log.Err(innerErr).Msg("Failed to send alert")
					}
				}
			}()
			if err := fn(ctx, log); err != nil {
				log.Err(err).Send()
				if innerErr := c.alerter.SendAlert(ctx, "api_error", fmt.Sprintf("Cron job failed with error: %s", err)); innerErr != nil {
					log.Err(innerErr).Msg("Failed to send alert")
				}
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
