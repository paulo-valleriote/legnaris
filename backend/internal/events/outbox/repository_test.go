package outbox_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/events/relay"
	relaymocks "legnaris/backend/internal/events/relay/mocks"
	"legnaris/backend/internal/sweeper"
	sweepermocks "legnaris/backend/internal/sweeper/mocks"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const entityCollection = "entities_under_test"

func newOutboxFixture(t *testing.T) (*mongo.Database, outbox.Repository) {
	t.Helper()

	mdb := dbtest.Database(t)
	require.NoError(t, db.EnsureIndexesWithRetention(context.Background(), mdb, 24*time.Hour))
	return mdb, outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError))
}

func newEntryFor(t *testing.T, now time.Time) outbox.Entry {
	t.Helper()
	entry, err := outbox.NewEntry(events.Event{
		Type:          "user.deleted",
		AggregateType: "user",
		AggregateID:   newUUIDString(t),
	}, now)
	require.NoError(t, err)
	return entry
}

func countEntries(t *testing.T, mdb *mongo.Database) int64 {
	t.Helper()
	count, err := mdb.Collection("event_outbox").CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return count
}

func TestRepositoryInsert(t *testing.T) {
	mdb, repo := newOutboxFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: a valid entry is stored once", func(t *testing.T) {
		require.NoError(t, repo.Insert(ctx, newEntryFor(t, now)))
		assert.Equal(t, int64(1), countEntries(t, mdb))
	})

	tests := []struct {
		name  string
		entry func(t *testing.T) outbox.Entry
	}{
		{
			name:  "Error: a non-canonical identity is rejected before the write",
			entry: func(*testing.T) outbox.Entry { return outbox.Entry{ID: "bad", Body: []byte(`{}`)} },
		},
		{
			name: "Edge: an empty identity is rejected",
			entry: func(*testing.T) outbox.Entry {
				return outbox.Entry{Body: []byte(`{}`)}
			},
		},
		{
			name: "Edge: an empty body is rejected",
			entry: func(t *testing.T) outbox.Entry {
				entry := newEntryFor(t, now)
				entry.Body = nil
				return entry
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := countEntries(t, mdb)
			err := repo.Insert(ctx, tc.entry(t))
			require.ErrorIs(t, err, outbox.ErrInvalidEntry)
			assert.Equal(t, before, countEntries(t, mdb), "a rejected entry must not be written")
		})
	}

	t.Run("Error: a duplicate identity is rejected by the unique _id", func(t *testing.T) {
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))
		assert.Error(t, repo.Insert(ctx, entry))
	})
}

