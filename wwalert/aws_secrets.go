package wwalert

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/pkg/errors"
	"os"
	"runtime"
	"sync"
)

var awsSession *session.Session
var awsSecretsManager *secretsmanager.SecretsManager
var cacheMutex sync.Mutex
var cachedSecrets = map[string]map[string]string{}

type AwsSecret struct {
	Id   string `json:"id"`
	Prop string `json:"prop"`
}

func (s AwsSecret) Resolve(ctx context.Context) (string, error) {
	return getAwsSecret(ctx, s.Id, s.Prop)
}

func getAwsSecret(ctx context.Context, id string, prop string) (string, error) {
	// Check cache.
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	secret, ok := cachedSecrets[id]
	if !ok {
		if awsSession == nil {
			// If $HOME is not set, the AWS SDK will fail to load credentials.
			// This can happen with systemd services.
			if runtime.GOOS != "windows" && os.Getenv("HOME") == "" {
				if os.Getuid() == 0 {
					if err := os.Setenv("HOME", "/root"); err != nil {
						return "", errors.Wrapf(err, "failed to set HOME")
					}
				} else if userName := os.Getenv("USER"); userName != "" {
					if err := os.Setenv("HOME", "/home/"+userName); err != nil {
						return "", errors.Wrapf(err, "failed to set HOME")
					}
				}
			}

			// NOTE: AWS region needs to be set via one of:
			// - AWS_REGION
			// - ~/.aws/config
			var err error
			awsSession, err = session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
			})
			if err != nil {
				return "", errors.Wrapf(err, "failed to init AWS session")
			}
		}
		if awsSecretsManager == nil {
			awsSecretsManager = secretsmanager.New(awsSession)
		}

		// Get secret.
		resp, err := awsSecretsManager.GetSecretValueWithContext(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(id),
		})
		if err != nil {
			return "", errors.Wrapf(err, "failed to get secret '%s'", id)
		}

		// Decode secret.
		if resp.SecretString == nil || *resp.SecretString == "" {
			return "", errors.Errorf("secret string is not set on '%s'", id)
		}
		if err := json.Unmarshal([]byte(*resp.SecretString), &secret); err != nil {
			return "", errors.Wrapf(err, "failed to decode secret '%s'", id)
		}

		// Update cache.
		cachedSecrets[id] = secret
	}

	// Get value.
	value, ok := secret[prop]
	if !ok {
		return "", errors.Errorf("secret '%s' has no prop '%s'", id, prop)
	}
	return value, nil
}
