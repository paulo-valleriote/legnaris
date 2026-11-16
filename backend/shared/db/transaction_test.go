package db_test

import (
	"context"
	"errors"
	"fmt"
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
	type ctxKey struct{}
	hookErr := errors.New("injected crash")
	wantMarker := "marker"

	assertingHook := func(ctx context.Context, step db.Step) error {
		if step != db.StepAfterOutboxInsert {
			return fmt.Errorf("hook got step %q", step)
		}
		if ctx.Value(ctxKey{}) != wantMarker {
			return errors.New("hook did not receive the caller's context")
		}
		return nil
	}

	tests := []struct {
		name    string
		tx      *db.Transactor
		step    db.Step
		wantErr error
	}{
		{name: "Edge: a nil transactor is a no-op", tx: nil, step: db.StepAfterEntityWrite},
		{name: "Edge: a transactor without a hook is a no-op for entity writes", tx: db.NewTransactor(nil, nil), step: db.StepAfterEntityWrite},
		{name: "Edge: a transactor without a hook is a no-op for outbox inserts", tx: db.NewTransactor(nil, nil), step: db.StepAfterOutboxInsert},
		{name: "Edge: a transactor without a hook is a no-op for provider reads", tx: db.NewTransactor(nil, nil), step: db.StepAfterProviderRead},
		{name: "Happy path: the hook receives the named step and the caller's context", tx: db.NewTransactor(nil, assertingHook), step: db.StepAfterOutboxInsert},
		{
			name:    "Error: a hook error is returned wrapped and matchable",
			tx:      db.NewTransactor(nil, func(context.Context, db.Step) error { return hookErr }),
			step:    db.StepAfterProviderRead,
			wantErr: hookErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ctxKey{}, wantMarker)

			err := tc.tx.Step(ctx, tc.step)

			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestTransactor_ReplicaSet(t *testing.T) {
	type entity struct {
		ID string `bson:"_id"`
	}
	crash := errors.New("injected crash")
	errNotBound := errors.New("callback context is not session-bound")

	insertEntity := func(ctx context.Context, mdb *mongo.Database) error {
		_, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"})
		return err
	}
	insertOutbox := func(ctx context.Context, mdb *mongo.Database) error {
		_, err := mdb.Collection("outbox").InsertOne(ctx, entity{ID: "o1"})
		return err
	}
	crashAfterOutbox := func(_ context.Context, step db.Step) error {
		if step == db.StepAfterOutboxInsert {
			return crash
		}
		return nil
	}
	noopHook := func(context.Context, db.Step) error { return nil }

	tests := []struct {
		name         string
		hook         db.StepHook
		run          func(ctx context.Context, mdb *mongo.Database, tx *db.Transactor) error
		wantErrs     []error
		wantEntities int64
		wantOutbox   int64
	}{
		{
			name: "Happy path: both writes commit together",
			run: func(ctx context.Context, mdb *mongo.Database, _ *db.Transactor) error {
				if err := insertEntity(ctx, mdb); err != nil {
					return err
				}
				return insertOutbox(ctx, mdb)
			},
			wantEntities: 1,
			wantOutbox:   1,
		},
		{
			name: "Edge: the callback receives a session-bound context",
			run: func(ctx context.Context, _ *mongo.Database, _ *db.Transactor) error {
				if mongo.SessionFromContext(ctx) == nil {
					return errNotBound
				}
				return nil
			},
		},
		{
			name: "Error: a callback error rolls back earlier writes and keeps its identity",
			run: func(ctx context.Context, mdb *mongo.Database, _ *db.Transactor) error {
				if err := insertEntity(ctx, mdb); err != nil {
					return err
				}
				return errCallbackFailed
			},
			wantErrs: []error{db.ErrTransactionAborted, errCallbackFailed},
		},
		{
			name: "Error: a crash injected after the outbox insert leaves neither write",
			hook: crashAfterOutbox,
			run: func(ctx context.Context, mdb *mongo.Database, tx *db.Transactor) error {
				if err := insertEntity(ctx, mdb); err != nil {
					return err
				}
				if err := insertOutbox(ctx, mdb); err != nil {
					return err
				}
				return tx.Step(ctx, db.StepAfterOutboxInsert)
			},
			wantErrs: []error{db.ErrTransactionAborted, crash},
		},
		{
			name: "Edge: a hook that returns nil lets the transaction commit",
			hook: noopHook,
			run: func(ctx context.Context, mdb *mongo.Database, tx *db.Transactor) error {
				if err := insertEntity(ctx, mdb); err != nil {
					return err
				}
				return tx.Step(ctx, db.StepAfterEntityWrite)
			},
			wantEntities: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			tx := db.NewTransactor(mdb.Client(), tc.hook)

			err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
				return tc.run(ctx, mdb, tx)
			})

			if len(tc.wantErrs) == 0 {
				require.NoError(t, err)
			}
			for _, want := range tc.wantErrs {
				assert.ErrorIs(t, err, want)
			}
			assert.Equal(t, tc.wantEntities, countDocs(t, mdb, "entities"))
			assert.Equal(t, tc.wantOutbox, countDocs(t, mdb, "outbox"))
		})
	}
}

