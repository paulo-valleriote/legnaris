package billing_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

const (
	consumerUserID     = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e10"
	consumerLocalSub   = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e11"
	consumerProviderID = "sub_consumer_1"
	consumerCustomer   = "cus_consumer_1"
)

var (
	t1 = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	t2 = t1.Add(time.Hour)
)

type consumerFixture struct {
	ledger   *billingtest.Ledger
	provider *billingtest.Provider
	consumer *billing.Consumer
	clock    time.Time
	hook     db.StepHook
	eventSeq int
}

func newConsumerFixture(t *testing.T) *consumerFixture {
	t.Helper()
	f := &consumerFixture{
		ledger:   billingtest.NewLedger(),
		provider: billingtest.NewProvider(),
		clock:    t1,
	}
	f.ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "free"})
	f.ledger.SeedSubscription(billing.Subscription{
		ID:                 consumerLocalSub,
		UserID:             consumerUserID,
		PlanID:             "paid",
		Provider:           billing.ProviderStripe,
		ProviderCustomerID: consumerCustomer,
		Status:             billing.StatusPending,
	})
	return f
}

func (f *consumerFixture) build() {
	tx := billingtest.NewTransactor(f.ledger, f.hook)
	f.consumer = billing.NewConsumer(billing.ConsumerDeps{
		Provider: f.provider,
		Ledger:   f.ledger,
		Tx:       tx,
		Outbox:   f.ledger.Outbox(),
		Now:      func() time.Time { return f.clock },
	})
}

func (f *consumerFixture) providerSays(status billing.Status, cancelAtEnd bool) {
	f.provider.SetSubscription(billing.ProviderSubscription{
		ID:                consumerProviderID,
		CustomerID:        consumerCustomer,
		Status:            status,
		CurrentPeriodEnd:  t1.AddDate(0, 1, 0),
		CancelAtPeriodEnd: cancelAtEnd,
	})
}

func (f *consumerFixture) queueEvent(t *testing.T, userID string) sweeper.Job[billing.Event] {
	t.Helper()
	f.eventSeq++
	id, err := idgen.New()
	require.NoError(t, err)
	ev := billing.Event{
		ID:                     id,
		Provider:               billing.ProviderStripe,
		ProviderEventID:        fmt.Sprintf("evt_consumer_%d", f.eventSeq),
		Kind:                   billing.KindSubscriptionChanged,
		UserID:                 userID,
		ProviderSubscriptionID: consumerProviderID,
		Status:                 billing.EventReceived,
		NextAttemptAt:          t1,
		ReceivedAt:             t1,
		ExpiresAt:              t1.Add(time.Hour),
	}
	require.NoError(t, f.ledger.InsertEvent(context.Background(), ev))
	return sweeper.Job[billing.Event]{ID: id, Payload: ev}
}

func (f *consumerFixture) run(job sweeper.Job[billing.Event]) error {
	return f.consumer.Handle(context.Background(), job, t1)
}

func (f *consumerFixture) planChanges(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, entry := range f.ledger.OutboxEntries() {
		ev, err := events.Decode(entry.Body)
		require.NoError(t, err)
		require.Equal(t, "plan.changed", ev.Type)
		var data map[string]any
		require.NoError(t, json.Unmarshal(ev.Data, &data))
		data["aggregate_version"] = ev.Aggregate.Version
		data["aggregate_id"] = ev.Aggregate.ID
		out = append(out, data)
	}
	return out
}

func (f *consumerFixture) planOf(t *testing.T, userID string) (string, int64) {
	t.Helper()
	acc, _ := f.ledger.Account(userID)
	return acc.Plan, acc.PlanVersion
}

func (f *consumerFixture) localSub(t *testing.T) billing.Subscription {
	t.Helper()
	sub, ok := findSub(f.ledger.Subscriptions(consumerUserID), consumerLocalSub)
	require.True(t, ok)
	return sub
}

func findSub(subs []billing.Subscription, id string) (billing.Subscription, bool) {
	for _, s := range subs {
		if s.ID == id {
			return s, true
		}
	}
	return billing.Subscription{}, false
}

func failAt(target db.Step) db.StepHook {
	return func(_ context.Context, step db.Step) error {
		if step == target {
			return errors.New("injected crash")
		}
		return nil
	}
}

