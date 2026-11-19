package billing_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCompExpiryRunner struct {
	calls int
	at    time.Time
	count int
	err   error
}

func (f *fakeCompExpiryRunner) Expire(_ context.Context, now time.Time) (int, error) {
	f.calls++
	f.at = now
	return f.count, f.err
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

func runCompExpiry(t *testing.T, runner billing.CompExpiryRunner, now time.Time) *compExpiryObserver {
	t.Helper()
	queue, err := billing.NewCompExpiryQueue(runner)
	require.NoError(t, err)
	obs := &compExpiryObserver{}
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", obs, nil)
	require.NoError(t, err)
	require.NoError(t, sw.RunOnce(context.Background(), now))
	return obs
}

func planChangesIn(t *testing.T, ledger *billingtest.Ledger) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, entry := range ledger.OutboxEntries() {
		ev, err := events.Decode(entry.Body)
		require.NoError(t, err)
		if ev.Type != billing.PlanChangedEvent {
			continue
		}
		var data map[string]any
		require.NoError(t, json.Unmarshal(ev.Data, &data))
		data["aggregate_id"] = ev.Aggregate.ID
		out = append(out, data)
	}
	return out
}

func expiredComp(t *testing.T, userID string, end time.Time) billing.Subscription {
	t.Helper()
	return billing.Subscription{
		ID:               userID + "-comp",
		UserID:           userID,
		PlanID:           "paid",
		Provider:         billing.ProviderComp,
		Status:           billing.StatusActive,
		CurrentPeriodEnd: &end,
	}
}

func newCompUserID(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func TestCompExpiry(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)

	t.Run("Happy path: expired comps are canceled and their users downgraded with plan.changed", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		u1 := newCompUserID(t)
		u2 := newCompUserID(t)
		ledger.AddAccount(u1, billing.AccountPlan{Plan: "paid"})
		ledger.AddAccount(u2, billing.AccountPlan{Plan: "paid"})
		ledger.SeedSubscription(expiredComp(t, u1, expired))
		ledger.SeedSubscription(expiredComp(t, u2, expired.Add(-time.Minute)))
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		for _, userID := range []string{u1, u2} {
			acc, _ := ledger.Account(userID)
			assert.Equal(t, "free", acc.Plan)
			subs := ledger.Subscriptions(userID)
			require.Len(t, subs, 1)
			assert.Equal(t, billing.StatusCanceled, subs[0].Status)
		}
		changes := planChangesIn(t, ledger)
		require.Len(t, changes, 2)
		assert.Equal(t, "free", changes[0]["to"])
		assert.Equal(t, "free", changes[1]["to"])
	})

	t.Run("Edge: an expired comp never downgrades a user with a live stripe subscription", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		u1 := newCompUserID(t)
		ledger.AddAccount(u1, billing.AccountPlan{Plan: "paid"})
		ledger.SeedSubscription(expiredComp(t, u1, expired))
		ledger.SeedSubscription(billing.Subscription{
			ID:       u1 + "-stripe",
			UserID:   u1,
			Provider: billing.ProviderStripe,
			Status:   billing.StatusActive,
		})
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		acc, _ := ledger.Account(u1)
		assert.Equal(t, "paid", acc.Plan)
		assert.Empty(t, planChangesIn(t, ledger))
	})

	t.Run("Edge: a perpetual comp never expires", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		u1 := newCompUserID(t)
		ledger.AddAccount(u1, billing.AccountPlan{Plan: "paid"})
		ledger.SeedSubscription(expiredComp(t, u1, now.Add(time.Hour)))
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("Edge: a deleting account is canceled without a plan change", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		u1 := newCompUserID(t)
		ledger.AddAccount(u1, billing.AccountPlan{Plan: "paid", Deleting: true})
		ledger.SeedSubscription(expiredComp(t, u1, expired))
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		acc, _ := ledger.Account(u1)
		assert.Equal(t, "paid", acc.Plan)
		assert.Empty(t, planChangesIn(t, ledger))
	})

	t.Run("Edge: more expired comps than one batch are all processed through the loop", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		total := billing.CompExpiryBatchSize + 1
		for i := 0; i < total; i++ {
			userID := newCompUserID(t)
			ledger.AddAccount(userID, billing.AccountPlan{Plan: "paid"})
			ledger.SeedSubscription(expiredComp(t, userID, expired))
		}
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Equal(t, total, count)
		assert.Len(t, planChangesIn(t, ledger), total)
	})

	t.Run("Error: a crash after the cancel rolls back the comp and the plan", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		u1 := newCompUserID(t)
		ledger.AddAccount(u1, billing.AccountPlan{Plan: "paid"})
		ledger.SeedSubscription(expiredComp(t, u1, expired))
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: ledger,
			Outbox: ledger.Outbox(),
			Tx:     billingtest.NewTransactor(ledger, failAt(db.StepAfterEntityWrite)),
		})

		_, err := expiry.Expire(context.Background(), now)
		require.Error(t, err)

		acc, _ := ledger.Account(u1)
		assert.Equal(t, "paid", acc.Plan)
		subs := ledger.Subscriptions(u1)
		require.Len(t, subs, 1)
		assert.Equal(t, billing.StatusActive, subs[0].Status)
		assert.Empty(t, planChangesIn(t, ledger))
	})

	t.Run("Edge: a batch that makes no progress stops instead of spinning", func(t *testing.T) {
		rows := make([]billing.Subscription, billing.CompExpiryBatchSize)
		for i := range rows {
			rows[i] = expiredComp(t, newCompUserID(t), expired)
		}
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: stuckCompExpiryLedger{rows: rows},
			Tx:     billingtest.NewTransactor(billingtest.NewLedger(), nil),
		})

		count, err := expiry.Expire(context.Background(), now)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("Error: a store failure is reported", func(t *testing.T) {
		expiry := billing.NewCompExpiry(billing.CompExpiryDeps{
			Ledger: errorCompExpiryLedger{err: assert.AnError},
			Tx:     billingtest.NewTransactor(billingtest.NewLedger(), nil),
		})

		_, err := expiry.Expire(context.Background(), now)
		require.ErrorIs(t, err, assert.AnError)
	})
}

