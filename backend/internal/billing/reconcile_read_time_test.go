package billing_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *reconFixture) buildSlow(readFor time.Duration) {
	slow := slowReadProvider{Provider: f.provider, advance: func() { f.clock = f.clock.Add(readFor) }}
	f.reconc = billing.NewReconciler(billing.ReconcileDeps{
		ConsumerDeps: billing.ConsumerDeps{
			Provider: slow,
			Ledger:   f.ledger,
			Tx:       billingtest.NewTransactor(f.ledger, f.hook),
			Outbox:   f.ledger.Outbox(),
			Now:      func() time.Time { return f.clock },
		},
		Claimer:  reconClaimer,
		Observer: f.observer,
		Log:      logger.NewLogger(slog.LevelError),
	})
}

func TestReconciler_ReadTime(t *testing.T) {
	t.Run("Edge: a slow read that started before a fresher sync cannot overwrite it when it returns", func(t *testing.T) {
		f := newReconFixture(t)
		fresh := t1.Add(30 * time.Minute)
		f.ledger.SeedSubscription(billing.Subscription{
			ID:                 reconLocalSub,
			UserID:             reconUserID,
			PlanID:             "paid",
			Provider:           billing.ProviderStripe,
			ProviderCustomerID: reconCustomer,
			ProviderSubID:      reconProviderID,
			Status:             billing.StatusActive,
			ProviderSyncedAt:   &fresh,
		})
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.buildSlow(2 * time.Hour)

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, ok := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		require.True(t, ok)
		assert.Equal(t, billing.StatusActive, sub.Status, "the stale read must not cancel the fresher state")
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(fresh))
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Happy path: a slow read stamps the sync with the time it started", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.buildSlow(2 * time.Hour)

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, ok := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		require.True(t, ok)
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(t1))
	})
}
