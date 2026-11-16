package stripe

import (
	"context"
	"testing"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_CreatePortalSession(t *testing.T) {
	t.Run("Happy path: opens a portal session for the customer and maps the URL", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 200,
			`{"id":"bps_1","object":"billing_portal.session","url":"https://billing.stripe.test/p/bps_1"}`))

		got, err := p.CreatePortalSession(context.Background(), billing.PortalRequest{CustomerID: "cus_1"})
		require.NoError(t, err)
		assert.Equal(t, "https://billing.stripe.test/p/bps_1", got.URL)
		assert.Equal(t, "/v1/billing_portal/sessions", rec.path)
		assert.Equal(t, "POST", rec.method)
		assert.Equal(t, "cus_1", rec.form["customer"])
	})

	t.Run("Error: a 404 from Stripe for an unknown customer is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 404,
			`{"error":{"type":"invalid_request_error","message":"no such customer"}}`))
		_, err := p.CreatePortalSession(context.Background(), billing.PortalRequest{CustomerID: "cus_missing"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a 5xx from Stripe is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 500,
			`{"error":{"type":"api_error","message":"boom"}}`))
		_, err := p.CreatePortalSession(context.Background(), billing.PortalRequest{CustomerID: "cus_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	t.Run("Error: a closed connection is unavailable", func(t *testing.T) {
		p, srv := newTestProvider(t, respondWith(&captured{}, 200, `{}`))
		srv.Close()
		_, err := p.CreatePortalSession(context.Background(), billing.PortalRequest{CustomerID: "cus_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})
}

func TestProvider_CancelAtPeriodEnd(t *testing.T) {
	t.Run("Happy path: sets cancel_at_period_end on the provider subscription", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 200,
			`{"id":"sub_1","object":"subscription","cancel_at_period_end":true}`))

		err := p.CancelAtPeriodEnd(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_1"})
		require.NoError(t, err)
		assert.Equal(t, "POST", rec.method)
		assert.Equal(t, "/v1/subscriptions/sub_1", rec.path)
		assert.Equal(t, "true", rec.form["cancel_at_period_end"])
	})

	t.Run("Error: a 404 from Stripe for an unknown subscription is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 404,
			`{"error":{"type":"invalid_request_error","message":"no such subscription"}}`))
		err := p.CancelAtPeriodEnd(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_gone"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a 503 from Stripe is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 503,
			`{"error":{"type":"api_error","message":"busy"}}`))
		err := p.CancelAtPeriodEnd(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	t.Run("Error: a closed connection is unavailable", func(t *testing.T) {
		p, srv := newTestProvider(t, respondWith(&captured{}, 200, `{}`))
		srv.Close()
		err := p.CancelAtPeriodEnd(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})
}
