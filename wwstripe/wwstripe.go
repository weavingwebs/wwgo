package wwstripe

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

type StripeEventHandler[API any] func(ctx context.Context, stripeApi *Stripe[API], event Event) ([]byte, error)

type Stripe[API any] struct {
	sc              StripeClient[API]
	log             zerolog.Logger
	webhookWg       *sync.WaitGroup
	stripePublicKey string
	stripeSecretKey string
	webhookUrl      string
	webhookSecret   string
}

type StripePublicSettings struct {
	StripePublicKey string `json:"stripePublicKey"`
}

// StripeClient is a generic interface to decouple from the stripe api client
// version. This is important because once set up, stripe webhooks will stick
// with the version we set until we migrate them.
type StripeClient[API any] interface {
	APIVersion() string
	Client() API
	Init(secretKey string)
	ListWebhookEndpoints() []*WebhookEndpoint
	UpdateWebhookEndpoint(id string, params *WebhookEndpointParams) (*WebhookEndpoint, error)
	NewWebhookEndpoint(params *WebhookEndpointParams) (*WebhookEndpoint, error)
	ConstructEvent(payload []byte, header string, secret string) (Event, error)
}

type WebhookEndpoint struct {
	APIVersion string `json:"api_version"`
	// The ID of the associated Connect application.
	Application string `json:"application"`
	// Time at which the object was created. Measured in seconds since the Unix epoch.
	Created int64 `json:"created"`
	Deleted bool  `json:"deleted"`
	// An optional description of what the webhook is used for.
	Description string `json:"description"`
	// The list of events to enable for this endpoint. `['*']` indicates that all events are enabled, except those that require explicit selection.
	EnabledEvents []string `json:"enabled_events"`
	// Unique identifier for the object.
	ID string `json:"id"`
	// Has the value `true` if the object exists in live mode or the value `false` if the object exists in test mode.
	Livemode bool `json:"livemode"`
	// Set of [key-value pairs](https://stripe.com/docs/api/metadata) that you can attach to an object. This can be useful for storing additional information about the object in a structured format.
	Metadata map[string]string `json:"metadata"`
	// String representing the object's type. Objects of the same type share the same value.
	Object string `json:"object"`
	// The endpoint's secret, used to generate [webhook signatures](https://stripe.com/docs/webhooks/signatures). Only returned at creation.
	Secret string `json:"secret"`
	// The status of the webhook. It can be `enabled` or `disabled`.
	Status string `json:"status"`
	// The URL of the webhook endpoint.
	URL string `json:"url"`
}

type WebhookEndpointParams struct {
	// Whether this endpoint should receive events from connected accounts (`true`), or from your account (`false`). Defaults to `false`.
	Connect *bool `form:"connect"`
	// An optional description of what the webhook is used for.
	Description *string `form:"description"`
	// Disable the webhook endpoint if set to true.
	Disabled *bool `form:"disabled"`
	// The list of events to enable for this endpoint. You may specify `['*']` to enable all events, except those that require explicit selection.
	EnabledEvents []*string `form:"enabled_events"`
	// Specifies which fields in the response should be expanded.
	Expand []*string `form:"expand"`
	// Set of [key-value pairs](https://stripe.com/docs/api/metadata) that you can attach to an object. This can be useful for storing additional information about the object in a structured format. Individual keys can be unset by posting an empty value to them. All keys can be unset by posting an empty value to `metadata`.
	Metadata map[string]string `form:"metadata"`
	// The URL of the webhook endpoint.
	URL *string `form:"url"`
	// This parameter is only available on creation.
	// We recommend setting the API version that the library is pinned to. See apiversion in stripe.go
	// Events sent to this endpoint will be generated with this Stripe Version instead of your account's default Stripe Version.
	APIVersion *string `form:"api_version"`
}

