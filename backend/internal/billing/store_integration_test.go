package billing_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func newStoreDB(t *testing.T) *billing.MongoStore {
	t.Helper()
	store, _ := newStoreWithDatabase(t)
	return store
}

func newStoreWithDatabase(t *testing.T) (*billing.MongoStore, *mongo.Database) {
	t.Helper()
	mdb := dbtest.Database(t)
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return billing.NewMongoStore(mdb), mdb
}

func newPendingRow(t *testing.T, user string) billing.Subscription {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return billing.Subscription{
		ID:        id,
		UserID:    user,
		PlanID:    "paid",
		Provider:  billing.ProviderStripe,
		Status:    billing.StatusPending,
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	}
}

func TestMongoStore_LiveSubscriptionRules(t *testing.T) {
	ctx := context.Background()

	t.Run("Edge: finding a user with no subscription reports none", func(t *testing.T) {
		store := newStoreDB(t)
		_, found, err := store.FindLive(ctx, userID)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("Error: a second live row for one user is rejected with ErrLiveExists", func(t *testing.T) {
		store := newStoreDB(t)
		require.NoError(t, store.InsertPending(ctx, newPendingRow(t, userID)))
		err := store.InsertPending(ctx, newPendingRow(t, userID))
		require.ErrorIs(t, err, billing.ErrLiveExists)
	})

	t.Run("Happy path: a canceled row frees the user for a new pending row", func(t *testing.T) {
		store := newStoreDB(t)
		first := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, first))
		require.NoError(t, store.Cancel(ctx, first.ID))
		require.NoError(t, store.InsertPending(ctx, newPendingRow(t, userID)))
	})

	t.Run("Edge: canceling an already canceled row is a no-op", func(t *testing.T) {
		store := newStoreDB(t)
		first := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, first))
		require.NoError(t, store.Cancel(ctx, first.ID))
		require.NoError(t, store.Cancel(ctx, first.ID))
	})

	t.Run("Edge: attaching a session to a canceled row matches nothing", func(t *testing.T) {
		store := newStoreDB(t)
		first := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, first))
		require.NoError(t, store.Cancel(ctx, first.ID))
		ok, err := store.AttachCheckout(ctx, first.ID, "cus_1", "https://x", fixedNow)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Happy path: attaching a session stores customer, URL and expiry on the pending row", func(t *testing.T) {
		store := newStoreDB(t)
		row := newPendingRow(t, userID)
		require.NoError(t, store.InsertPending(ctx, row))
		expires := fixedNow.Add(time.Hour)
		ok, err := store.AttachCheckout(ctx, row.ID, "cus_1", "https://checkout.example/x", expires)
		require.NoError(t, err)
		require.True(t, ok)

		got, found, err := store.FindLive(ctx, userID)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "cus_1", got.ProviderCustomerID)
		assert.Equal(t, "https://checkout.example/x", got.CheckoutURL)
		assert.True(t, expires.Equal(got.CheckoutExpiresAt))
	})
}

func TestCheckout_ConcurrentRequestsOnReplicaSet(t *testing.T) {
	store, mdb := newStoreWithDatabase(t)
	provider := billingtest.NewProvider()
	svc := newService(store, provider)

	t.Run("Edge: concurrent checkouts for one user produce one live subscription and one provider customer", func(t *testing.T) {
		const workers = 8
		var wg sync.WaitGroup
		urls := make([]string, workers)
		errs := make([]error, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				got, err := svc.Checkout(context.Background(), userID)
				urls[i], errs[i] = got.URL, err
			}(i)
		}
		close(start)
		wg.Wait()

		for i := 0; i < workers; i++ {
			require.NoError(t, errs[i])
			assert.Equal(t, urls[0], urls[i], "every request must receive the same session")
		}
		assert.Equal(t, 1, provider.CustomersCreated())
		assert.Equal(t, 1, provider.SessionsCreated())

		count, err := mdb.Collection("subscriptions").CountDocuments(context.Background(), bson.D{{Key: "user_id", Value: userID}})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}
