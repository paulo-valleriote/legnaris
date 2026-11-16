package stripe

import (
	"context"
	"testing"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_CancelNow(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{
			name:   "Happy path: cancels the subscription immediately with the idempotency key",
			status: 200,
			body:   `{"id":"sub_1","object":"subscription","status":"canceled"}`,
		},
		{
			name:    "Error: a 400 from Stripe, such as an already canceled subscription, is a rejection",
			status:  400,
			body:    `{"error":{"type":"invalid_request_error","message":"already canceled"}}`,
			wantErr: billing.ErrProviderRejected,
		},
		{
			name:    "Error: a 5xx from Stripe is unavailable",
			status:  500,
			body:    `{"error":{"type":"api_error","message":"boom"}}`,
			wantErr: billing.ErrProviderUnavailable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &captured{}
			p, _ := newTestProvider(t, respondWith(rec, tc.status, tc.body))

			err := p.CancelNow(context.Background(), billing.CancelRequest{
				IdempotencyKey:         "cancel:sub_local_1",
				ProviderSubscriptionID: "sub_1",
			})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "DELETE", rec.method)
			assert.Equal(t, "/v1/subscriptions/sub_1", rec.path)
			assert.Equal(t, "cancel:sub_local_1", rec.idemKey)
		})
	}
}

func TestProvider_ScrubCustomer(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{
			name:   "Happy path: unsets the personal fields and the user metadata on the customer",
			status: 200,
			body:   `{"id":"cus_1","object":"customer"}`,
		},
		{
			name:    "Error: a 404 for a customer that Stripe no longer has is a rejection",
			status:  404,
			body:    `{"error":{"type":"invalid_request_error","message":"no such customer"}}`,
			wantErr: billing.ErrProviderRejected,
		},
		{
			name:    "Error: a 503 from Stripe is unavailable",
			status:  503,
			body:    `{"error":{"type":"api_error","message":"down"}}`,
			wantErr: billing.ErrProviderUnavailable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &captured{}
			p, _ := newTestProvider(t, respondWith(rec, tc.status, tc.body))

			err := p.ScrubCustomer(context.Background(), billing.ScrubRequest{
				IdempotencyKey: "scrub:cus_1",
				CustomerID:     "cus_1",
			})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "POST", rec.method)
			assert.Equal(t, "/v1/customers/cus_1", rec.path)
			assert.Equal(t, "scrub:cus_1", rec.idemKey)
			for _, field := range []string{"name", "email", "phone", "description", "metadata[user_id]"} {
				value, sent := rec.form[field]
				assert.True(t, sent, "%s must be sent so Stripe unsets it", field)
				assert.Empty(t, value, field)
			}
		})
	}
}
