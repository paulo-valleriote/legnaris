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

func summarySub(id string, status billing.Status, paidAgo time.Duration) billing.Subscription {
	paid := time.Now().Add(-paidAgo)
	return billing.Subscription{
		ID:          id,
		UserID:      consumerUserID,
		Status:      status,
		FirstPaidAt: &paid,
		UpdatedAt:   time.Now(),
	}
}

func TestStatusReader_RefundSummary(t *testing.T) {
	t.Run("Happy path: an active subscription paid two days ago is refund eligible", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		ledger.SeedSubscription(summarySub(consumerLocalSub, billing.StatusActive, 48*time.Hour))

		got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		assert.True(t, got.RefundEligible)
		assert.Empty(t, got.RefundState)
	})

	t.Run("Edge: a subscription paid eight days ago is not refund eligible", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		ledger.SeedSubscription(summarySub(consumerLocalSub, billing.StatusActive, 8*24*time.Hour))

		got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		assert.False(t, got.RefundEligible)
	})

	t.Run("Edge: a refund already requested shows its state and is not eligible", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		sub := summarySub(consumerLocalSub, billing.StatusActive, time.Hour)
		sub.RefundState = billing.RefundRequested
		ledger.SeedSubscription(sub)

		got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		assert.Equal(t, billing.RefundRequested, got.RefundState)
		assert.False(t, got.RefundEligible)
	})

	t.Run("Edge: a refund used on another subscription of the user blocks eligibility", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		used := time.Now().Add(-30 * 24 * time.Hour)
		old := summarySub("0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e20", billing.StatusCanceled, 60*24*time.Hour)
		old.RefundUsedAt = &used
		ledger.SeedSubscription(old)
		ledger.SeedSubscription(summarySub(consumerLocalSub, billing.StatusActive, time.Hour))

		got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		assert.False(t, got.RefundEligible)
	})

	t.Run("Edge: a user with no subscription has no refund", func(t *testing.T) {
		got, err := billing.NewStatusReader(billingtest.NewLedger()).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		assert.False(t, got.RefundEligible)
		assert.Empty(t, got.RefundState)
	})
}
