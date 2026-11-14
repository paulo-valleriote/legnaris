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

const (
	unreachableURITemplate = "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS="
	failFastSelectionMS    = "50"
	longSelectionMS        = "30000"
	outageUpperBound       = 20 * time.Second
)

// newUnreachableClient returns a client whose server cannot be selected, so
// operations fail without any external service. selectionMS is how long the
// driver waits for a server before giving up.
func newUnreachableClient(t *testing.T, selectionMS string) *mongo.Client {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI(unreachableURITemplate + selectionMS))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

func TestTransactor_WithTransaction(t *testing.T) {
	noop := func(context.Context) error { return nil }
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		tx      func(t *testing.T) *db.Transactor
		ctx     context.Context
		fn      func(context.Context) error
		wantErr error
	}{
		{
			name:    "Edge: a nil transactor reports a missing client",
			tx:      func(*testing.T) *db.Transactor { return nil },
			ctx:     context.Background(),
			fn:      noop,
			wantErr: db.ErrNoClient,
		},
		{
			name:    "Edge: a transactor without a client reports a missing client",
			tx:      func(*testing.T) *db.Transactor { return db.NewTransactor(nil, nil) },
			ctx:     context.Background(),
			fn:      noop,
			wantErr: db.ErrNoClient,
		},
		{
			name: "Edge: a nil callback is rejected before any server contact",
			tx: func(t *testing.T) *db.Transactor {
				return db.NewTransactor(newUnreachableClient(t, failFastSelectionMS), nil)
			},
			ctx:     context.Background(),
			fn:      nil,
			wantErr: db.ErrNilCallback,
		},
		{
			name: "Happy path: an empty callback commits without contacting the server",
			tx: func(t *testing.T) *db.Transactor {
				return db.NewTransactor(newUnreachableClient(t, failFastSelectionMS), nil)
			},
			ctx: context.Background(),
			fn:  noop,
		},
		{
			name: "Edge: a canceled context fails the transaction instead of waiting for server selection",
			tx: func(t *testing.T) *db.Transactor {
				return db.NewTransactor(newUnreachableClient(t, longSelectionMS), nil)
			},
			ctx:     canceled,
			fn:      noop,
			wantErr: db.ErrTransactionFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			err := tc.tx(t).WithTransaction(tc.ctx, tc.fn)

			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			assert.Less(t, time.Since(start), outageUpperBound)
		})
	}
}

