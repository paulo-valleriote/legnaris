package billing_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type slowReadProvider struct {
	billing.Provider
	advance func()
}

func (p slowReadProvider) GetSubscription(ctx context.Context, id string) (billing.ProviderSubscription, error) {
	p.advance()
	return p.Provider.GetSubscription(ctx, id)
}

func (f *consumerFixture) buildSlow(readFor time.Duration) {
	slow := slowReadProvider{Provider: f.provider, advance: func() { f.clock = f.clock.Add(readFor) }}
	f.consumer = billing.NewConsumer(billing.ConsumerDeps{
		Provider: slow,
		Ledger:   f.ledger,
		Tx:       billingtest.NewTransactor(f.ledger, f.hook),
		Outbox:   f.ledger.Outbox(),
		Now:      func() time.Time { return f.clock },
	})
}

func TestConsumer_ReadTime(t *testing.T) {
	t.Run("Happy path: the provider sync time is when the read started, not when it returned", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.buildSlow(2 * time.Hour)

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(t1))
	})

	t.Run("Edge: a slow read that started before a fresher sync cannot overwrite it when it returns", func(t *testing.T) {
		f := newConsumerFixture(t)
		fresh := t1.Add(30 * time.Minute)
		f.ledger.SeedSubscription(billing.Subscription{
			ID:                 consumerLocalSub,
			UserID:             consumerUserID,
			PlanID:             "paid",
			Provider:           billing.ProviderStripe,
			ProviderCustomerID: consumerCustomer,
			ProviderSubID:      consumerProviderID,
			Status:             billing.StatusActive,
			FirstPaidAt:        &t1,
			ProviderSyncedAt:   &fresh,
		})
		f.providerSays(billing.StatusCanceled, false)
		f.buildSlow(2 * time.Hour)

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		assert.Equal(t, billing.StatusActive, sub.Status, "the stale read must not cancel the fresher state")
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(fresh))
	})
}
