package billing_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopObserver struct{}

func (noopObserver) OldestPending(string, time.Duration) {}
func (noopObserver) Failed(string)                       {}

func queueFixture(t *testing.T, attemptCap int) (*billingtest.Ledger, *billingtest.Provider, sweeper.Queue) {
	t.Helper()
	ledger := billingtest.NewLedger()
	ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "free"})
	provider := billingtest.NewProvider()
	provider.SetSubscription(billing.ProviderSubscription{ID: consumerProviderID, Status: billing.StatusActive})
	deps := billing.ConsumerDeps{
		Provider: provider,
		Ledger:   ledger,
		Tx:       billingtest.NewTransactor(ledger, nil),
		Outbox:   ledger.Outbox(),
		Now:      func() time.Time { return t1 },
	}
	queue, err := billing.NewConsumerQueue(deps, attemptCap, 10)
	require.NoError(t, err)
	return ledger, provider, queue
}

func runQueueOnce(t *testing.T, q sweeper.Queue, now time.Time) {
	t.Helper()
	s, err := sweeper.New([]sweeper.Queue{q}, "relay-test", noopObserver{}, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	_ = s.RunOnce(context.Background(), now)
}

func seedReceived(t *testing.T, ledger *billingtest.Ledger, id, userID, providerEventID string) {
	t.Helper()
	require.NoError(t, ledger.InsertEvent(context.Background(), billing.Event{
		ID:                     id,
		Provider:               billing.ProviderStripe,
		ProviderEventID:        providerEventID,
		Kind:                   billing.KindSubscriptionChanged,
		UserID:                 userID,
		ProviderSubscriptionID: consumerProviderID,
		Status:                 billing.EventReceived,
		NextAttemptAt:          t1,
		ReceivedAt:             t1,
		ExpiresAt:              t1.Add(time.Hour),
	}))
}

func TestConsumerQueue(t *testing.T) {
	t.Run("Happy path: a due event is handled in a pass and the account is planned", func(t *testing.T) {
		ledger, _, q := queueFixture(t, 5)
		seedReceived(t, ledger, "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e21", consumerUserID, "evt_q_1")

		runQueueOnce(t, q, t1)

		ev, _ := ledger.Event("0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e21")
		assert.Equal(t, billing.EventProcessed, ev.Status)
		acc, _ := ledger.Account(consumerUserID)
		assert.Equal(t, "paid", acc.Plan)
	})

	t.Run("Edge: two events for one user are handled one per pass, never concurrently", func(t *testing.T) {
		ledger, _, q := queueFixture(t, 5)
		first := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e22"
		second := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e23"
		seedReceived(t, ledger, first, consumerUserID, "evt_q_2")
		seedReceived(t, ledger, second, consumerUserID, "evt_q_3")

		runQueueOnce(t, q, t1)

		evFirst, _ := ledger.Event(first)
		evSecond, _ := ledger.Event(second)
		assert.Equal(t, billing.EventProcessed, evFirst.Status)
		assert.Equal(t, billing.EventReceived, evSecond.Status)

		runQueueOnce(t, q, t1)
		evSecond, _ = ledger.Event(second)
		assert.Equal(t, billing.EventProcessed, evSecond.Status)
	})

	t.Run("Edge: a user whose lock is still held is skipped until the lease expires", func(t *testing.T) {
		ledger, _, q := queueFixture(t, 5)
		id := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e24"
		seedReceived(t, ledger, id, consumerUserID, "evt_q_4")
		ctx := context.Background()
		claimed, err := ledger.ClaimEvents(ctx, "other-relay", t1, time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)

		runQueueOnce(t, q, t1)
		ev, _ := ledger.Event(id)
		assert.Equal(t, billing.EventReceived, ev.Status)

		runQueueOnce(t, q, t1.Add(2*time.Minute))
		ev, _ = ledger.Event(id)
		assert.Equal(t, billing.EventProcessed, ev.Status)
	})

	t.Run("Error: a provider outage reschedules the event and counts an attempt", func(t *testing.T) {
		ledger, provider, q := queueFixture(t, 5)
		provider.Unavailable = true
		id := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e25"
		seedReceived(t, ledger, id, consumerUserID, "evt_q_5")

		runQueueOnce(t, q, t1)

		ev, _ := ledger.Event(id)
		assert.Equal(t, billing.EventReceived, ev.Status)
		assert.Equal(t, 1, ev.Attempts)
		assert.Contains(t, ev.LastError, billing.ErrProviderUnavailable.Error())
	})

	t.Run("Error: reaching the attempt cap marks the event failed", func(t *testing.T) {
		ledger, provider, q := queueFixture(t, 1)
		provider.Unavailable = true
		id := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e26"
		seedReceived(t, ledger, id, consumerUserID, "evt_q_6")

		runQueueOnce(t, q, t1)

		ev, _ := ledger.Event(id)
		assert.Equal(t, billing.EventFailed, ev.Status)
	})

	t.Run("Error: a permanent provider rejection marks the event failed at once", func(t *testing.T) {
		ledger, _, q := queueFixture(t, 5)
		id := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e27"
		require.NoError(t, ledger.InsertEvent(context.Background(), billing.Event{
			ID: id, Provider: billing.ProviderStripe, ProviderEventID: "evt_q_7",
			Kind: billing.KindSubscriptionChanged, UserID: consumerUserID,
			ProviderSubscriptionID: "sub_missing", Status: billing.EventReceived,
			NextAttemptAt: t1, ReceivedAt: t1,
		}))

		runQueueOnce(t, q, t1)

		ev, _ := ledger.Event(id)
		assert.Equal(t, billing.EventFailed, ev.Status)
	})
}

func TestNewConsumerQueue_Edge(t *testing.T) {
	t.Run("Error: a non-positive attempt cap is rejected", func(t *testing.T) {
		_, err := billing.NewConsumerQueue(billing.ConsumerDeps{}, 0, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, sweeper.ErrInvalidQueue)
	})
}
