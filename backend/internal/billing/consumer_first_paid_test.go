package billing_test

import (
	"testing"

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