type errorCompExpiryLedger struct {
	err error
}

func (e errorCompExpiryLedger) AccountPlan(context.Context, string) (billing.AccountPlan, error) {
	return billing.AccountPlan{}, e.err
}

func (e errorCompExpiryLedger) SubscriptionsOf(context.Context, string) ([]billing.Subscription, error) {
	return nil, e.err
}

func (e errorCompExpiryLedger) SetPlan(context.Context, string, string) (int64, error) {
	return 0, e.err
}

func (e errorCompExpiryLedger) ExpiredComps(context.Context, time.Time, int) ([]billing.Subscription, error) {
	return nil, e.err
}

func (e errorCompExpiryLedger) CancelComp(context.Context, string, time.Time) (bool, error) {
	return false, e.err
}

type stuckCompExpiryLedger struct {
	errorCompExpiryLedger
	rows []billing.Subscription
}

func (s stuckCompExpiryLedger) ExpiredComps(context.Context, time.Time, int) ([]billing.Subscription, error) {
	return s.rows, nil
}

func (s stuckCompExpiryLedger) CancelComp(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func TestCompExpiryQueue(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: one sweep expires comps at the sweeper clock", func(t *testing.T) {
		runner := &fakeCompExpiryRunner{count: 1}
		obs := runCompExpiry(t, runner, now)

		assert.Equal(t, 1, runner.calls)
		assert.True(t, runner.at.Equal(now))
		assert.Equal(t, time.Duration(0), obs.oldest[billing.CompExpiryQueueName])
		assert.Zero(t, obs.failed)
	})

	t.Run("Edge: the queue is named and paced like the other billing queues", func(t *testing.T) {
		queue, err := billing.NewCompExpiryQueue(&fakeCompExpiryRunner{})
		require.NoError(t, err)

		assert.Equal(t, "billing_comp_expiry", queue.Name)
		assert.Equal(t, time.Hour, queue.Interval)
	})

	t.Run("Error: a store failure is counted and does not stop the sweep", func(t *testing.T) {
		runner := &fakeCompExpiryRunner{err: errors.New("mongo down")}
		obs := runCompExpiry(t, runner, now)

		assert.Equal(t, 1, runner.calls)
		assert.Equal(t, 1, obs.failed)
	})
}
