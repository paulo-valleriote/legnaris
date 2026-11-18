package billing_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	otherUserID  = "0190a1b2-0000-7000-8000-000000000002"
	thirdUserID  = "0190a1b2-0000-7000-8000-000000000003"
	fourthUserID = "0190a1b2-0000-7000-8000-000000000004"
)

func compRow(t *testing.T, user string, end *time.Time) billing.Subscription {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return billing.Subscription{
		ID:               id,
		UserID:           user,
		PlanID:           "paid",
		Provider:         billing.ProviderComp,
		Status:           billing.StatusActive,
		CurrentPeriodEnd: end,
		CreatedAt:        fixedNow,
		UpdatedAt:        fixedNow,
	}
}

func TestMongoStore_CompSubscriptions(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: canceling a live comp marks it canceled", func(t *testing.T) {
		store := newStoreDB(t)
		row := compRow(t, userID, nil)
		require.NoError(t, store.InsertSubscription(ctx, row))

		canceled, err := store.CancelComp(ctx, row.ID, fixedNow.Add(time.Hour))
		require.NoError(t, err)
		require.True(t, canceled)

		_, found, err := store.FindLive(ctx, userID)
		require.NoError(t, err)
		assert.False(t, found, "a canceled comp is no longer live")
	})

	t.Run("Edge: canceling an already canceled comp matches nothing", func(t *testing.T) {
		store := newStoreDB(t)
		row := compRow(t, userID, nil)
		require.NoError(t, store.InsertSubscription(ctx, row))
		canceled, err := store.CancelComp(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		require.True(t, canceled)

		canceled, err = store.CancelComp(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		assert.False(t, canceled)
	})

	t.Run("Edge: a stripe subscription is not reachable through the comp cancel", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))

		canceled, err := store.CancelComp(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		assert.False(t, canceled)
	})

	t.Run("Happy path: the sweeper cancels only expired comps across users", func(t *testing.T) {
		store := newStoreDB(t)
		expired := compRow(t, userID, ptrTime(fixedNow.Add(-time.Minute)))
		live := compRow(t, otherUserID, ptrTime(fixedNow.Add(time.Hour)))
		perpetual := compRow(t, thirdUserID, nil)
		stripe := newPendingRow(t, fourthUserID)
		stripe.CurrentPeriodEnd = ptrTime(fixedNow.Add(-time.Hour))
		activateRow(t, store, stripe, billing.StatusActive)

		for _, row := range []billing.Subscription{expired, live, perpetual, stripe} {
			require.NoError(t, store.InsertSubscription(ctx, row))
		}

		n, err := store.CancelExpiredComps(ctx, fixedNow)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		for _, tc := range []struct {
			user  string
			alive bool
		}{
			{user: userID, alive: false},
			{user: otherUserID, alive: true},
			{user: thirdUserID, alive: true},
			{user: fourthUserID, alive: true},
		} {
			_, found, err := store.FindLive(ctx, tc.user)
			require.NoError(t, err)
			assert.Equal(t, tc.alive, found, "user %s", tc.user)
		}

		again, err := store.CancelExpiredComps(ctx, fixedNow)
		require.NoError(t, err)
		assert.Zero(t, again, "the cancel is idempotent")
	})
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
