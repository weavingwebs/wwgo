package wwaws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/smithy-go/logging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"os"
	"runtime"
)

type awsLogger struct {
	log zerolog.Logger
}

func (l awsLogger) Logf(classification logging.Classification, format string, v ...interface{}) {
	var logEvent *zerolog.Event
	if classification == logging.Warn {
		logEvent = l.log.Warn()
	} else {
		logEvent = l.log.Debug()
	}
	logEvent.Msgf(format, v...)
}

func NewAwsConfig(ctx context.Context, log zerolog.Logger) aws.Config {
	ensureHomeEnv(log)

	configs := []func(options *config.LoadOptions) error{
		config.WithLogger(awsLogger{log: log.With().Str("lib", "aws").Logger()}),
	}
	role := os.Getenv("AWS_ROLE_ARN")
	if role != "" {
		configs = append(configs, config.WithAssumeRoleCredentialOptions(func(assumeRoleOptions *stscreds.AssumeRoleOptions) {
			assumeRoleOptions.RoleARN = role
		}))
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, configs...)
	if err != nil {
		panic(errors.Wrapf(err, "failed to load aws config"))
	}

	return awsConfig
}

// ensureHomeEnv sets $HOME if missing (e.g. systemd services), otherwise the AWS SDK fails to load credentials.
func ensureHomeEnv(log zerolog.Logger) {
	if runtime.GOOS == "windows" || os.Getenv("HOME") != "" {
		return
	}
	home := ""
	if os.Getuid() == 0 {
		home = "/root"
	} else if userName := os.Getenv("USER"); userName != "" {
		home = "/home/" + userName
	}
	if home == "" {
		return
	}
	if err := os.Setenv("HOME", home); err != nil {
		log.Warn().Err(err).Msg("Failed to set HOME")
	}
}
