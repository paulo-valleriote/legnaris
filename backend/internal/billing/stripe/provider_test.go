package stripe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripesdk "github.com/stripe/stripe-go/v87"
)

type captured struct {
	method  string
	path    string
	auth    string
	idemKey string
	form    map[string]string
}

func newTestProvider(t *testing.T, handler http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := stripesdk.NewClient("sk_test_adapter", stripesdk.WithBackends(
		stripesdk.NewBackendsWithConfig(&stripesdk.BackendConfig{
			URL:               stripesdk.String(srv.URL),
			HTTPClient:        srv.Client(),
			MaxNetworkRetries: stripesdk.Int64(0),
		}),
	))
	return &Provider{client: client}, srv
}

func respondWith(rec *captured, status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		rec.idemKey = r.Header.Get("Idempotency-Key")
		rec.form = map[string]string{}
		for k := range r.PostForm {
			rec.form[k] = r.PostForm.Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestProvider_CreateCustomer(t *testing.T) {
	t.Run("Happy path: creates a customer with the idempotency key and user metadata", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 200, `{"id":"cus_1","object":"customer"}`))

		got, err := p.CreateCustomer(context.Background(), billing.CustomerRequest{
			IdempotencyKey: "customer:u-1", UserID: "u-1",
		})
		require.NoError(t, err)
		assert.Equal(t, "cus_1", got.ID)
		assert.Equal(t, "Bearer sk_test_adapter", rec.auth)
		assert.Equal(t, "customer:u-1", rec.idemKey)
		assert.Equal(t, "u-1", rec.form["metadata[user_id]"])
		assert.Equal(t, "/v1/customers", rec.path)
	})

	t.Run("Error: a 400 from Stripe is a rejection", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 400,
			`{"error":{"type":"invalid_request_error","message":"bad"}}`))
		_, err := p.CreateCustomer(context.Background(), billing.CustomerRequest{IdempotencyKey: "k", UserID: "u"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
		assert.False(t, errors.Is(err, billing.ErrProviderUnavailable))
	})

	t.Run("Error: a 5xx from Stripe is unavailable", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 500,
			`{"error":{"type":"api_error","message":"boom"}}`))
		_, err := p.CreateCustomer(context.Background(), billing.CustomerRequest{IdempotencyKey: "k", UserID: "u"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})

	for _, status := range []int{409, 429} {
		t.Run("Error: a retryable status is unavailable", func(t *testing.T) {
			p, _ := newTestProvider(t, respondWith(&captured{}, status,
				`{"error":{"type":"idempotency_error","message":"busy"}}`))
			_, err := p.CreateCustomer(context.Background(), billing.CustomerRequest{IdempotencyKey: "k", UserID: "u"})
			require.ErrorIs(t, err, billing.ErrProviderUnavailable)
		})
	}

	t.Run("Error: a closed connection is unavailable", func(t *testing.T) {
		p, srv := newTestProvider(t, respondWith(&captured{}, 200, `{}`))
		srv.Close()
		_, err := p.CreateCustomer(context.Background(), billing.CustomerRequest{IdempotencyKey: "k", UserID: "u"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})
}

func TestProvider_CreateCheckoutSession(t *testing.T) {
	expires := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)

	t.Run("Happy path: sends a card-only subscription session and maps the URL and expiry", func(t *testing.T) {
		rec := &captured{}
		body := `{"id":"cs_1","object":"checkout.session","url":"https://checkout.stripe.test/c/cs_1","expires_at":` +
			strconv.FormatInt(expires.Unix(), 10) + `}`
		p, _ := newTestProvider(t, respondWith(rec, 200, body))

		got, err := p.CreateCheckoutSession(context.Background(), billing.CheckoutRequest{
			IdempotencyKey: "checkout:s-1",
			UserID:         "u-1",
			SubscriptionID: "s-1",
			CustomerID:     "cus_1",
			PriceID:        "price_test_usd",
			SuccessURL:     "https://app.example/ok",
			CancelURL:      "https://app.example/cancel",
		})
		require.NoError(t, err)
		assert.Equal(t, "cs_1", got.ID)
		assert.Equal(t, "https://checkout.stripe.test/c/cs_1", got.URL)
		assert.True(t, expires.Equal(got.ExpiresAt))

		assert.Equal(t, "/v1/checkout/sessions", rec.path)
		assert.Equal(t, "checkout:s-1", rec.idemKey)
		assert.Equal(t, "subscription", rec.form["mode"])
		assert.Equal(t, "cus_1", rec.form["customer"])
		assert.Equal(t, "s-1", rec.form["client_reference_id"])
		assert.Equal(t, "card", rec.form["allowed_payment_method_types[0]"])
		assert.Equal(t, "price_test_usd", rec.form["line_items[0][price]"])
		assert.Equal(t, "1", rec.form["line_items[0][quantity]"])
		assert.Equal(t, "https://app.example/ok", rec.form["success_url"])
		assert.Equal(t, "https://app.example/cancel", rec.form["cancel_url"])
		assert.Equal(t, "u-1", rec.form["metadata[user_id]"])
	})

	t.Run("Happy path: sends the hosted page settings chosen in Checkout Studio", func(t *testing.T) {
		rec := &captured{}
		p, _ := newTestProvider(t, respondWith(rec, 200, `{"id":"cs_1","object":"checkout.session"}`))

		_, err := p.CreateCheckoutSession(context.Background(), billing.CheckoutRequest{IdempotencyKey: "checkout:s-1"})
		require.NoError(t, err)

		want := map[string]string{
			"ui_mode":                                           "hosted_page",
			"billing_address_collection":                        "auto",
			"phone_number_collection[enabled]":                  "false",
			"automatic_tax[enabled]":                            "false",
			"allow_promotion_codes":                             "false",
			"payment_method_collection":                         "always",
			"submit_type":                                       "auto",
			"saved_payment_method_options[payment_method_save]": "enabled",
			"integration_identifier":                            "hosted_web_0001",
			"origin_context":                                    "web",
		}
		for field, value := range want {
			assert.Equal(t, value, rec.form[field], field)
		}
	})

	t.Run("Error: a 402 from Stripe is a rejection", func(t *testing.T) {
		p, _ := newTestProvider(t, respondWith(&captured{}, 402,
			`{"error":{"type":"card_error","message":"declined"}}`))
		_, err := p.CreateCheckoutSession(context.Background(), billing.CheckoutRequest{IdempotencyKey: "k"})
		require.ErrorIs(t, err, billing.ErrProviderRejected)
	})

	t.Run("Error: a timeout on the request is unavailable", func(t *testing.T) {
		p, srv := newTestProvider(t, respondWith(&captured{}, 200, `{}`))
		srv.Close()
		_, err := p.CreateCheckoutSession(context.Background(), billing.CheckoutRequest{IdempotencyKey: "k"})
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
	})
}

func TestNew(t *testing.T) {
	t.Run("Edge: an empty secret key is a missing configuration", func(t *testing.T) {
		_, err := New(Config{SecretKey: ""})
		require.ErrorIs(t, err, ErrMissingConfiguration)
	})

	t.Run("Happy path: a secret key builds a provider", func(t *testing.T) {
		p, err := New(Config{SecretKey: "sk_test_x"})
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
}
