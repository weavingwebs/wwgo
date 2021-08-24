package wwstripe

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/client"
	"github.com/stripe/stripe-go/v72/webhook"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

type StripeEventHandler func(stripeApi *Stripe, event stripe.Event) error

type Stripe struct {
	sc            *client.API
	log           zerolog.Logger
	webhookWg     *sync.WaitGroup
	stripeKey     string
	webhookUrl    string
	webhookSecret string
}

func NewStripeFromEnv(log zerolog.Logger) (*Stripe, error) {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		return nil, errors.New("missing STRIPE_SECRET_KEY")
	}
	sApi := &Stripe{
		log:           log,
		webhookWg:     &sync.WaitGroup{},
		stripeKey:     stripeKey,
		webhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	sApi.sc = &client.API{}
	sApi.sc.Init(stripeKey, nil)

	return sApi, nil
}

func (sApi *Stripe) Client() *client.API {
	// Wait if webhook is still setting up.
	sApi.webhookWg.Wait()
	return sApi.sc
}

func (sApi *Stripe) isTestMode() bool {
	return strings.HasPrefix(sApi.stripeKey, "sk_test_")
}

func (sApi *Stripe) WebUrl(stripeId string) string {
	url := "https://dashboard.stripe.com/"
	if sApi.isTestMode() {
		url += "test/"
	}
	url += "payments/" + stripeId
	return url
}

type WebhookInput struct {
	// Events to subscribe to i.e. checkout.session.completed,
	// payment_intent.succeeded, charge.refunded.
	// https://stripe.com/docs/api/events/types
	Events []string
	// The full url for the webhook i.e. https://example.com/webhook/stripe
	Url string
}

func (wi WebhookInput) stripeEvents() []*string {
	events := make([]*string, len(wi.Events))
	for i, e := range wi.Events {
		events[i] = stripe.String(e)
	}
	return events
}

// MigrateWebhook asynchronously checks stripe API for an existing webhook and
// updates the subscribed events if needed. Panics if a webhook for the given
// url does not exist or the webhook secret is not set.
func (sApi *Stripe) MigrateWebhook(input WebhookInput) {
	// IMPORTANT: Do not to call Client() from here, it will deadlock.
	sApi.webhookWg.Add(1)
	go func() {
		defer sApi.webhookWg.Done()

		// Check if the webhook is already setup.
		enabledEvents := input.stripeEvents()
		listParams := &stripe.WebhookEndpointListParams{}
		listParams.Filters.AddFilter("limit", "", "100")
		i := sApi.sc.WebhookEndpoints.List(listParams)
		for i.Next() {
			we := i.WebhookEndpoint()
			if we.URL == input.Url {
				if we.Status == "enabled" && eventsMatch(we.EnabledEvents, enabledEvents) {
					sApi.log.Info().Msgf("Stripe Webhook already setup: " + we.ID)
					sApi.log.Debug().Interface("webhook", we).Msgf("Stripe Webhook")

					if sApi.webhookSecret == "" {
						sApi.log.Panic().Msgf("FATAL: Webhook exists but webhook secret is not configured, find it at https://dashboard.stripe.com/")
					}

					return
				}

				// Update the webhook.
				params := &stripe.WebhookEndpointParams{
					Disabled:      stripe.Bool(false),
					EnabledEvents: enabledEvents,
				}
				we, err := sApi.sc.WebhookEndpoints.Update(
					we.ID,
					params,
				)
				if err != nil {
					sApi.log.Err(errors.Wrap(err, "error updating webhook")).Send()
					return
				}
				sApi.log.Info().Msgf("Stripe Webhook updated: " + we.ID)
				sApi.log.Debug().Interface("webhook", we).Msgf("Updated Stripe Webhook")
				if sApi.webhookSecret == "" {
					sApi.log.Panic().Msgf("FATAL: Webhook exists but webhook secret is not configured, find it at https://dashboard.stripe.com/")
				}
				return
			}
		}

		// Webhook does not exist.
		sApi.log.Panic().Err(errors.Errorf("Stripe webhook does not exist")).Send()
	}()
}

func (sApi *Stripe) CreateWebhook(input WebhookInput) (*stripe.WebhookEndpoint, error) {
	// Check if the webhook is already setup.
	listParams := &stripe.WebhookEndpointListParams{}
	listParams.Filters.AddFilter("limit", "", "100")
	i := sApi.sc.WebhookEndpoints.List(listParams)
	for i.Next() {
		we := i.WebhookEndpoint()
		if we.URL == input.Url {
			return nil, errors.Errorf("Webhook already exists for %s: %s", input.Url, we.ID)
		}
	}

	// Create webhook.
	params := &stripe.WebhookEndpointParams{
		URL:           stripe.String(input.Url),
		EnabledEvents: input.stripeEvents(),
	}
	we, err := sApi.sc.WebhookEndpoints.New(
		params,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating webhook")
	}
	sApi.log.Info().Msgf("Stripe Webhook created: " + we.ID)
	sApi.log.Debug().Interface("webhook", we).Msgf("Created Stripe Webhook")
	return we, nil
}

func (sApi *Stripe) WebhookHandlerFunc(onWebhook StripeEventHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Read request body.
		const MaxBodyBytes = int64(65536)
		req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
		payload, err := ioutil.ReadAll(req.Body)
		if err != nil {
			sApi.log.Err(errors.Wrap(err, "error reading stripe webhook payload")).Send()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sApi.log.Debug().Interface("webhookEvent", payload).Msgf("Stripe Webhook Event")

		// Parse the event.
		event, err := webhook.ConstructEvent(
			payload,
			req.Header.Get("Stripe-Signature"),
			sApi.webhookSecret,
		)
		if err != nil {
			sApi.log.Err(errors.Wrap(err, "error parsing stripe webhook event")).Interface("payload", payload).Send()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sApi.log.Info().Msgf("Stripe webhook event %s received", event.ID)

		// NOTE: We want to fully process the webhook event before returning a
		// response so that if we fail, stripe will know.
		if err := onWebhook(sApi, event); err != nil {
			sApi.log.Err(errors.Wrap(err, "onWebhook failed")).Interface("payload", payload).Send()
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func eventsMatch(a []string, b []*string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, aV := range a {
		found := false
		for _, bV := range b {
			if aV == *bV {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
