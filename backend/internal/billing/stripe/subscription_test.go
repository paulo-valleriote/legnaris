package stripe

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripesdk "github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

func TestMapStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   billing.Status
		err    bool
	}{
		{name: "Happy path: incomplete is pending", status: "incomplete", want: billing.StatusPending},
		{name: "Happy path: active is active", status: "active", want: billing.StatusActive},
		{name: "Happy path: past_due is past_due", status: "past_due", want: billing.StatusPastDue},
		{name: "Edge: paused is past_due", status: "paused", want: billing.StatusPastDue},
		{name: "Edge: unpaid is canceled", status: "unpaid", want: billing.StatusCanceled},
		{name: "Edge: incomplete_expired is canceled", status: "incomplete_expired", want: billing.StatusCanceled},
		{name: "Edge: canceled is canceled", status: "canceled", want: billing.StatusCanceled},
		{name: "Error: an unknown status is rejected", status: "trialing", err: true},
		{name: "Error: an empty status is rejected", status: "", err: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mapStatus(stripesdk.SubscriptionStatus(tc.status))
			if tc.err {
				require.Error(t, err)
				assert.ErrorIs(t, err, billing.ErrProviderRejected)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

const subscriptionBody = `{
  "id": "sub_read_1",
  "object": "subscription",
  "status": "past_due",
  "cancel_at_period_end": true,
  "customer": "cus_read_1",
  "items": {"object": "list", "data": [{"id": "si_1", "object": "subscription_item", "current_period_end": 1790000000}]}
}`

func TestProvider_GetSubscription(t *testing.T) {
	t.Run("Happy path: maps status, customer, period end and cancellation", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, http.StatusOK, subscriptionBody))

		got, err := p.GetSubscription(t.Context(), "sub_read_1")
		require.NoError(t, err)
		assert.Equal(t, "sub_read_1", got.ID)
		assert.Equal(t, "cus_read_1", got.CustomerID)
		assert.Equal(t, billing.StatusPastDue, got.Status)
		assert.True(t, got.CancelAtPeriodEnd)
		assert.True(t, got.CurrentPeriodEnd.Equal(time.Unix(1790000000, 0).UTC()))
		assert.Equal(t, http.MethodGet, rec.method)
		assert.Equal(t, "/v1/subscriptions/sub_read_1", rec.path)
	})

	t.Run("Edge: a subscription without items has a zero period end", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusOK,
			`{"id":"sub_bare","object":"subscription","status":"active","customer":"cus_x"}`))

		got, err := p.GetSubscription(t.Context(), "sub_bare")
		require.NoError(t, err)
		assert.True(t, got.CurrentPeriodEnd.IsZero())
		assert.Equal(t, billing.StatusActive, got.Status)
	})

	t.Run("Happy path: the subscription start date is the provider's first payment time", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusOK,
			`{"id":"sub_started","object":"subscription","status":"active","customer":"cus_x","start_date":1780000000}`))

		got, err := p.GetSubscription(t.Context(), "sub_started")
		require.NoError(t, err)
		assert.True(t, got.StartedAt.Equal(time.Unix(1780000000, 0).UTC()))
	})

	t.Run("Edge: a subscription without a start date has no provider start time", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusOK,
			`{"id":"sub_unstarted","object":"subscription","status":"active","customer":"cus_x"}`))

		got, err := p.GetSubscription(t.Context(), "sub_unstarted")
		require.NoError(t, err)
		assert.True(t, got.StartedAt.IsZero())
	})

	t.Run("Error: a 404 from the provider is rejected", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusNotFound,
			`{"error":{"type":"invalid_request_error","message":"No such subscription"}}`))

		_, err := p.GetSubscription(t.Context(), "sub_missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a 500 from the provider is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusInternalServerError,
			`{"error":{"type":"api_error","message":"boom"}}`))

		_, err := p.GetSubscription(t.Context(), "sub_read_1")
		require.Error(t, err)
		assert.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	t.Run("Error: an unknown status from the provider is rejected", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, http.StatusOK,
			`{"id":"sub_weird","object":"subscription","status":"trialing","customer":"cus_x"}`))

		_, err := p.GetSubscription(t.Context(), "sub_weird")
		require.Error(t, err)
		assert.ErrorIs(t, err, billing.ErrProviderRejected)
	})
}

const webhookSecret = "whsec_test_adapter"

func signedEvent(t *testing.T, body string) (payload []byte, header string) {
	t.Helper()
	payload = []byte(body)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: webhookSecret})
	return payload, signed.Header
}

func eventBody(t *testing.T, id, eventType string, object any) string {
	t.Helper()
	raw, err := json.Marshal(object)
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"id": id, "object": "event", "type": eventType,
		"api_version": stripesdk.APIVersion,
		"data":        map[string]any{"object": json.RawMessage(raw)},
	})
	require.NoError(t, err)
	return string(body)
}

