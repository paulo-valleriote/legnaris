package billing_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refundProviderSub = "sub_refund_1"

type refundObserver struct {
	mu     sync.Mutex
	oldest map[string]time.Duration
	failed int
}

func (o *refundObserver) OldestPending(queue string, age time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.oldest == nil {
		o.oldest = map[string]time.Duration{}
	}
	o.oldest[queue] = age
}

func (o *refundObserver) Failed(string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failed++
}

type refundRig struct {
	ledger   *billingtest.Ledger
	provider *billingtest.Provider
	obs      *refundObserver
	sub      billing.Subscription
}

func newRefundRig(t *testing.T, state billing.RefundState) *refundRig {
	t.Helper()
	ledger := billingtest.NewLedger()
	provider := billingtest.NewProvider()
	requested := refundClock
	sub := billing.Subscription{
		ID:                 consumerLocalSub,
		UserID:             consumerUserID,
		PlanID:             "paid",
		Provider:           billing.ProviderStripe,
		ProviderSubID:      refundProviderSub,
		ProviderCustomerID: consumerCustomer,
		Status:             billing.StatusActive,
		FirstPaidAt:        paidAt(time.Hour),
		RefundState:        state,
		RefundRequestedAt:  &requested,
		RefundUsedAt:       &requested,
		RefundNextAttempt:  refundClock,
		CreatedAt:          refundClock,
		UpdatedAt:          refundClock,
	}
	ledger.SeedSubscription(sub)
	return &refundRig{ledger: ledger, provider: provider, obs: &refundObserver{}, sub: sub}
}

func (r *refundRig) sweep(t *testing.T, attemptCap int, at time.Time) {
	t.Helper()
	queue, err := billing.NewRefundQueue(billing.RefundDeps{Provider: r.provider, Ledger: r.ledger}, attemptCap, 10)
	require.NoError(t, err)
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", r.obs, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	require.NoError(t, sw.RunOnce(context.Background(), at))
}

func (r *refundRig) stored(t *testing.T) billing.Subscription {
	t.Helper()
	subs := r.ledger.Subscriptions(consumerUserID)
	require.Len(t, subs, 1)
	return subs[0]
}

func TestRefundQueue_Drive(t *testing.T) {
	const attemptCap = 10
	outage := fmt.Errorf("stripe: %w", billing.ErrProviderUnavailable)
	rejected := fmt.Errorf("stripe: %w", billing.ErrProviderRejected)

	t.Run("Happy path: a requested refund is refunded then canceled, each with its idempotency key", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		r.sweep(t, attemptCap, refundClock)

		assert.Equal(t, []string{"refund:" + consumerLocalSub}, r.provider.RefundKeys())
		assert.Equal(t, []string{"cancel:" + consumerLocalSub}, r.provider.CancelKeys())
		assert.Equal(t, billing.RefundCanceled, r.stored(t).RefundState)
	})

	t.Run("Edge: a canceled refund is not driven again on the next pass", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundCanceled)
		r.sweep(t, attemptCap, refundClock)
		r.sweep(t, attemptCap, refundClock.Add(time.Minute))

		assert.Empty(t, r.provider.RefundKeys())
		assert.Empty(t, r.provider.CancelKeys())
	})

	t.Run("Edge: a refund whose next attempt is in the future is not claimed", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		sub := r.stored(t)
		sub.RefundNextAttempt = refundClock.Add(time.Hour)
		r.ledger.SeedSubscription(sub)

		r.sweep(t, attemptCap, refundClock)

		assert.Empty(t, r.provider.RefundKeys())
		assert.Equal(t, billing.RefundRequested, r.stored(t).RefundState)
	})

	t.Run("Edge: a failed refund is never claimed again", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundFailed)
		r.sweep(t, attemptCap, refundClock)

		assert.Empty(t, r.provider.RefundKeys())
		assert.Empty(t, r.provider.CancelKeys())
	})

	t.Run("Edge: a dead refunded cancel is not claimed again", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRefunded)
		sub := r.stored(t)
		sub.RefundDead = true
		r.ledger.SeedSubscription(sub)

		r.sweep(t, attemptCap, refundClock)

		assert.Empty(t, r.provider.CancelKeys())
	})

	t.Run("Happy path: a provider outage keeps the refund requested, and recovery completes it", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		r.provider.RefundErr = outage
		r.sweep(t, attemptCap, refundClock)

		afterOutage := r.stored(t)
		assert.Equal(t, billing.RefundRequested, afterOutage.RefundState)
		assert.Equal(t, 1, afterOutage.RefundAttempts)
		assert.NotEmpty(t, afterOutage.RefundLastError)
		assert.NotNil(t, afterOutage.RefundUsedAt, "the right stays spent while the refund is retried")

		r.provider.RefundErr = nil
		r.sweep(t, attemptCap, refundClock.Add(time.Minute))

		assert.Equal(t, billing.RefundCanceled, r.stored(t).RefundState)
		assert.Equal(t, []string{"refund:" + consumerLocalSub}, r.provider.RefundKeys())
	})

	t.Run("Error: a permanent refund rejection fails the refund and returns the right", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		r.provider.RefundErr = rejected
		r.sweep(t, attemptCap, refundClock)

		got := r.stored(t)
		assert.Equal(t, billing.RefundFailed, got.RefundState)
		assert.Nil(t, got.RefundUsedAt, "the refund right is returned")
		assert.True(t, got.RefundDead)
		assert.Empty(t, r.provider.CancelKeys())
		assert.Equal(t, 1, r.obs.failed)
	})

	t.Run("Error: a permanent cancel rejection after the refund stops the item without reverting the refund", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRefunded)
		r.provider.CancelErr = rejected
		r.sweep(t, attemptCap, refundClock)

		got := r.stored(t)
		assert.Equal(t, billing.RefundRefunded, got.RefundState)
		assert.True(t, got.RefundDead)
		assert.NotNil(t, got.RefundUsedAt)
	})

	t.Run("Edge: a transient outage that reaches the attempt attemptCap fails the refund and returns the right", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		r.provider.RefundErr = outage
		r.sweep(t, 2, refundClock)
		r.sweep(t, 2, refundClock.Add(time.Minute))

		got := r.stored(t)
		assert.Equal(t, billing.RefundFailed, got.RefundState)
		assert.Nil(t, got.RefundUsedAt)
	})

	t.Run("Happy path: the oldest requested refund age is reported for the alert", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundRequested)
		r.provider.RefundErr = outage
		at := refundClock.Add(90 * time.Minute)
		r.sweep(t, attemptCap, at)

		assert.Equal(t, 90*time.Minute, r.obs.oldest[billing.RefundQueueName])
	})

	t.Run("Edge: with no requested refund the oldest age is zero", func(t *testing.T) {
		r := newRefundRig(t, billing.RefundCanceled)
		r.sweep(t, attemptCap, refundClock)

		assert.Equal(t, time.Duration(0), r.obs.oldest[billing.RefundQueueName])
	})
}
