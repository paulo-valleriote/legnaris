package billing_test

import (
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumer_FirstPaidAt(t *testing.T) {
	t.Run("Happy path: the first active read stamps the first payment with the read time", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.FirstPaidAt)
		assert.True(t, sub.FirstPaidAt.Equal(t1))
	})

	t.Run("Edge: a later active read keeps the original first payment time", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.build()
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		f.clock = t2
		f.build()
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.FirstPaidAt)
		assert.True(t, sub.FirstPaidAt.Equal(t1), "the first payment never moves")
	})

	t.Run("Edge: a subscription that never became active has no first payment", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusPastDue, false)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		assert.Nil(t, f.localSub(t).FirstPaidAt)
	})
}

func TestConsumer_FirstPaidAtFromProvider(t *testing.T) {
	t.Run("Happy path: the first payment is the provider's subscription start, not the read time", func(t *testing.T) {
		f := newConsumerFixture(t)
		start := t1.Add(-2 * time.Hour)
		f.provider.SetSubscription(billing.ProviderSubscription{
			ID: consumerProviderID, CustomerID: consumerCustomer, Status: billing.StatusActive,
			CurrentPeriodEnd: t1.AddDate(0, 1, 0), StartedAt: start,
		})
		f.clock = t2
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.FirstPaidAt)
		assert.True(t, sub.FirstPaidAt.Equal(start))
	})

	t.Run("Edge: a delayed read keeps the provider's first payment, so the refund window does not extend", func(t *testing.T) {
		f := newConsumerFixture(t)
		start := t1.Add(-3 * 24 * time.Hour)
		f.provider.SetSubscription(billing.ProviderSubscription{
			ID: consumerProviderID, CustomerID: consumerCustomer, Status: billing.StatusActive,
			CurrentPeriodEnd: t1.AddDate(0, 1, 0), StartedAt: start,
		})
		f.clock = t1.Add(6 * 24 * time.Hour)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.FirstPaidAt)
		assert.True(t, sub.FirstPaidAt.Equal(start))
		assert.False(t, billing.RefundOpen(sub, f.clock), "the window closed four days after the payment")
	})

	t.Run("Edge: a provider that supplies no start time falls back to the read time", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.clock = t2
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.FirstPaidAt)
		assert.True(t, sub.FirstPaidAt.Equal(t2))
	})
}
