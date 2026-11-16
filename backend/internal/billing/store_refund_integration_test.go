package billing_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func activateRow(t *testing.T, store *billing.MongoStore, sub billing.Subscription, status billing.Status) {
	t.Helper()
	sub.Status = status
	sub.ProviderSubID = "sub_" + sub.ID
	synced := fixedNow
	sub.ProviderSyncedAt = &synced
	_, err := store.ApplySubscription(context.Background(), sub, fixedNow.Add(time.Second))
	require.NoError(t, err)
}

func TestMongoStore_RefundRules(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a live active subscription moves to requested and spends the right in the same write", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))
		activateRow(t, store, row, billing.StatusActive)

		ok, err := store.RequestRefund(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		require.True(t, ok)

		got, found, err := store.FindLive(ctx, userID)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, billing.RefundRequested, got.RefundState)
		require.NotNil(t, got.RefundUsedAt)
	})

	t.Run("Edge: a repeated request on the same subscription does not match again", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))
		activateRow(t, store, row, billing.StatusActive)

		ok, err := store.RequestRefund(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		require.True(t, ok)
		ok, err = store.RequestRefund(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Edge: a pending subscription cannot be refunded", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))

		ok, err := store.RequestRefund(ctx, row.ID, fixedNow)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Error: a second subscription of the same user cannot use the refund right again", func(t *testing.T) {
		store := newStoreDB(t)
		first := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, first))
		activateRow(t, store, first, billing.StatusActive)
		ok, err := store.RequestRefund(ctx, first.ID, fixedNow)
		require.NoError(t, err)
		require.True(t, ok)
		activateRow(t, store, first, billing.StatusCanceled)

		second := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, second))
		activateRow(t, store, second, billing.StatusActive)

		ok, err = store.RequestRefund(ctx, second.ID, fixedNow)
		require.ErrorIs(t, err, billing.ErrRefundAlreadyUsed)
		assert.False(t, ok)
	})

	t.Run("Edge: a failed refund returns the right so the next subscription can request it", func(t *testing.T) {
		store := newStoreDB(t)
		first := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, first))
		activateRow(t, store, first, billing.StatusActive)
		_, err := store.RequestRefund(ctx, first.ID, fixedNow)
		require.NoError(t, err)
		require.NoError(t, store.FailRefund(ctx, first.ID, "permanent"))
		activateRow(t, store, first, billing.StatusCanceled)

		second := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, second))
		activateRow(t, store, second, billing.StatusActive)

		ok, err := store.RequestRefund(ctx, second.ID, fixedNow)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Concurrency: simultaneous requests on one subscription produce exactly one requested refund", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))
		activateRow(t, store, row, billing.StatusActive)

		const callers = 8
		var wg sync.WaitGroup
		results := make(chan bool, callers)
		errs := make(chan error, callers)
		for range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := store.RequestRefund(ctx, row.ID, fixedNow)
				results <- ok
				errs <- err
			}()
		}
		wg.Wait()
		close(results)
		close(errs)

		wins := 0
		for ok := range results {
			if ok {
				wins++
			}
		}
		for err := range errs {
			assert.NoError(t, err)
		}
		assert.Equal(t, 1, wins)
	})
}
