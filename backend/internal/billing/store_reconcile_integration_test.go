package billing_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMongoLedger_Reconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("Edge: live subscriptions without a provider id are not reconciled", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		userID := newID(t)
		seedAccount(t, mdb, userID)
		at := mustParse(t, integrationNow)
		pending := billing.Subscription{ID: newID(t), UserID: userID, PlanID: "paid", Provider: billing.ProviderStripe, Status: billing.StatusPending, CreatedAt: at, UpdatedAt: at}
		require.NoError(t, store.InsertSubscription(ctx, pending))

		subs, err := store.LiveSubscriptions(ctx)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})

	t.Run("Happy path: every live subscription with a provider id is listed, canceled ones are not", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		userA, userB := newID(t), newID(t)
		seedAccount(t, mdb, userA)
		seedAccount(t, mdb, userB)
		active := billing.Subscription{ID: newID(t), UserID: userA, PlanID: "paid", Provider: billing.ProviderStripe, ProviderSubID: "sub_live_a", Status: billing.StatusActive, CreatedAt: at, UpdatedAt: at}
		canceled := billing.Subscription{ID: newID(t), UserID: userB, PlanID: "paid", Provider: billing.ProviderStripe, ProviderSubID: "sub_done_b", Status: billing.StatusCanceled, CreatedAt: at, UpdatedAt: at}
		require.NoError(t, store.InsertSubscription(ctx, active))
		require.NoError(t, store.InsertSubscription(ctx, canceled))

		subs, err := store.LiveSubscriptions(ctx)
		require.NoError(t, err)
		require.Len(t, subs, 1)
		assert.Equal(t, active.ID, subs[0].ID)
	})

	t.Run("Edge: a second acquire on a held user lease is refused", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		userID := newID(t)
		at := mustParse(t, integrationNow)

		first, err := store.AcquireUser(ctx, userID, "reconciler-a", at, time.Minute)
		require.NoError(t, err)
		second, err := store.AcquireUser(ctx, userID, "reconciler-b", at, time.Minute)
		require.NoError(t, err)

		assert.True(t, first)
		assert.False(t, second)
	})

	t.Run("Happy path: failed events return to received and stuck received events are pulled forward", func(t *testing.T) {
		_, mdb := ledgerDB(t)
		store := billing.NewMongoStore(mdb)
		at := mustParse(t, integrationNow)
		userID := newID(t)
		failed := billing.Event{ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_req_failed", Kind: billing.KindSubscriptionChanged, UserID: userID, Status: billing.EventFailed, Attempts: 5, NextAttemptAt: at, ReceivedAt: at, ExpiresAt: at.Add(time.Hour)}
		stuck := billing.Event{ID: newID(t), Provider: billing.ProviderStripe, ProviderEventID: "evt_req_stuck", Kind: billing.KindSubscriptionChanged, UserID: userID, Status: billing.EventReceived, NextAttemptAt: at.Add(time.Hour), ReceivedAt: at.Add(-2 * time.Hour), ExpiresAt: at.Add(time.Hour)}
		require.NoError(t, store.InsertEvent(ctx, failed))
		require.NoError(t, store.InsertEvent(ctx, stuck))

		requeued, err := store.RequeueEvents(ctx, at, at.Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 2, requeued)

		again, err := store.RequeueEvents(ctx, at, at.Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, again)
	})
}
