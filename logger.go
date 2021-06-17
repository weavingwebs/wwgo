package wwgo

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"os"
	"strings"
)

func NewDefaultLogger() zerolog.Logger {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger := zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()

	// Set log level from env.
	logLevel := zerolog.InfoLevel
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr != "" {
		switch strings.ToUpper(logLevelStr) {
		case "TRACE":
			logLevel = zerolog.TraceLevel
		case "DEBUG":
			logLevel = zerolog.DebugLevel
		case "INFO":
			logLevel = zerolog.InfoLevel
		default:
			logger.Error().Msgf("Unsupported LOG_LEVEL %s", logLevelStr)
		}
	}
	logger = logger.Level(logLevel)

	return logger
}
