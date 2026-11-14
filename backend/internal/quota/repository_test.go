package quota_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"legnaris/backend/internal/quota"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// errNoAllowance aborts a transaction whose reservation was refused, so the
// caller can tell a refused reservation from a committed one.
var errNoAllowance = errors.New("reservation refused")

// newMongoStore returns a store on an isolated replica-set database with the
// quota indexes in place. It skips when DATABASE_URI is unset.
func newMongoStore(t *testing.T) (*quota.MongoStore, *mongo.Database) {
	t.Helper()
	mdb := dbtest.Database(t)
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return quota.NewMongoStore(mdb), mdb
}

// seedTrial inserts the row LGN-229 writes when it grants an allowance.
func seedTrial(t *testing.T, mdb *mongo.Database, userID string, allowance, used int) {
	t.Helper()
	_, err := mdb.Collection(quota.TrialCollection).InsertOne(context.Background(),
		quota.TrialRow{UserID: userID, Allowance: allowance, Used: used})
	require.NoError(t, err)
}

// trialUsed reads the stored used counter directly, bypassing the store under test.
func trialUsed(t *testing.T, mdb *mongo.Database, userID string) int {
	t.Helper()
	var row quota.TrialRow
	require.NoError(t, mdb.Collection(quota.TrialCollection).FindOne(context.Background(), bson.D{{Key: "_id", Value: userID}}).Decode(&row))
	return row.Used
}

// countDailyRows counts the usage rows for a user, so a test can prove that a
// refused or refunded reservation created no stray row.
func countDailyRows(t *testing.T, mdb *mongo.Database, userID string) int64 {
	t.Helper()
	n, err := mdb.Collection(quota.UsageCollection).CountDocuments(context.Background(), bson.D{{Key: "user_id", Value: userID}})
	require.NoError(t, err)
	return n
}

// seedDaily fills one day's counter to count by reserving it, so the limit is
// reached through the same path production uses.
func seedDaily(t *testing.T, store *quota.MongoStore, userID, day string, count int) {
	t.Helper()
	for range count {
		ok, err := store.ReserveDaily(context.Background(), userID, day, time.Now().Add(time.Hour))
		require.NoError(t, err)
		require.True(t, ok)
	}
}

const (
	storeUser = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"
	otherUser = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e10"
	dayOne    = "2026-11-14"
	dayTwo    = "2026-11-15"
)

func TestMongoStoreReserveTrial(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: an unused allowance is reserved and counted", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 1, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: an account without a trial row is refused and gets no row", func(t *testing.T) {
		store, mdb := newMongoStore(t)

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.False(t, ok)

		n, err := mdb.Collection(quota.TrialCollection).CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		assert.Zero(t, n, "reserving must never create the allowance")
	})

	t.Run("Edge: a zero allowance is refused even with a row", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 0, 0)

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, 0, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: a spent allowance is refused and not counted past its limit", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 3)

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, 3, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: concurrent reservations at the limit grant exactly the allowance", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)

		var granted atomic.Int32
		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := store.ReserveTrial(ctx, storeUser)
				assert.NoError(t, err)
				if ok {
					granted.Add(1)
				}
			}()
		}
		wg.Wait()

		assert.EqualValues(t, 3, granted.Load())
		assert.Equal(t, 3, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: another account's allowance is not touched", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, otherUser, 3, 0)

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, 0, trialUsed(t, mdb, otherUser))
	})
}

func TestMongoStoreRefundTrial(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a refund returns one generation and frees a slot", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 3)

		require.NoError(t, store.RefundTrial(ctx, storeUser))
		assert.Equal(t, 2, trialUsed(t, mdb, storeUser))

		ok, err := store.ReserveTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.True(t, ok, "the refunded slot must be reservable again")
	})

	t.Run("Edge: a refund on an unused allowance never goes negative", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)

		require.NoError(t, store.RefundTrial(ctx, storeUser))
		assert.Equal(t, 0, trialUsed(t, mdb, storeUser))
	})

	t.Run("Edge: a refund for an account without a row is a no-op and creates nothing", func(t *testing.T) {
		store, mdb := newMongoStore(t)

		require.NoError(t, store.RefundTrial(ctx, storeUser))

		n, err := mdb.Collection(quota.TrialCollection).CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		assert.Zero(t, n)
	})
}