func TestRepositoryClaim(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	lease := 30 * time.Second

	t.Run("Happy path: an eligible entry is claimed and leased to the claimer", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))

		claimed, err := repo.Claim(ctx, "relay-a", now, lease, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		assert.Equal(t, entry.ID, claimed[0].ID)
		assert.Equal(t, "relay-a", claimed[0].LockedBy)
		require.NotNil(t, claimed[0].LockedUntil)
		assert.True(t, claimed[0].LockedUntil.Equal(now.Add(lease)))
		assert.Equal(t, outbox.StatusPending, claimed[0].Status, "a claim does not change status")
	})

	t.Run("Edge: a leased entry is not claimed by a second relay", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		require.NoError(t, repo.Insert(ctx, newEntryFor(t, now)))

		first, err := repo.Claim(ctx, "relay-a", now, lease, 10)
		require.NoError(t, err)
		second, err := repo.Claim(ctx, "relay-b", now, lease, 10)
		require.NoError(t, err)

		assert.Len(t, first, 1)
		assert.Empty(t, second, "two relays must never share an entry")
	})

	t.Run("Edge: an expired lease is claimed again", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		require.NoError(t, repo.Insert(ctx, newEntryFor(t, now)))

		_, err := repo.Claim(ctx, "relay-a", now, lease, 10)
		require.NoError(t, err)

		reclaimed, err := repo.Claim(ctx, "relay-b", now.Add(lease), lease, 10)
		require.NoError(t, err)
		require.Len(t, reclaimed, 1)
		assert.Equal(t, "relay-b", reclaimed[0].LockedBy)
	})

	t.Run("Edge: an entry whose eligibility is in the future is not claimed", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		require.NoError(t, repo.Insert(ctx, newEntryFor(t, now.Add(time.Hour))))

		claimed, err := repo.Claim(ctx, "relay-a", now, lease, 10)
		require.NoError(t, err)
		assert.Empty(t, claimed)
	})

	t.Run("Edge: a limit of zero claims nothing", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		require.NoError(t, repo.Insert(ctx, newEntryFor(t, now)))

		claimed, err := repo.Claim(ctx, "relay-a", now, lease, 0)
		require.NoError(t, err)
		assert.Nil(t, claimed)
	})

	t.Run("Edge: the batch limit caps the claim and leaves the rest pending", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		for range 3 {
			require.NoError(t, repo.Insert(ctx, newEntryFor(t, now)))
		}

		claimed, err := repo.Claim(ctx, "relay-a", now, lease, 2)
		require.NoError(t, err)
		assert.Len(t, claimed, 2)

		rest, err := repo.Claim(ctx, "relay-b", now, lease, 10)
		require.NoError(t, err)
		assert.Len(t, rest, 1)
	})
}

func TestRepositoryTransitions(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: MarkPublished sets the terminal state and clears the lease", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))
		_, err := repo.Claim(ctx, "relay-a", now, time.Minute, 1)
		require.NoError(t, err)

		require.NoError(t, repo.MarkPublished(ctx, entry.ID, now))

		stored := readEntry(t, mdb, entry.ID)
		assert.Equal(t, outbox.StatusPublished, stored.Status)
		require.NotNil(t, stored.PublishedAt)
		assert.Nil(t, stored.LockedUntil)
		assert.Empty(t, stored.LockedBy)
	})

	t.Run("Edge: MarkPublished repeated on a published entry leaves the same state", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))

		require.NoError(t, repo.MarkPublished(ctx, entry.ID, now))
		require.NoError(t, repo.MarkPublished(ctx, entry.ID, now))

		assert.Equal(t, outbox.StatusPublished, readEntry(t, mdb, entry.ID).Status)
	})

	t.Run("Happy path: Reschedule returns the entry to pending with one more attempt", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))
		_, err := repo.Claim(ctx, "relay-a", now, time.Minute, 1)
		require.NoError(t, err)

		next := now.Add(5 * time.Second)
		require.NoError(t, repo.Reschedule(ctx, entry.ID, now, next, "broker nacked"))

		stored := readEntry(t, mdb, entry.ID)
		assert.Equal(t, outbox.StatusPending, stored.Status)
		assert.Equal(t, 1, stored.Attempts)
		assert.True(t, stored.NextAttemptAt.Equal(next))
		assert.Equal(t, "broker nacked", stored.LastError)
		assert.Nil(t, stored.LockedUntil)
	})

	t.Run("Happy path: MarkTerminal fails the entry so it is never claimed again", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))

		require.NoError(t, repo.MarkTerminal(ctx, entry.ID, now, "attempt cap"))

		assert.Equal(t, outbox.StatusDead, readEntry(t, mdb, entry.ID).Status)
		claimed, err := repo.Claim(ctx, "relay-a", now.Add(24*time.Hour), time.Minute, 10)
		require.NoError(t, err)
		assert.Empty(t, claimed)
	})

	t.Run("Error: transitions on a non-canonical identity are rejected", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()

		require.ErrorIs(t, repo.MarkPublished(ctx, "bad", now), outbox.ErrInvalidEntry)
		require.ErrorIs(t, repo.Reschedule(ctx, "bad", now, now, "x"), outbox.ErrInvalidEntry)
		assert.ErrorIs(t, repo.MarkTerminal(ctx, "bad", now, "x"), outbox.ErrInvalidEntry)
	})

	t.Run("Edge: a transition on an unknown identity matches no document and is not an error", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		unknown := newEntryFor(t, now).ID

		assert.NoError(t, repo.MarkPublished(ctx, unknown, now))
	})
}