func TestTransactor_Timeout(t *testing.T) {
	t.Run("Edge: a server outage returns within the transaction timeout, not the selection timeout", func(t *testing.T) {
		client := newUnreachableClient(t, longSelectionMS)
		tx := db.NewTransactor(client, nil).WithTimeout(200 * time.Millisecond)

		start := time.Now()
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			_, insertErr := client.Database("outage").Collection("entities").InsertOne(ctx, bson.D{{Key: "_id", Value: "e1"}})
			return insertErr
		})
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Less(t, elapsed, outageUpperBound)
	})

	t.Run("Edge: a nil transactor stays nil when given a timeout", func(t *testing.T) {
		var tx *db.Transactor
		assert.Nil(t, tx.WithTimeout(time.Second))
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

// labeledError returns a server error carrying one of the driver's retry labels,
// the shape the server gives a write that must be retried.
func labeledError(label string) error {
	return mongo.CommandError{Code: 112, Name: "WriteConflict", Message: "injected", Labels: []string{label}}
}

// configureFailPoint arms a failCommand failpoint on the replica set and disarms it
// at cleanup. It skips the test when the server refuses failpoints, which is the
// case for mongod started without enableTestCommands, such as the dev stack.
func configureFailPoint(t *testing.T, mdb *mongo.Database, data bson.D) {
	t.Helper()
	admin := mdb.Client().Database("admin")
	arm := bson.D{
		{Key: "configureFailPoint", Value: "failCommand"},
		{Key: "mode", Value: bson.D{{Key: "times", Value: 1}}},
		{Key: "data", Value: data},
	}
	if err := admin.RunCommand(context.Background(), arm).Err(); err != nil {
		t.Skipf("server refuses failCommand failpoints (needs enableTestCommands): %v", err)
	}
	t.Cleanup(func() {
		_ = admin.RunCommand(context.Background(), bson.D{
			{Key: "configureFailPoint", Value: "failCommand"},
			{Key: "mode", Value: "off"},
		}).Err()
	})
}

func TestTransactor_Retry(t *testing.T) {
	type entity struct {
		ID string `bson:"_id"`
	}

	countEntities := func(t *testing.T, mdb *mongo.Database) int64 {
		t.Helper()
		n, err := mdb.Collection("entities").CountDocuments(context.Background(), bson.D{})
		require.NoError(t, err)
		return n
	}

	t.Run("Happy path: a callback error with TransientTransactionError re-runs the callback and commits the retry", func(t *testing.T) {
		mdb := dbtest.Database(t)
		tx := db.NewTransactor(mdb.Client(), nil)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			attempts++
			if _, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"}); err != nil {
				return err
			}
			if attempts == 1 {
				return labeledError("TransientTransactionError")
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 2, attempts, "the labeled first attempt must be re-run once")
		assert.Equal(t, int64(1), countEntities(t, mdb), "the aborted first attempt must leave no write behind")
	})

	t.Run("Edge: a live context keeps the TransientTransactionError label through the retry guard", func(t *testing.T) {
		mdb := dbtest.Database(t)
		tx := db.NewTransactor(mdb.Client(), nil)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(context.Context) error {
			attempts++
			if attempts < 3 {
				return labeledError("TransientTransactionError")
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("Error: a callback error with UnknownTransactionCommitResult is not re-run as a callback error", func(t *testing.T) {
		mdb := dbtest.Database(t)
		tx := db.NewTransactor(mdb.Client(), nil)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(context.Context) error {
			attempts++
			return labeledError("UnknownTransactionCommitResult")
		})
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Equal(t, 1, attempts, "only TransientTransactionError re-runs the callback")
	})

	t.Run("Edge: a labeled callback error after the deadline is not retried", func(t *testing.T) {
		mdb := dbtest.Database(t)
		tx := db.NewTransactor(mdb.Client(), nil).WithTimeout(300 * time.Millisecond)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			attempts++
			<-ctx.Done()
			return labeledError("TransientTransactionError")
		})
		assert.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Equal(t, 1, attempts, "the unlabeled guard must stop the driver from re-running a dead callback")
	})

	t.Run("Edge: a labeled error inside the deadline is retried until the callback succeeds", func(t *testing.T) {
		mdb := dbtest.Database(t)
		tx := db.NewTransactor(mdb.Client(), nil).WithTimeout(5 * time.Second)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			attempts++
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempts == 1 {
				return labeledError("TransientTransactionError")
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("Edge: a commit that fails with TransientTransactionError re-runs the callback", func(t *testing.T) {
		mdb := dbtest.Database(t)
		configureFailPoint(t, mdb, bson.D{
			{Key: "failCommands", Value: bson.A{"commitTransaction"}},
			{Key: "errorCode", Value: 112},
			{Key: "errorLabels", Value: bson.A{"TransientTransactionError"}},
		})
		tx := db.NewTransactor(mdb.Client(), nil)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			attempts++
			_, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"})
			return err
		})
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
		assert.Equal(t, int64(1), countEntities(t, mdb))
	})

	t.Run("Edge: a commit that fails with UnknownTransactionCommitResult is retried at commit without re-running the callback", func(t *testing.T) {
		mdb := dbtest.Database(t)
		configureFailPoint(t, mdb, bson.D{
			{Key: "failCommands", Value: bson.A{"commitTransaction"}},
			{Key: "errorCode", Value: 251},
			{Key: "errorLabels", Value: bson.A{"UnknownTransactionCommitResult"}},
		})
		tx := db.NewTransactor(mdb.Client(), nil)

		attempts := 0
		err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
			attempts++
			_, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"})
			return err
		})
		require.NoError(t, err)
		assert.Equal(t, 1, attempts)
		assert.Equal(t, int64(1), countEntities(t, mdb))
	})
}