type Event struct {
	// The connected account that originates the event.
	Account string `json:"account"`
	// The Stripe API version used to render `data`. This property is populated only for events on or after October 31, 2014.
	APIVersion string `json:"api_version"`
	// Time at which the object was created. Measured in seconds since the Unix epoch.
	Created int64      `json:"created"`
	Data    *EventData `json:"data"`
	// Unique identifier for the object.
	ID string `json:"id"`
	// Has the value `true` if the object exists in live mode or the value `false` if the object exists in test mode.
	Livemode bool `json:"livemode"`
	// String representing the object's type. Objects of the same type share the same value.
	Object string `json:"object"`
	// Number of webhooks that haven't been successfully delivered (for example, to return a 20x response) to the URLs you specify.
	PendingWebhooks int64 `json:"pending_webhooks"`
	// Information on the API request that triggers the event.
	Request *EventRequest `json:"request"`
	// Description of the event (for example, `invoice.created` or `charge.refunded`).
	Type string `json:"type"`
}

type EventRequest struct {
	// ID is the request ID of the request that created an event, if the event
	// was created by a request.
	// ID of the API request that caused the event. If null, the event was automatic (e.g., Stripe's automatic subscription handling). Request logs are available in the [dashboard](https://dashboard.stripe.com/logs), but currently not in the API.
	ID string `json:"id"`

	// IdempotencyKey is the idempotency key of the request that created an
	// event, if the event was created by a request and if an idempotency key
	// was specified for that request.
	// The idempotency key transmitted during the request, if any. *Note: This property is populated only for events on or after May 23, 2017*.
	IdempotencyKey string `json:"idempotency_key"`
}

type EventData struct {
	// Object is a raw mapping of the API resource contained in the event.
	// Although marked with json:"-", it's still populated independently by
	// a custom UnmarshalJSON implementation.
	// Object containing the API resource relevant to the event. For example, an `invoice.created` event will have a full [invoice object](https://stripe.com/docs/api#invoice_object) as the value of the object key.
	Object map[string]interface{} `json:"-"`
	// Object containing the names of the updated attributes and their values prior to the event (only included in events of type `*.updated`). If an array attribute has any updated elements, this object contains the entire array. In Stripe API versions 2017-04-06 or earlier, an updated array attribute in this object includes only the updated array elements.
	PreviousAttributes map[string]interface{} `json:"previous_attributes"`
	Raw                json.RawMessage        `json:"object"`
}

func NewStripeFromEnv[API any](log zerolog.Logger, stripeClient StripeClient[API]) (*Stripe[API], error) {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		return nil, errors.New("missing STRIPE_SECRET_KEY")
	}
	sApi := &Stripe[API]{
		sc:              stripeClient,
		log:             log,
		webhookWg:       &sync.WaitGroup{},
		stripePublicKey: os.Getenv("STRIPE_PUBLIC_KEY"),
		stripeSecretKey: stripeKey,
		webhookUrl:      "",
		webhookSecret:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	sApi.sc.Init(stripeKey)

	return sApi, nil
}

func (sApi *Stripe[API]) Client() API {
	// Wait if webhook is still setting up.
	sApi.webhookWg.Wait()
	return sApi.sc.Client()
}

func (sApi *Stripe[API]) isTestMode() bool {
	return strings.HasPrefix(sApi.stripeSecretKey, "sk_test_")
}

func (sApi *Stripe[API]) WebUrl(stripeId string) string {
	url := "https://dashboard.stripe.com/"
	if sApi.isTestMode() {
		url += "test/"
	}
	url += "payments/" + stripeId
	return url
}

func (sApi *Stripe[API]) PublicSettings() StripePublicSettings {
	return StripePublicSettings{StripePublicKey: sApi.stripePublicKey}
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
		events[i] = wwgo.ToPtr(e)
	}
	return events
}

var ErrNoWebhook = errors.Errorf("Stripe webhook does not exist")
var ErrNoWebhookSecret = errors.Errorf("FATAL: Webhook exists but webhook secret is not configured, find it at https://dashboard.stripe.com/")

type MigrateWebhookFailHandler func(err error)

