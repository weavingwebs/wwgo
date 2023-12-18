package wwgo

import (
	"context"
)

type Alerter interface {
	SendAlert(ctx context.Context, msg string) error
}

type CategoryAlerter interface {
	SendAlert(ctx context.Context, category string, msg string) error
}