func TestMongoStoreReserveDaily(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: the first reservation of a day opens its counter", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		expires := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)

		ok, err := store.ReserveDaily(ctx, storeUser, dayOne, expires)
		require.NoError(t, err)
		assert.True(t, ok)

		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, 1, used)

		var row struct {
			ExpiresAt time.Time `bson:"expires_at"`
		}
		require.NoError(t, mdb.Collection(quota.UsageCollection).FindOne(ctx, bson.D{
			{Key: "user_id", Value: storeUser}, {Key: "day_utc", Value: dayOne},
		}).Decode(&row))
		assert.True(t, row.ExpiresAt.Equal(expires), "the TTL instant must be stored")
	})

	t.Run("Edge: the 31st reservation of a paid day is refused and not counted", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, storeUser, dayOne, quota.PaidDailyLimit)

		ok, err := store.ReserveDaily(ctx, storeUser, dayOne, time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.False(t, ok)

		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, quota.PaidDailyLimit, used)
	})

	t.Run("Edge: concurrent reservations on a fresh day grant exactly the daily limit", func(t *testing.T) {
		store, _ := newMongoStore(t)

		var granted atomic.Int32
		var wg sync.WaitGroup
		for range 40 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := store.ReserveDaily(ctx, storeUser, dayOne, time.Now().Add(time.Hour))
				assert.NoError(t, err)
				if ok {
					granted.Add(1)
				}
			}()
		}
		wg.Wait()

		assert.EqualValues(t, quota.PaidDailyLimit, granted.Load())
		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, quota.PaidDailyLimit, used)
	})

	t.Run("Edge: a new UTC day starts from zero while the closed day stays full", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, storeUser, dayOne, quota.PaidDailyLimit)

		ok, err := store.ReserveDaily(ctx, storeUser, dayTwo, time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.True(t, ok)

		used, err := store.FetchDaily(ctx, storeUser, dayTwo)
		require.NoError(t, err)
		assert.Equal(t, 1, used)

		closed, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, quota.PaidDailyLimit, closed)
	})

	t.Run("Edge: another account's day is not touched", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, otherUser, dayOne, quota.PaidDailyLimit)

		ok, err := store.ReserveDaily(ctx, storeUser, dayOne, time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func TestMongoStoreRefundDaily(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a refund on the reserved day frees a slot", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, storeUser, dayOne, quota.PaidDailyLimit)

		require.NoError(t, store.RefundDaily(ctx, storeUser, dayOne))

		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, quota.PaidDailyLimit-1, used)
	})

	t.Run("Edge: a refund on an empty day never goes negative", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, storeUser, dayOne, 1)
		require.NoError(t, store.RefundDaily(ctx, storeUser, dayOne))

		require.NoError(t, store.RefundDaily(ctx, storeUser, dayOne))
		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Zero(t, used)
	})

	t.Run("Edge: a refund for a day with no counter creates no row", func(t *testing.T) {
		store, mdb := newMongoStore(t)

		require.NoError(t, store.RefundDaily(ctx, storeUser, dayOne))
		assert.Zero(t, countDailyRows(t, mdb, storeUser))
	})

	t.Run("Edge: a refund names its own day and leaves the next one alone", func(t *testing.T) {
		store, _ := newMongoStore(t)
		seedDaily(t, store, storeUser, dayOne, 2)
		seedDaily(t, store, storeUser, dayTwo, 2)

		require.NoError(t, store.RefundDaily(ctx, storeUser, dayOne))

		first, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		second, err := store.FetchDaily(ctx, storeUser, dayTwo)
		require.NoError(t, err)
		assert.Equal(t, 1, first)
		assert.Equal(t, 2, second)
	})
}

func TestMongoStoreFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("Edge: a missing trial row reads as the zero value", func(t *testing.T) {
		store, _ := newMongoStore(t)

		row, err := store.FetchTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.Equal(t, quota.TrialRow{}, row)
	})

	t.Run("Happy path: a trial row reads back with its allowance and used count", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, quota.TrialAllowance, 2)

		row, err := store.FetchTrial(ctx, storeUser)
		require.NoError(t, err)
		assert.Equal(t, quota.TrialRow{UserID: storeUser, Allowance: quota.TrialAllowance, Used: 2}, row)
	})

	t.Run("Edge: a day with no counter reads as zero", func(t *testing.T) {
		store, _ := newMongoStore(t)

		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Zero(t, used)
	})
}

// TestMongoStoreInTransaction proves the store joins the transaction helper's
// session: a rolled-back callback leaves no counter change, and concurrent
// transactions cannot exceed the allowance.
func TestMongoStoreInTransaction(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a committed transaction keeps both reservations", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)
		transactor := db.NewTransactor(mdb.Client(), nil)

		err := transactor.WithTransaction(ctx, func(tx context.Context) error {
			ok, err := store.ReserveTrial(tx, storeUser)
			if err != nil || !ok {
				return errors.Join(errNoAllowance, err)
			}
			ok, err = store.ReserveDaily(tx, storeUser, dayOne, time.Now().Add(time.Hour))
			if err != nil || !ok {
				return errors.Join(errNoAllowance, err)
			}
			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, 1, trialUsed(t, mdb, storeUser))
		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Equal(t, 1, used)
	})

	t.Run("Edge: a rolled-back transaction leaves both counters untouched", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)
		transactor := db.NewTransactor(mdb.Client(), nil)

		err := transactor.WithTransaction(ctx, func(tx context.Context) error {
			if _, err := store.ReserveTrial(tx, storeUser); err != nil {
				return err
			}
			if _, err := store.ReserveDaily(tx, storeUser, dayOne, time.Now().Add(time.Hour)); err != nil {
				return err
			}
			return errNoAllowance
		})
		require.ErrorIs(t, err, db.ErrTransactionAborted)
		require.ErrorIs(t, err, errNoAllowance)

		assert.Equal(t, 0, trialUsed(t, mdb, storeUser))
		used, err := store.FetchDaily(ctx, storeUser, dayOne)
		require.NoError(t, err)
		assert.Zero(t, used)
	})

	t.Run("Edge: concurrent transactions at the trial limit grant exactly the allowance", func(t *testing.T) {
		store, mdb := newMongoStore(t)
		seedTrial(t, mdb, storeUser, 3, 0)
		transactor := db.NewTransactor(mdb.Client(), nil)

		var granted atomic.Int32
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := transactor.WithTransaction(ctx, func(tx context.Context) error {
					ok, err := store.ReserveTrial(tx, storeUser)
					if err != nil {
						return err
					}
					if !ok {
						return errNoAllowance
					}
					return nil
				})
				if err == nil {
					granted.Add(1)
				}
			}()
		}
		wg.Wait()

		assert.EqualValues(t, 3, granted.Load())
		assert.Equal(t, 3, trialUsed(t, mdb, storeUser))
	})
}
