package billing_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCompExpiry struct {
	calls int
	at    time.Time
	err   error
}

func (f *fakeCompExpiry) CancelExpiredComps(_ context.Context, now time.Time) (int, error) {
	f.calls++
	f.at = now
	return 1, f.err
}

type compExpiryObserver struct {
	oldest map[string]time.Duration
	failed int
}

func (o *compExpiryObserver) OldestPending(queue string, age time.Duration) {
	if o.oldest == nil {
		o.oldest = map[string]time.Duration{}
	}
	o.oldest[queue] = age
}

func (o *compExpiryObserver) Failed(string) { o.failed++ }

func runCompExpiry(t *testing.T, ledger billing.CompExpiryLedger, now time.Time) *compExpiryObserver {
	t.Helper()
	queue, err := billing.NewCompExpiryQueue(ledger)
	require.NoError(t, err)
	obs := &compExpiryObserver{}
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", obs, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	require.NoError(t, sw.RunOnce(context.Background(), now))
	return obs
}

func TestCompExpiryQueue(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: one sweep cancels expired comps at the sweeper clock", func(t *testing.T) {
		ledger := &fakeCompExpiry{}
		obs := runCompExpiry(t, ledger, now)

		assert.Equal(t, 1, ledger.calls)
		assert.True(t, ledger.at.Equal(now))
		assert.Equal(t, time.Duration(0), obs.oldest[billing.CompExpiryQueueName])
		assert.Zero(t, obs.failed)
	})

	t.Run("Edge: the queue is named and paced like the other billing queues", func(t *testing.T) {
		queue, err := billing.NewCompExpiryQueue(&fakeCompExpiry{})
		require.NoError(t, err)

		assert.Equal(t, "billing_comp_expiry", queue.Name)
		assert.Equal(t, time.Hour, queue.Interval)
	})

	t.Run("Error: a store failure is counted and does not stop the sweep", func(t *testing.T) {
		ledger := &fakeCompExpiry{err: errors.New("mongo down")}
		obs := runCompExpiry(t, ledger, now)

		assert.Equal(t, 1, ledger.calls)
		assert.Equal(t, 1, obs.failed)
	})
}
