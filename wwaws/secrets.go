package wwaws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/pkg/errors"
)

// SecretsFetcher implements wwalert.SecretFetcher.
type SecretsFetcher struct {
	client *secretsmanager.Client
}

func NewSecretsFetcher(cfg aws.Config) *SecretsFetcher {
	return &SecretsFetcher{client: secretsmanager.NewFromConfig(cfg)}
}

func (f *SecretsFetcher) GetSecretString(ctx context.Context, id string) (string, error) {
	resp, err := f.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(id),
	})
	if err != nil {
		return "", errors.WithStack(err)
	}
	return aws.ToString(resp.SecretString), nil
}