func TestRepositoryOldestPending(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Edge: an empty outbox reports no pending entry", func(t *testing.T) {
		_, repo := newOutboxFixture(t)

		_, pending, err := repo.OldestPending(context.Background())
		require.NoError(t, err)
		assert.False(t, pending)
	})

	t.Run("Happy path: the oldest pending entry by creation time is reported", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		older := newEntryFor(t, now.Add(-time.Hour))
		newer := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, newer))
		require.NoError(t, repo.Insert(ctx, older))

		created, pending, err := repo.OldestPending(ctx)
		require.NoError(t, err)
		require.True(t, pending)
		assert.True(t, created.Equal(older.CreatedAt))

		require.NoError(t, repo.MarkPublished(ctx, older.ID, now))
		created, pending, err = repo.OldestPending(ctx)
		require.NoError(t, err)
		require.True(t, pending)
		assert.True(t, created.Equal(newer.CreatedAt), "a published entry leaves the probe")
		assert.Equal(t, outbox.StatusPublished, readEntry(t, mdb, older.ID).Status)
	})

	t.Run("Edge: a dead entry is not pending and leaves the probe", func(t *testing.T) {
		_, repo := newOutboxFixture(t)
		ctx := context.Background()
		entry := newEntryFor(t, now)
		require.NoError(t, repo.Insert(ctx, entry))
		require.NoError(t, repo.MarkTerminal(ctx, entry.ID, now, "attempt cap"))

		_, pending, err := repo.OldestPending(ctx)
		require.NoError(t, err)
		assert.False(t, pending)
	})
}

