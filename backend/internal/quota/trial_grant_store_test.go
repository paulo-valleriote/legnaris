package quota_test

import (
	"context"
	"sync"
	"testing"

	"legnaris/backend/internal/quota"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoStoreGrantTrial(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a new account gets a row with the allowance and no usage", func(t *testing.T) {
		store, mdb := newMongoStore(t)

		require.NoError(t, store.GrantTrial(ctx, storeUser, quota.TrialAllowance))

		var row quota.TrialRow
		require.NoError(t, mdb.Collection(quota.TrialCollection).FindOne(ctx, bson.D{{Key: "_id", Value: storeUser}}).Decode(&row))
		assert.Equal(t, quota.TrialRow{UserID: storeUser, Allowance: quota.TrialAllowance, Used: 0}, row)
	})

	t.Run("Edge: a repeated grant never resets the used count", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		require.NoError(t, store.GrantTrial(ctx, storeUser, quota.TrialAllowance))
		for i := 0; i < 2; i++ {
			ok, err := store.ReserveTrial(ctx, storeUser)
			require.NoError(t, err)
			require.True(t, ok)
		}

		require.NoError(t, store.GrantTrial(ctx, storeUser, quota.TrialAllowance))

		assert.Equal(t, 2, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: a repeated grant keeps the existing allowance", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 5, 1)

		require.NoError(t, store.GrantTrial(ctx, storeUser, quota.TrialAllowance))

		var row quota.TrialRow
		require.NoError(t, mdb.Collection(quota.TrialCollection).FindOne(ctx, bson.D{{Key: "_id", Value: storeUser}}).Decode(&row))
		assert.Equal(t, 5, row.Allowance)
		assert.Equal(t, 1, row.Used)
	})

	t.Run("Edge: concurrent grants for one account create exactly one row without error", func(t *testing.T) {
		store, mdb := newMongoStore(t)

		const racers = 16
		var wg sync.WaitGroup
		errs := make(chan error, racers)
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- store.GrantTrial(ctx, storeUser, quota.TrialAllowance)
			}()
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}
		n, err := mdb.Collection(quota.TrialCollection).CountDocuments(ctx, bson.D{{Key: "_id", Value: storeUser}})
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
	})
}