func TestConsumer_Handle(t *testing.T) {
	t.Run("Happy path: an active provider read activates the subscription, derives paid, and emits one plan.changed", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		require.NoError(t, f.run(job))

		sub := f.localSub(t)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Equal(t, consumerProviderID, sub.ProviderSubID)
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(t1))

		plan, version := f.planOf(t, consumerUserID)
		assert.Equal(t, "paid", plan)
		assert.Equal(t, int64(1), version)

		changes := f.planChanges(t)
		require.Len(t, changes, 1)
		assert.Equal(t, "free", changes[0]["from"])
		assert.Equal(t, "paid", changes[0]["to"])
		assert.EqualValues(t, 1, changes[0]["aggregate_version"])
		assert.Equal(t, consumerUserID, changes[0]["aggregate_id"])

		ev, ok := f.ledger.Event(job.ID)
		require.True(t, ok)
		assert.Equal(t, billing.EventProcessed, ev.Status)
		assert.Empty(t, ev.Reason)
	})

	t.Run("Happy path: a cancellation read takes the plan back to free with a second plan.changed", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.build()
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		f.providerSays(billing.StatusCanceled, false)
		f.clock = t2
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		plan, version := f.planOf(t, consumerUserID)
		assert.Equal(t, "free", plan)
		assert.Equal(t, int64(2), version)
		changes := f.planChanges(t)
		require.Len(t, changes, 2)
		assert.Equal(t, "paid", changes[1]["from"])
		assert.Equal(t, "free", changes[1]["to"])
		assert.EqualValues(t, 2, changes[1]["aggregate_version"])
	})

	t.Run("Edge: a read that does not change the plan emits no plan.changed and keeps the version", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusPastDue, false)
		f.build()
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		plan, version := f.planOf(t, consumerUserID)
		assert.Equal(t, "free", plan)
		assert.Equal(t, int64(0), version)
		assert.Empty(t, f.planChanges(t))
	})

	t.Run("Edge: replaying an already processed event does not emit plan.changed again", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.build()
		job := f.queueEvent(t, consumerUserID)
		require.NoError(t, f.run(job))
		require.NoError(t, f.run(job))

		assert.Len(t, f.planChanges(t), 1)
		_, version := f.planOf(t, consumerUserID)
		assert.Equal(t, int64(1), version)
	})

	t.Run("Edge: a user in deleting is marked processed with reason ignored and the provider is not written", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "free", Deleting: true})
		f.providerSays(billing.StatusActive, false)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		require.NoError(t, f.run(job))

		ev, _ := f.ledger.Event(job.ID)
		assert.Equal(t, billing.EventProcessed, ev.Status)
		assert.Equal(t, billing.ReasonIgnored, ev.Reason)
		assert.Equal(t, billing.StatusPending, f.localSub(t).Status)
		assert.Empty(t, f.planChanges(t))
		assert.Equal(t, 0, f.provider.Reads())
	})

	t.Run("Edge: a tombstoned user (no account) is marked processed with reason ignored", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.ledger.RemoveAccount(consumerUserID)
		f.providerSays(billing.StatusActive, false)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		require.NoError(t, f.run(job))

		ev, _ := f.ledger.Event(job.ID)
		assert.Equal(t, billing.EventProcessed, ev.Status)
		assert.Equal(t, billing.ReasonIgnored, ev.Reason)
		assert.Empty(t, f.planChanges(t))
	})

	t.Run("Error: a provider outage returns a retryable error and writes nothing", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.provider.Unavailable = true
		f.build()
		job := f.queueEvent(t, consumerUserID)

		err := f.run(job)
		require.Error(t, err)
		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
		require.NotErrorIs(t, err, sweeper.ErrPermanent)

		ev, _ := f.ledger.Event(job.ID)
		assert.Equal(t, billing.EventReceived, ev.Status)
		assert.Equal(t, billing.StatusPending, f.localSub(t).Status)
		assert.Empty(t, f.planChanges(t))
	})

	t.Run("Error: a subscription the provider does not know is permanent", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		err := f.run(job)
		require.Error(t, err)
		assert.ErrorIs(t, err, sweeper.ErrPermanent)
	})

	t.Run("Error: a crash after the provider read rolls back and the retry emits exactly one plan.changed", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.hook = failAt(db.StepAfterProviderRead)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		require.Error(t, f.run(job))
		assert.Empty(t, f.planChanges(t))
		assert.Equal(t, billing.StatusPending, f.localSub(t).Status)

		f.hook = nil
		f.build()
		require.NoError(t, f.run(job))
		assert.Len(t, f.planChanges(t), 1)
	})

	t.Run("Error: a crash after the outbox insert rolls back the outbox row, the plan and the subscription", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.providerSays(billing.StatusActive, false)
		f.hook = failAt(db.StepAfterOutboxInsert)
		f.build()
		job := f.queueEvent(t, consumerUserID)

		require.Error(t, f.run(job))
		assert.Empty(t, f.planChanges(t))
		plan, version := f.planOf(t, consumerUserID)
		assert.Equal(t, "free", plan)
		assert.Equal(t, int64(0), version)
		assert.Equal(t, billing.StatusPending, f.localSub(t).Status)
		ev, _ := f.ledger.Event(job.ID)
		assert.Equal(t, billing.EventReceived, ev.Status)

		f.hook = nil
		f.build()
		require.NoError(t, f.run(job))
		assert.Len(t, f.planChanges(t), 1)
	})

	t.Run("Edge: a stale read applied after a fresh one never regresses the status", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.build()

		f.clock = t2
		f.providerSays(billing.StatusActive, false)
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		f.clock = t1
		f.providerSays(billing.StatusPastDue, false)
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.True(t, sub.ProviderSyncedAt.Equal(t2))
		plan, _ := f.planOf(t, consumerUserID)
		assert.Equal(t, "paid", plan)
		assert.Len(t, f.planChanges(t), 1)
	})

	t.Run("Edge: a fresh read applied after a stale one ends on the newest state", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.build()

		f.clock = t1
		f.providerSays(billing.StatusPastDue, false)
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		f.clock = t2
		f.providerSays(billing.StatusActive, false)
		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		sub := f.localSub(t)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.True(t, sub.ProviderSyncedAt.Equal(t2))
		plan, _ := f.planOf(t, consumerUserID)
		assert.Equal(t, "paid", plan)
		assert.Len(t, f.planChanges(t), 1)
	})

	t.Run("Edge: a second live subscription keeps the plan paid while the first is canceled", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "paid"})
		otherID := "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e12"
		f.ledger.SeedSubscription(billing.Subscription{
			ID: otherID, UserID: consumerUserID, Status: billing.StatusActive,
			ProviderSubID: "sub_other", ProviderCustomerID: consumerCustomer,
			ProviderSyncedAt: &t1,
		})
		f.providerSays(billing.StatusCanceled, false)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		plan, _ := f.planOf(t, consumerUserID)
		assert.Equal(t, "paid", plan)
		assert.Empty(t, f.planChanges(t))
	})

	t.Run("Edge: a live comp keeps the plan paid when the stripe subscription cancels", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "paid"})
		end := t1.Add(24 * time.Hour)
		f.ledger.SeedSubscription(billing.Subscription{
			ID:               "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e13",
			UserID:           consumerUserID,
			PlanID:           "paid",
			Provider:         billing.ProviderComp,
			Status:           billing.StatusActive,
			CurrentPeriodEnd: &end,
		})
		f.ledger.SeedSubscription(billing.Subscription{
			ID:                 "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e15",
			UserID:             consumerUserID,
			Provider:           billing.ProviderStripe,
			ProviderSubID:      consumerProviderID,
			ProviderCustomerID: consumerCustomer,
			Status:             billing.StatusPending,
		})
		f.providerSays(billing.StatusCanceled, false)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		plan, _ := f.planOf(t, consumerUserID)
		assert.Equal(t, "paid", plan)
		assert.Empty(t, f.planChanges(t))
	})

	t.Run("Edge: an expired comp is not entitled when the stripe subscription cancels", func(t *testing.T) {
		f := newConsumerFixture(t)
		f.ledger.AddAccount(consumerUserID, billing.AccountPlan{Plan: "paid"})
		end := t1.Add(-time.Hour)
		f.ledger.SeedSubscription(billing.Subscription{
			ID:               "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e14",
			UserID:           consumerUserID,
			PlanID:           "paid",
			Provider:         billing.ProviderComp,
			Status:           billing.StatusActive,
			CurrentPeriodEnd: &end,
		})
		f.ledger.SeedSubscription(billing.Subscription{
			ID:                 "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e16",
			UserID:             consumerUserID,
			Provider:           billing.ProviderStripe,
			ProviderSubID:      consumerProviderID,
			ProviderCustomerID: consumerCustomer,
			Status:             billing.StatusPending,
		})
		f.providerSays(billing.StatusCanceled, false)
		f.build()

		require.NoError(t, f.run(f.queueEvent(t, consumerUserID)))

		plan, _ := f.planOf(t, consumerUserID)
		assert.Equal(t, "free", plan)
		changes := f.planChanges(t)
		require.Len(t, changes, 1)
		assert.Equal(t, "paid", changes[0]["from"])
		assert.Equal(t, "free", changes[0]["to"])
	})
}
