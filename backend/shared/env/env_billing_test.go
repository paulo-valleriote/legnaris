package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillingEnv(t *testing.T) {
	t.Run("Edge: unset billing variables default to empty so billing stays unconfigured", func(t *testing.T) {
		for _, k := range []string{"STRIPE_SECRET_KEY", "STRIPE_PRICE_ID_USD", "BILLING_SUCCESS_URL", "BILLING_CANCEL_URL"} {
			unsetEnv(t, k)
		}
		e := newTestEnv()
		vars := e.buildVariables()
		require.Empty(t, vars.StripeSecretKey)
		require.Empty(t, vars.StripePriceIDUSD)
		require.Empty(t, vars.BillingSuccessURL)
		require.Empty(t, vars.BillingCancelURL)
	})

	t.Run("Happy path: billing variables are read from the environment", func(t *testing.T) {
		t.Setenv("STRIPE_SECRET_KEY", "sk_test_x")
		t.Setenv("STRIPE_PRICE_ID_USD", "price_x")
		t.Setenv("BILLING_SUCCESS_URL", "https://app.example/ok")
		t.Setenv("BILLING_CANCEL_URL", "https://app.example/cancel")
		e := newTestEnv()
		vars := e.buildVariables()
		require.Equal(t, "sk_test_x", vars.StripeSecretKey)
		require.Equal(t, "price_x", vars.StripePriceIDUSD)
		require.Equal(t, "https://app.example/ok", vars.BillingSuccessURL)
		require.Equal(t, "https://app.example/cancel", vars.BillingCancelURL)
	})
}
