package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var errCallbackFailed = errors.New("callback failed")

// newUnreachableClient returns a client whose server cannot be selected, so
// operations fail fast without any external service.
func newUnreachableClient(t *testing.T) *mongo.Client {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=50"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

func TestTransactor_WithTransaction(t *testing.T) {
	t.Run("Edge: a nil transactor reports a missing client", func(t *testing.T) {
		var tx *db.Transactor
		err := tx.WithTransaction(context.Background(), func(context.Context) error { return nil })
		assert.ErrorIs(t, err, db.ErrNoClient)
	})

	t.Run("Edge: a transactor without a client reports a missing client", func(t *testing.T) {
		tx := db.NewTransactor(nil, nil)
		err := tx.WithTransaction(context.Background(), func(context.Context) error { return nil })
		assert.ErrorIs(t, err, db.ErrNoClient)
	})

	t.Run("Edge: a nil callback is rejected before any server contact", func(t *testing.T) {
		tx := db.NewTransactor(newUnreachableClient(t), nil)
		err := tx.WithTransaction(context.Background(), nil)
		assert.ErrorIs(t, err, db.ErrNilCallback)
	})

	t.Run("Edge: an empty callback commits without contacting the server", func(t *testing.T) {
		tx := db.NewTransactor(newUnreachableClient(t), nil)
		err := tx.WithTransaction(context.Background(), func(context.Context) error { return nil })
		assert.NoError(t, err)
	})

	t.Run("Edge: a canceled context fails the transaction without waiting", func(t *testing.T) {
		tx := db.NewTransactor(newUnreachableClient(t), nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		start := time.Now()
		err := tx.WithTransaction(ctx, func(context.Context) error { return nil })
		assert.ErrorIs(t, err, db.ErrTransactionFailed)
		assert.Less(t, time.Since(start), time.Second)
	})
}

func TestTransactor_Timeout(t *testing.T) {
	outageClient := func(t *testing.T) *mongo.Client {
		t.Helper()
		client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=30000"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
		return client
	}

	t.Run("Edge: a server outage returns within the transaction timeout", func(t *testing.T) {
		client := outageClient(t)
		tx := db.NewTransactor(client, nil).WithTimeout(200 * time.Millisecond)

		start := time.Now()
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			_, insertErr := client.Database("outage").Collection("entities").InsertOne(ctx, bson.D{{Key: "_id", Value: "e1"}})
			return insertErr
		})
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Less(t, elapsed, 5*time.Second)
	})

	t.Run("Edge: a nil transactor stays nil when given a timeout", func(t *testing.T) {
		var tx *db.Transactor
		assert.Nil(t, tx.WithTimeout(time.Second))
	})

	t.Run("Edge: a non-positive timeout keeps the default cap", func(t *testing.T) {
		tx := db.NewTransactor(newUnreachableClient(t), nil).WithTimeout(0)
		assert.NoError(t, tx.WithTransaction(context.Background(), func(context.Context) error { return nil }))
	})
}

func TestTransactor_Step(t *testing.T) {
	t.Run("Edge: a nil transactor is a no-op", func(t *testing.T) {
		var tx *db.Transactor
		assert.NoError(t, tx.Step(context.Background(), db.StepAfterEntityWrite))
	})

	t.Run("Edge: a transactor without a hook is a no-op in production", func(t *testing.T) {
		tx := db.NewTransactor(nil, nil)
		for _, step := range []db.Step{db.StepAfterEntityWrite, db.StepAfterOutboxInsert, db.StepAfterProviderRead} {
			assert.NoError(t, tx.Step(context.Background(), step), string(step))
		}
	})

	t.Run("Happy path: the hook receives the named step and the caller's context", func(t *testing.T) {
		type ctxKey struct{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

		var gotStep db.Step
		var gotMarker any
		tx := db.NewTransactor(nil, func(ctx context.Context, step db.Step) error {
			gotStep = step
			gotMarker = ctx.Value(ctxKey{})
			return nil
		})

		require.NoError(t, tx.Step(ctx, db.StepAfterOutboxInsert))
		assert.Equal(t, db.StepAfterOutboxInsert, gotStep)
		assert.Equal(t, "marker", gotMarker)
	})

	t.Run("Error: a hook error is returned wrapped and matchable", func(t *testing.T) {
		hookErr := errors.New("injected crash")
		tx := db.NewTransactor(nil, func(context.Context, db.Step) error { return hookErr })

		err := tx.Step(context.Background(), db.StepAfterProviderRead)
		assert.ErrorIs(t, err, hookErr)
	})
}

func TestTransactor_ReplicaSet(t *testing.T) {
	type entity struct {
		ID string `bson:"_id"`
	}

	countDocs := func(t *testing.T, mdb *mongo.Database, collection string) int64 {
		t.Helper()
		n, err := mdb.Collection(collection).CountDocuments(context.Background(), bson.D{})
		require.NoError(t, err)
		return n
	}

	t.Run("Happy path: both writes commit together", func(t *testing.T) {
		mdb := dbtest.Database(t)
		client := mdb.Client()
		tx := db.NewTransactor(client, nil)

		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			if _, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"}); err != nil {
				return err
			}
			_, err := mdb.Collection("outbox").InsertOne(ctx, entity{ID: "o1"})
			return err
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), countDocs(t, mdb, "entities"))
		assert.Equal(t, int64(1), countDocs(t, mdb, "outbox"))
	})

	t.Run("Edge: the callback receives a session-bound context", func(t *testing.T) {
		mdb := dbtest.Database(t)
		client := mdb.Client()
		tx := db.NewTransactor(client, nil)

		var bound bool
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			bound = mongo.SessionFromContext(ctx) != nil
			return nil
		})
		require.NoError(t, err)
		assert.True(t, bound)
	})

	t.Run("Error: a callback error rolls back earlier writes and keeps its identity", func(t *testing.T) {
		mdb := dbtest.Database(t)
		client := mdb.Client()
		tx := db.NewTransactor(client, nil)

		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			if _, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"}); err != nil {
				return err
			}
			return errCallbackFailed
		})
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.ErrorIs(t, err, errCallbackFailed)
		assert.Equal(t, int64(0), countDocs(t, mdb, "entities"))
	})

	t.Run("Error: a crash injected after the outbox insert leaves neither write", func(t *testing.T) {
		mdb := dbtest.Database(t)
		client := mdb.Client()
		crash := errors.New("injected crash")
		tx := db.NewTransactor(client, func(_ context.Context, step db.Step) error {
			if step == db.StepAfterOutboxInsert {
				return crash
			}
			return nil
		})

		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			if _, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"}); err != nil {
				return err
			}
			if _, err := mdb.Collection("outbox").InsertOne(ctx, entity{ID: "o1"}); err != nil {
				return err
			}
			return tx.Step(ctx, db.StepAfterOutboxInsert)
		})
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.ErrorIs(t, err, crash)
		assert.Equal(t, int64(0), countDocs(t, mdb, "entities"))
		assert.Equal(t, int64(0), countDocs(t, mdb, "outbox"))
	})

	t.Run("Edge: a hook that returns nil lets the transaction commit", func(t *testing.T) {
		mdb := dbtest.Database(t)
		client := mdb.Client()
		tx := db.NewTransactor(client, func(context.Context, db.Step) error { return nil })

		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			if _, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"}); err != nil {
				return err
			}
			return tx.Step(ctx, db.StepAfterEntityWrite)
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), countDocs(t, mdb, "entities"))
	})
}