func TestProvider_ParseWebhook(t *testing.T) {
	provider := &Provider{webhookSecret: webhookSecret}

	t.Run("Error: without a configured secret the webhook is unavailable", func(t *testing.T) {
		unconfigured := &Provider{}
		payload, header := signedEvent(t, eventBody(t, "evt_1", "customer.subscription.updated", map[string]any{"id": "sub_1"}))

		_, err := unconfigured.ParseWebhook(payload, header)
		assert.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	t.Run("Error: a bad signature is invalid", func(t *testing.T) {
		payload, _ := signedEvent(t, eventBody(t, "evt_1", "customer.subscription.updated", map[string]any{"id": "sub_1"}))

		_, err := provider.ParseWebhook(payload, "t=1,v1=forged")
		assert.ErrorIs(t, err, billing.ErrInvalidSignature)
	})

	t.Run("Error: an empty signature header is invalid", func(t *testing.T) {
		payload, _ := signedEvent(t, eventBody(t, "evt_1", "customer.subscription.updated", map[string]any{"id": "sub_1"}))

		_, err := provider.ParseWebhook(payload, "")
		assert.ErrorIs(t, err, billing.ErrInvalidSignature)
	})

	t.Run("Happy path: a subscription update carries the provider ids and the user from metadata", func(t *testing.T) {
		body := eventBody(t, "evt_sub", "customer.subscription.updated", map[string]any{
			"id": "sub_9", "object": "subscription", "status": "active",
			"customer": "cus_9", "metadata": map[string]string{"user_id": "user-9"},
		})
		payload, header := signedEvent(t, body)

		n, err := provider.ParseWebhook(payload, header)
		require.NoError(t, err)
		assert.Equal(t, "evt_sub", n.ID)
		assert.Equal(t, billing.KindSubscriptionChanged, n.Kind)
		assert.Equal(t, "sub_9", n.ProviderSubscriptionID)
		assert.Equal(t, "cus_9", n.ProviderCustomerID)
		assert.Equal(t, "user-9", n.UserID)
	})

	t.Run("Happy path: a completed subscription checkout carries the subscription and customer", func(t *testing.T) {
		body := eventBody(t, "evt_co", "checkout.session.completed", map[string]any{
			"id": "cs_1", "object": "checkout.session", "mode": "subscription",
			"subscription": "sub_new", "customer": "cus_new",
			"metadata": map[string]string{"user_id": "user-1"},
		})
		payload, header := signedEvent(t, body)

		n, err := provider.ParseWebhook(payload, header)
		require.NoError(t, err)
		assert.Equal(t, billing.KindSubscriptionChanged, n.Kind)
		assert.Equal(t, "sub_new", n.ProviderSubscriptionID)
		assert.Equal(t, "cus_new", n.ProviderCustomerID)
		assert.Equal(t, "user-1", n.UserID)
	})

	t.Run("Edge: a one-time payment checkout is ignored", func(t *testing.T) {
		body := eventBody(t, "evt_pay", "checkout.session.completed", map[string]any{
			"id": "cs_2", "object": "checkout.session", "mode": "payment", "customer": "cus_x",
		})
		payload, header := signedEvent(t, body)

		n, err := provider.ParseWebhook(payload, header)
		require.NoError(t, err)
		assert.Equal(t, billing.KindIgnored, n.Kind)
	})

	t.Run("Edge: an event type we do not use is ignored", func(t *testing.T) {
		payload, header := signedEvent(t, eventBody(t, "evt_inv", "invoice.paid", map[string]any{"id": "in_1"}))

		n, err := provider.ParseWebhook(payload, header)
		require.NoError(t, err)
		assert.Equal(t, billing.KindIgnored, n.Kind)
		assert.Equal(t, "evt_inv", n.ID)
	})

	t.Run("Edge: a subscription object without an id is ignored", func(t *testing.T) {
		payload, header := signedEvent(t, eventBody(t, "evt_empty", "customer.subscription.deleted", map[string]any{"object": "subscription"}))

		n, err := provider.ParseWebhook(payload, header)
		require.NoError(t, err)
		assert.Equal(t, billing.KindIgnored, n.Kind)
	})

	t.Run("Error: a body the SDK cannot parse is reported as an invalid webhook (400)", func(t *testing.T) {
		body := `{"id":"evt_bad","object":"event","type":"customer.subscription.updated","data":{"object":"not-an-object"}}`
		payload, header := signedEvent(t, body)

		_, err := provider.ParseWebhook(payload, header)
		assert.ErrorIs(t, err, billing.ErrInvalidSignature)
	})
}
