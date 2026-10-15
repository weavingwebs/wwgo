package stripev74

import (
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/client"
	"github.com/stripe/stripe-go/v74/webhook"
	"github.com/weavingwebs/wwgo/wwstripe"
)

var _ wwstripe.StripeClient[*client.API] = (*Client)(nil)

type Client struct {
	sc *client.API
}

func NewClient() *Client {
	return &Client{sc: &client.API{}}
}

func (sApi *Client) APIVersion() string {
	return stripe.APIVersion
}

func (sApi *Client) Client() *client.API {
	return sApi.sc
}

func (sApi *Client) Init(secretKey string) {
	sApi.sc.Init(secretKey, nil)
}

func (sApi *Client) ListWebhookEndpoints() []*wwstripe.WebhookEndpoint {
	listParams := &stripe.WebhookEndpointListParams{}
	listParams.Filters.AddFilter("limit", "", "100")
	i := sApi.sc.WebhookEndpoints.List(listParams)

	var res []*wwstripe.WebhookEndpoint
	for i.Next() {
		res = append(res, webhookEndpointFromStripe(i.WebhookEndpoint()))
	}
	return res
}

func (sApi *Client) UpdateWebhookEndpoint(id string, params *wwstripe.WebhookEndpointParams) (*wwstripe.WebhookEndpoint, error) {
	res, err := sApi.sc.WebhookEndpoints.Update(
		id,
		webhookEndpointParamsToStripe(params),
	)
	return webhookEndpointFromStripe(res), err
}

func (sApi *Client) NewWebhookEndpoint(params *wwstripe.WebhookEndpointParams) (*wwstripe.WebhookEndpoint, error) {
	res, err := sApi.sc.WebhookEndpoints.New(
		webhookEndpointParamsToStripe(params),
	)
	return webhookEndpointFromStripe(res), err
}

func (sApi *Client) ConstructEvent(payload []byte, header string, secret string) (wwstripe.Event, error) {
	res, err := webhook.ConstructEvent(
		payload,
		header,
		secret,
	)
	return eventFromStripe(res), err
}

func webhookEndpointFromStripe(endpoint *stripe.WebhookEndpoint) *wwstripe.WebhookEndpoint {
	if endpoint == nil {
		return nil
	}
	return &wwstripe.WebhookEndpoint{
		APIVersion:    endpoint.APIVersion,
		Application:   endpoint.Application,
		Created:       endpoint.Created,
		Deleted:       endpoint.Deleted,
		Description:   endpoint.Description,
		EnabledEvents: endpoint.EnabledEvents,
		ID:            endpoint.ID,
		Livemode:      endpoint.Livemode,
		Metadata:      endpoint.Metadata,
		Object:        endpoint.Object,
		Secret:        endpoint.Secret,
		Status:        endpoint.Status,
		URL:           endpoint.URL,
	}
}

func webhookEndpointParamsToStripe(params *wwstripe.WebhookEndpointParams) *stripe.WebhookEndpointParams {
	if params == nil {
		return nil
	}
	return &stripe.WebhookEndpointParams{
		Params: stripe.Params{
			Expand:   params.Expand,
			Metadata: params.Metadata,
		},
		Connect:       params.Connect,
		Description:   params.Description,
		Disabled:      params.Disabled,
		EnabledEvents: params.EnabledEvents,
		URL:           params.URL,
		APIVersion:    params.APIVersion,
	}
}

func eventFromStripe(event stripe.Event) wwstripe.Event {
	return wwstripe.Event{
		ID:         event.ID,
		Object:     event.Object,
		Account:    event.Account,
		APIVersion: event.APIVersion,
		Data:       eventDataFromStripe(event.Data),
		Livemode:   event.Livemode,
		Type:       event.Type,
	}
}

func eventDataFromStripe(data *stripe.EventData) *wwstripe.EventData {
	if data == nil {
		return nil
	}
	return &wwstripe.EventData{
		Object:             data.Object,
		PreviousAttributes: data.PreviousAttributes,
		Raw:                data.Raw,
	}
}