func countDocs(t *testing.T, mdb *mongo.Database, collection string) int64 {
	t.Helper()
	n, err := mdb.Collection(collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

func labeledError(label string) error {
	return mongo.CommandError{Code: 112, Name: "WriteConflict", Message: "injected", Labels: []string{label}}
}

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
	const transient = "TransientTransactionError"
	const unknownCommit = "UnknownTransactionCommitResult"

	insertEntity := func(ctx context.Context, mdb *mongo.Database) error {
		_, err := mdb.Collection("entities").InsertOne(ctx, entity{ID: "e1"})
		return err
	}
	commitFailure := func(code int, label string) bson.D {
		return bson.D{
			{Key: "failCommands", Value: bson.A{"commitTransaction"}},
			{Key: "errorCode", Value: code},
			{Key: "errorLabels", Value: bson.A{label}},
		}
	}

	tests := []struct {
		name         string
		timeout      time.Duration
		failPoint    bson.D
		run          func(ctx context.Context, mdb *mongo.Database, attempt int) error
		wantErr      error
		wantAttempts int
		wantEntities int64
	}{
		{
			name: "Happy path: a callback error with TransientTransactionError re-runs the callback and commits the retry",
			run: func(ctx context.Context, mdb *mongo.Database, attempt int) error {
				if err := insertEntity(ctx, mdb); err != nil {
					return err
				}
				if attempt == 1 {
					return labeledError(transient)
				}
				return nil
			},
			wantAttempts: 2,
			wantEntities: 1,
		},
		{
			name: "Edge: a live context keeps the TransientTransactionError label through the retry guard",
			run: func(_ context.Context, _ *mongo.Database, attempt int) error {
				if attempt < 3 {
					return labeledError(transient)
				}
				return nil
			},
			wantAttempts: 3,
		},
		{
			name: "Error: a callback error with UnknownTransactionCommitResult is not re-run as a callback error",
			run: func(context.Context, *mongo.Database, int) error {
				return labeledError(unknownCommit)
			},
			wantErr:      db.ErrTransactionAborted,
			wantAttempts: 1,
		},
		{
			name:    "Edge: a labeled callback error after the deadline is not retried",
			timeout: 300 * time.Millisecond,
			run: func(ctx context.Context, _ *mongo.Database, _ int) error {
				<-ctx.Done()
				return labeledError(transient)
			},
			wantErr:      db.ErrTransactionAborted,
			wantAttempts: 1,
		},
		{
			name:    "Edge: a labeled error inside the deadline is retried until the callback succeeds",
			timeout: 5 * time.Second,
			run: func(ctx context.Context, _ *mongo.Database, attempt int) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if attempt == 1 {
					return labeledError(transient)
				}
				return nil
			},
			wantAttempts: 2,
		},
		{
			name:      "Edge: a commit that fails with TransientTransactionError re-runs the callback",
			failPoint: commitFailure(112, transient),
			run: func(ctx context.Context, mdb *mongo.Database, _ int) error {
				return insertEntity(ctx, mdb)
			},
			wantAttempts: 2,
			wantEntities: 1,
		},
		{
			name:      "Edge: a commit that fails with UnknownTransactionCommitResult is retried at commit without re-running the callback",
			failPoint: commitFailure(251, unknownCommit),
			run: func(ctx context.Context, mdb *mongo.Database, _ int) error {
				return insertEntity(ctx, mdb)
			},
			wantAttempts: 1,
			wantEntities: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			if tc.failPoint != nil {
				configureFailPoint(t, mdb, tc.failPoint)
			}
			tx := db.NewTransactor(mdb.Client(), nil)
			if tc.timeout > 0 {
				tx = tx.WithTimeout(tc.timeout)
			}

			attempts := 0
			err := tx.WithTransaction(context.Background(), func(ctx context.Context) error {
				attempts++
				return tc.run(ctx, mdb, attempts)
			})

			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			assert.Equal(t, tc.wantAttempts, attempts)
			assert.Equal(t, tc.wantEntities, countDocs(t, mdb, "entities"))
		})
	}
}