func runPass(t *testing.T, repo outbox.Repository, publisher relay.Publisher, observer sweeper.Observer, now time.Time) {
	t.Helper()
	queue, err := relay.NewQueue(repo, publisher, relay.Config{
		Interval:       time.Minute,
		Batch:          10,
		ConfirmTimeout: time.Second,
		AttemptCap:     5,
	})
	require.NoError(t, err)
	sw, err := sweeper.New([]sweeper.Queue{queue}, "relay-test", observer, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	require.NoError(t, sw.RunOnce(context.Background(), now))
}

func TestRepositoryTransactions(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	errCrash := errors.New("injected crash")

	writeEntityAndEntry := func(
		ctx context.Context,
		transactor *db.Transactor,
		repo outbox.Repository,
		mdb *mongo.Database,
		entryID string,
		entry outbox.Entry,
	) error {
		return transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			if _, err := mdb.Collection(entityCollection).InsertOne(txCtx, bson.D{{Key: "_id", Value: entryID}}); err != nil {
				return err
			}
			if err := transactor.Step(txCtx, db.StepAfterEntityWrite); err != nil {
				return err
			}
			if err := repo.Insert(txCtx, entry); err != nil {
				return err
			}
			return transactor.Step(txCtx, db.StepAfterOutboxInsert)
		})
	}

	failAt := func(target db.Step) db.StepHook {
		return func(_ context.Context, step db.Step) error {
			if step == target {
				return errCrash
			}
			return nil
		}
	}

	t.Run("Happy path: the entity write and the outbox row commit together", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		transactor := db.NewTransactor(mdb.Client(), nil)
		entryID := newUUIDString(t)
		entry := newEntryFor(t, now)

		require.NoError(t, writeEntityAndEntry(context.Background(), transactor, repo, mdb, entryID, entry))

		assert.Equal(t, int64(1), countDocs(t, mdb, entityCollection))
		assert.Equal(t, int64(1), countEntries(t, mdb))
	})

	t.Run("Error: a crash after the entity write emits nothing and the retry emits once", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		entryID := newUUIDString(t)
		entry := newEntryFor(t, now)

		crashing := db.NewTransactor(mdb.Client(), failAt(db.StepAfterEntityWrite))
		err := writeEntityAndEntry(context.Background(), crashing, repo, mdb, entryID, entry)
		require.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Equal(t, int64(0), countDocs(t, mdb, entityCollection), "the entity write rolls back")
		assert.Equal(t, int64(0), countEntries(t, mdb), "no outbox row without the entity write")

		retry := db.NewTransactor(mdb.Client(), nil)
		require.NoError(t, writeEntityAndEntry(context.Background(), retry, repo, mdb, entryID, entry))
		assert.Equal(t, int64(1), countEntries(t, mdb), "the retry emits exactly one entry")
	})

	t.Run("Error: a crash after the outbox insert rolls back both writes", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		entryID := newUUIDString(t)

		crashing := db.NewTransactor(mdb.Client(), failAt(db.StepAfterOutboxInsert))
		err := writeEntityAndEntry(context.Background(), crashing, repo, mdb, entryID, newEntryFor(t, now))
		require.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Equal(t, int64(0), countDocs(t, mdb, entityCollection))
		assert.Equal(t, int64(0), countEntries(t, mdb))
	})

	t.Run("Error: a callback error rolls back the outbox insert", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		transactor := db.NewTransactor(mdb.Client(), nil)

		err := transactor.WithTransaction(context.Background(), func(txCtx context.Context) error {
			if err := repo.Insert(txCtx, newEntryFor(t, now)); err != nil {
				return err
			}
			return errCrash
		})
		require.ErrorIs(t, err, db.ErrTransactionAborted)
		require.ErrorIs(t, err, errCrash)
		assert.Equal(t, int64(0), countEntries(t, mdb))
	})

	t.Run("Edge: an insert made with the outer context does not join the transaction", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		transactor := db.NewTransactor(mdb.Client(), nil)

		err := transactor.WithTransaction(context.Background(), func(txCtx context.Context) error {
			require.NoError(t, repo.Insert(context.Background(), newEntryFor(t, now)))
			return errCrash
		})
		require.ErrorIs(t, err, db.ErrTransactionAborted)
		assert.Equal(t, int64(1), countEntries(t, mdb), "a write outside the session context survives the rollback")
	})

	t.Run("Happy path: a committed row published after a failed first attempt is published once", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		ctx := context.Background()
		transactor := db.NewTransactor(mdb.Client(), nil)
		entry := newEntryFor(t, now)
		require.NoError(t, transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			return repo.Insert(txCtx, entry)
		}))

		publisher := relaymocks.NewMockPublisher(t)
		publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errCrash).Once()
		publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
		observer := sweepermocks.NewMockObserver(t)
		observer.EXPECT().OldestPending(relay.QueueName, mock.Anything).Maybe()
		observer.EXPECT().Failed(relay.QueueName).Once()

		runPass(t, repo, publisher, observer, now)
		assert.Equal(t, outbox.StatusPending, readEntry(t, mdb, entry.ID).Status, "a failed publish reschedules, it does not publish")

		runPass(t, repo, publisher, observer, now.Add(time.Hour))
		assert.Equal(t, outbox.StatusPublished, readEntry(t, mdb, entry.ID).Status)
	})
}

func newUUIDString(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func countDocs(t *testing.T, mdb *mongo.Database, collection string) int64 {
	t.Helper()
	count, err := mdb.Collection(collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return count
}

func readEntry(t *testing.T, mdb *mongo.Database, id string) outbox.Entry {
	t.Helper()
	var entry outbox.Entry
	require.NoError(t, mdb.Collection("event_outbox").FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&entry))
	return entry
}