// MigrateWebhook asynchronously checks stripe API for an existing webhook and
// updates the subscribed events if needed. onFail is called if a webhook for
// the given url does not exist or the webhook secret is not set.
func (sApi *Stripe[API]) MigrateWebhook(input WebhookInput, onFail MigrateWebhookFailHandler) {
	// IMPORTANT: Do not to call Client() from here, it will deadlock.
	sApi.webhookWg.Add(1)
	go func() {
		defer sApi.webhookWg.Done()

		// Check if the webhook is already setup.
		enabledEvents := input.stripeEvents()
		for _, we := range sApi.sc.ListWebhookEndpoints() {
			if we.URL == input.Url {
				if we.Status == "enabled" && eventsMatch(we.EnabledEvents, enabledEvents) && we.APIVersion == sApi.sc.APIVersion() {
					sApi.log.Info().Msgf("Stripe Webhook already setup: " + we.ID)
					sApi.log.Debug().Interface("webhook", we).Msgf("Stripe Webhook")

					if sApi.webhookSecret == "" {
						onFail(ErrNoWebhookSecret)
					}

					return
				}

				// If the API version is not the same, we need to recreate the webhook.
				if we.APIVersion != sApi.sc.APIVersion() {
					err := errors.Errorf("Stripe Webhook API version mismatch (webhook version: %s, SDK: %s). Ensure all pending webhooks are complete (using the previous version) then disable the existing webhook (deleting it will lose history) and create a new one.", we.APIVersion, sApi.sc.APIVersion())
					sApi.log.Err(err).Send()
					onFail(err)
					return
				}

				// Update the webhook.
				params := &WebhookEndpointParams{
					Disabled:      wwgo.ToPtr(false),
					EnabledEvents: enabledEvents,
				}
				we, err := sApi.sc.UpdateWebhookEndpoint(
					we.ID,
					params,
				)
				if err != nil {
					err = errors.Wrap(err, "error updating webhook")
					sApi.log.Err(err).Send()
					onFail(err)
					return
				}
				sApi.log.Info().Msgf("Stripe Webhook updated: " + we.ID)
				sApi.log.Debug().Interface("webhook", we).Msgf("Updated Stripe Webhook")
				if sApi.webhookSecret == "" {
					onFail(ErrNoWebhookSecret)
				}
				return
			}
		}

		// Webhook does not exist.
		onFail(ErrNoWebhook)
	}()
}

func (sApi *Stripe[API]) CreateWebhook(input WebhookInput) (*WebhookEndpoint, error) {
	// Check if the webhook is already setup.
	for _, we := range sApi.sc.ListWebhookEndpoints() {
		if we.URL == input.Url && we.Status == "enabled" {
			return nil, errors.Errorf("Webhook already exists for %s: %s", input.Url, we.ID)
		}
	}

	// Create webhook.
	params := &WebhookEndpointParams{
		URL:           wwgo.ToPtr(input.Url),
		EnabledEvents: input.stripeEvents(),
		APIVersion:    wwgo.ToPtr(sApi.sc.APIVersion()),
	}
	we, err := sApi.sc.NewWebhookEndpoint(
		params,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating webhook")
	}
	sApi.log.Info().Msgf("Stripe Webhook created: " + we.ID)
	sApi.log.Debug().Interface("webhook", we).Msgf("Created Stripe Webhook")
	return we, nil
}

func (sApi *Stripe[API]) WebhookHandlerFunc(onWebhook StripeEventHandler[API], onError func(err error)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Read request body.
		const MaxBodyBytes = int64(65536)
		req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			sApi.log.Err(errors.Wrap(err, "error reading stripe webhook payload")).Send()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sApi.log.Trace().Interface("webhookPayload", payload).Msgf("Stripe Webhook Event")

		// Parse the event.
		event, err := sApi.sc.ConstructEvent(
			payload,
			req.Header.Get("Stripe-Signature"),
			sApi.webhookSecret,
		)
		if err != nil {
			err = errors.Wrap(err, "error parsing stripe webhook event")
			sApi.log.Err(err).Interface("payload", payload).Send()
			onError(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sApi.log.Trace().RawJSON("webhookEvent", event.Data.Raw).Msgf("Stripe Webhook Data")
		sApi.log.Info().Msgf("Stripe webhook event %s received", event.ID)

		// NOTE: We want to fully process the webhook event before returning a
		// response so that if we fail, stripe will know.
		resp, err := onWebhook(req.Context(), sApi, event)
		if err != nil {
			err = errors.Wrapf(err, "onWebhook failed to process event %s", event.ID)
			sApi.log.Err(err).RawJSON("webhookEvent", event.Data.Raw).Send()
			onError(err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
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
