package wwalert

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"sync"
)

// SecretFetcher returns the raw secret string for an id, e.g. wwaws.SecretsFetcher.
type SecretFetcher interface {
	GetSecretString(ctx context.Context, id string) (string, error)
}

var cacheMutex sync.Mutex
var cachedSecrets = map[string]map[string]string{}

type AwsSecret struct {
	Id   string `json:"id"`
	Prop string `json:"prop"`
}

func (s AwsSecret) Resolve(ctx context.Context, fetcher SecretFetcher) (string, error) {
	return getSecret(ctx, fetcher, s.Id, s.Prop)
}

func getSecret(ctx context.Context, fetcher SecretFetcher, id string, prop string) (string, error) {
	// Check cache.
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	secret, ok := cachedSecrets[id]
	if !ok {
		if fetcher == nil {
			return "", errors.Errorf("no SecretFetcher configured to resolve secret '%s'", id)
		}

		// Get secret.
		secretString, err := fetcher.GetSecretString(ctx, id)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get secret '%s'", id)
		}

		// Decode secret.
		if secretString == "" {
			return "", errors.Errorf("secret string is not set on '%s'", id)
		}
		if err := json.Unmarshal([]byte(secretString), &secret); err != nil {
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
