package billing_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSubscriptionLister struct {
	subs []billing.Subscription
	err  error
}

func (f fakeSubscriptionLister) SubscriptionsOf(context.Context, string) ([]billing.Subscription, error) {
	return f.subs, f.err
}

func TestPlanReader_EffectivePlanID(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	t.Run("Happy path: an active comp inside its window keeps the plan paid", func(t *testing.T) {
		reader := billing.NewPlanReader(fakeSubscriptionLister{subs: []billing.Subscription{
			{Provider: billing.ProviderComp, Status: billing.StatusActive, CurrentPeriodEnd: &future},
		}}, func() time.Time { return now })

		got, err := reader.EffectivePlanID(context.Background(), "u1")
		require.NoError(t, err)
		assert.Equal(t, plan.IDPaid, got)
	})

	t.Run("Edge: an expired comp reads as free", func(t *testing.T) {
		reader := billing.NewPlanReader(fakeSubscriptionLister{subs: []billing.Subscription{
			{Provider: billing.ProviderComp, Status: billing.StatusActive, CurrentPeriodEnd: &past},
		}}, func() time.Time { return now })

		got, err := reader.EffectivePlanID(context.Background(), "u1")
		require.NoError(t, err)
		assert.Equal(t, plan.IDFree, got)
	})

	t.Run("Edge: no subscriptions reads as free", func(t *testing.T) {
		reader := billing.NewPlanReader(fakeSubscriptionLister{}, func() time.Time { return now })

		got, err := reader.EffectivePlanID(context.Background(), "u1")
		require.NoError(t, err)
		assert.Equal(t, plan.IDFree, got)
	})

	t.Run("Error: a store failure is reported", func(t *testing.T) {
		reader := billing.NewPlanReader(fakeSubscriptionLister{err: assert.AnError}, func() time.Time { return now })

		_, err := reader.EffectivePlanID(context.Background(), "u1")
		require.ErrorIs(t, err, assert.AnError)
	})
}
