package stripe

import (
	"context"
	"net/http"
	"testing"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	invoicesBody           = `{"object":"list","data":[{"id":"in_1","object":"invoice","status":"paid"}],"has_more":false,"url":"/v1/invoices"}`
	emptyListBody          = `{"object":"list","data":[],"has_more":false,"url":"/v1/invoices"}`
	paymentsBody           = `{"object":"list","data":[{"id":"inpay_1","object":"invoice_payment","status":"paid","payment":{"type":"payment_intent","payment_intent":{"id":"pi_1","object":"payment_intent"}}}],"has_more":false,"url":"/v1/invoice_payments"}`
	noPaymentsBody         = `{"object":"list","data":[],"has_more":false,"url":"/v1/invoice_payments"}`
	refundCreatedBody      = `{"id":"re_1","object":"refund","status":"succeeded"}`
	cancelSubscriptionBody = `{"id":"sub_1","object":"subscription","status":"canceled"}`
)

func routedStripe(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		handler, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}
}

func jsonReply(rec *captured, status int, body string) func(w http.ResponseWriter, r *http.Request) {
	return respondWith(rec, status, body)
}

func TestProvider_RefundLatest(t *testing.T) {
	t.Run("Happy path: refunds the payment intent of the latest paid invoice with the idempotency key", func(t *testing.T) {
		refund := &captured{}
		p, _ := newTestProvider(t, routedStripe(t, map[string]func(http.ResponseWriter, *http.Request){
			"/v1/invoices":         jsonReply(&captured{}, 200, invoicesBody),
			"/v1/invoice_payments": jsonReply(&captured{}, 200, paymentsBody),
			"/v1/refunds":          jsonReply(refund, 200, refundCreatedBody),
		}))

		err := p.RefundLatest(context.Background(), billing.RefundRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "refund:loc_1"})
		require.NoError(t, err)
		assert.Equal(t, "pi_1", refund.form["payment_intent"])
		assert.Equal(t, "refund:loc_1", refund.idemKey)
	})

	t.Run("Edge: a subscription with no paid invoice is a rejection, nothing is refunded", func(t *testing.T) {
		p, _ := newTestProvider(t, routedStripe(t, map[string]func(http.ResponseWriter, *http.Request){
			"/v1/invoices": jsonReply(&captured{}, 200, emptyListBody),
		}))
		err := p.RefundLatest(context.Background(), billing.RefundRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "refund:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Edge: an invoice with no paid payment is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, routedStripe(t, map[string]func(http.ResponseWriter, *http.Request){
			"/v1/invoices":         jsonReply(&captured{}, 200, invoicesBody),
			"/v1/invoice_payments": jsonReply(&captured{}, 200, noPaymentsBody),
		}))
		err := p.RefundLatest(context.Background(), billing.RefundRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "refund:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a 5xx while listing invoices is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, routedStripe(t, map[string]func(http.ResponseWriter, *http.Request){
			"/v1/invoices": jsonReply(&captured{}, 500, `{"error":{"type":"api_error","message":"boom"}}`),
		}))
		err := p.RefundLatest(context.Background(), billing.RefundRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "refund:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	t.Run("Error: a 400 from the refund call is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, routedStripe(t, map[string]func(http.ResponseWriter, *http.Request){
			"/v1/invoices":         jsonReply(&captured{}, 200, invoicesBody),
			"/v1/invoice_payments": jsonReply(&captured{}, 200, paymentsBody),
			"/v1/refunds":          jsonReply(&captured{}, 400, `{"error":{"type":"invalid_request_error","message":"already refunded"}}`),
		}))
		err := p.RefundLatest(context.Background(), billing.RefundRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "refund:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})
}

func TestProvider_CancelNow(t *testing.T) {
	t.Run("Happy path: cancels the subscription immediately with the idempotency key", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 200, cancelSubscriptionBody))

		err := p.CancelNow(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "cancel:loc_1"})
		require.NoError(t, err)
		assert.Equal(t, "DELETE", rec.method)
		assert.Equal(t, "/v1/subscriptions/sub_1", rec.path)
		assert.Equal(t, "cancel:loc_1", rec.idemKey)
	})

	t.Run("Error: a 404 for an unknown subscription is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 404, `{"error":{"type":"invalid_request_error","message":"no such subscription"}}`))
		err := p.CancelNow(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_missing", IdempotencyKey: "cancel:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a 503 is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 503, `{"error":{"type":"api_error","message":"down"}}`))
		err := p.CancelNow(context.Background(), billing.CancelRequest{ProviderSubscriptionID: "sub_1", IdempotencyKey: "cancel:loc_1"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})
}
