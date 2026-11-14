package accountdeletion_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"legnaris/backend/internal/accountdeletion"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	outboxmocks "legnaris/backend/internal/events/outbox/mocks"
	"legnaris/backend/internal/quota"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/logger"
)

const (
	victimID = "01900000-0000-7000-8000-000000000001"
	otherID  = "01900000-0000-7000-8000-000000000002"
)

var deleteAt = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// seedAccount writes one row in every collection the cascade owns for userID.
func seedAccount(t *testing.T, mdb *mongo.Database, userID string) {
	t.Helper()
	ctx := context.Background()
	seeds := map[string]bson.D{
		"sheets":              {{Key: "owner_id", Value: userID}, {Key: "name", Value: "Hero"}},
		"sheet_counts":        {{Key: "owner_id", Value: userID}, {Key: "kind", Value: "character"}, {Key: "count", Value: 1}},
		"generations":         {{Key: "user_id", Value: userID}, {Key: "status", Value: "running"}},
		quota.TrialCollection: {{Key: "_id", Value: userID}, {Key: "allowance", Value: 3}, {Key: "used", Value: 1}},
		quota.UsageCollection: {{Key: "user_id", Value: userID}, {Key: "day_utc", Value: "2026-11-14"}, {Key: "count", Value: 1}},
		"users":               {{Key: "_id", Value: userID}, {Key: "email", Value: "pii@example.com"}, {Key: "username", Value: "hero"}},
	}
	for collection, doc := range seeds {
		_, err := mdb.Collection(collection).InsertOne(ctx, doc)
		require.NoError(t, err, collection)
	}
}

// countOwned counts the rows of collection that belong to userID, by the given field.
func countOwned(t *testing.T, mdb *mongo.Database, collection, field, userID string) int64 {
	t.Helper()
	n, err := mdb.Collection(collection).CountDocuments(context.Background(), bson.D{{Key: field, Value: userID}})
	require.NoError(t, err)
	return n
}

func newCascade(mdb *mongo.Database, outboxRepo outbox.Repository) *accountdeletion.Cascade {
	return accountdeletion.NewCascade(mdb, outboxRepo, func() time.Time { return deleteAt })
}

func newOutboxRepo(mdb *mongo.Database) outbox.Repository {
	return outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError))
}

// userDeletedEntries returns the user.deleted rows of the event outbox for userID.
func userDeletedEntries(t *testing.T, mdb *mongo.Database, userID string) []outbox.Entry {
	t.Helper()
	cursor, err := mdb.Collection("event_outbox").Find(context.Background(),
		bson.D{{Key: "event_type", Value: accountdeletion.EventUserDeleted}, {Key: "body", Value: bson.D{{Key: "$exists", Value: true}}}})
	require.NoError(t, err)
	var entries []outbox.Entry
	require.NoError(t, cursor.All(context.Background(), &entries))

	var matching []outbox.Entry
	for _, entry := range entries {
		env, err := events.Decode(entry.Body)
		require.NoError(t, err)
		if env.Aggregate.ID == userID {
			matching = append(matching, entry)
		}
	}
	return matching
}

// TestCascade_Remove covers the account removal: the owned rows and the user record
// go, the tombstone and the event are written, and nothing else is touched.
func TestCascade_Remove(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, mdb *mongo.Database)
		check func(t *testing.T, mdb *mongo.Database)
	}{
		{
			name: "Happy path: the account's Sheets, counts, generations, quota rows and record are removed, the other account is kept",
			setup: func(t *testing.T, mdb *mongo.Database) {
				seedAccount(t, mdb, victimID)
				seedAccount(t, mdb, otherID)
			},
			check: func(t *testing.T, mdb *mongo.Database) {
				assert.Zero(t, countOwned(t, mdb, "sheets", "owner_id", victimID))
				assert.Zero(t, countOwned(t, mdb, "sheet_counts", "owner_id", victimID))
				assert.Zero(t, countOwned(t, mdb, "generations", "user_id", victimID))
				assert.Zero(t, countOwned(t, mdb, quota.TrialCollection, "_id", victimID))
				assert.Zero(t, countOwned(t, mdb, quota.UsageCollection, "user_id", victimID))
				assert.Zero(t, countOwned(t, mdb, "users", "_id", victimID))

				assert.Equal(t, int64(1), countOwned(t, mdb, "sheets", "owner_id", otherID))
				assert.Equal(t, int64(1), countOwned(t, mdb, "users", "_id", otherID))
			},
		},
		{
			name: "Edge: an account with no owned rows is still removed and tombstoned",
			setup: func(t *testing.T, mdb *mongo.Database) {
				_, err := mdb.Collection("users").InsertOne(context.Background(), bson.D{{Key: "_id", Value: victimID}})
				require.NoError(t, err)
			},
			check: func(t *testing.T, mdb *mongo.Database) {
				assert.Zero(t, countOwned(t, mdb, "users", "_id", victimID))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			tc.setup(t, mdb)

			cascade := newCascade(mdb, newOutboxRepo(mdb))
			require.NoError(t, cascade.Remove(context.Background(), victimID))

			tc.check(t, mdb)

			var tomb accountdeletion.Tombstone
			require.NoError(t, mdb.Collection("user_tombstones").FindOne(context.Background(), bson.D{{Key: "_id", Value: victimID}}).Decode(&tomb))
			assert.True(t, tomb.DeletedAt.Equal(deleteAt))

			var raw bson.M
			require.NoError(t, mdb.Collection("user_tombstones").FindOne(context.Background(), bson.D{{Key: "_id", Value: victimID}}).Decode(&raw))
			assert.ElementsMatch(t, []string{"_id", "deleted_at"}, keys(raw), "the tombstone must carry no PII")

			entries := userDeletedEntries(t, mdb, victimID)
			require.Len(t, entries, 1)
			env, err := events.Decode(entries[0].Body)
			require.NoError(t, err)
			assert.Equal(t, accountdeletion.EventUserDeleted, env.Type)
			assert.Equal(t, "user", env.Aggregate.Type)
			assert.Equal(t, victimID, env.Aggregate.ID)
			assert.JSONEq(t, `{}`, string(env.Data), "the event must carry no PII")
		})
	}
}

// TestCascade_Remove_Edges covers the cases that must not double-write or fail:
// a repeat run, an unknown account and a missing record.
func TestCascade_Remove_Edges(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade)
	}{
		{
			name: "Edge: a repeat after a committed removal writes no second event and changes no tombstone",
			run: func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade) {
				seedAccount(t, mdb, victimID)
				require.NoError(t, cascade.Remove(context.Background(), victimID))
				require.NoError(t, cascade.Remove(context.Background(), victimID))

				assert.Len(t, userDeletedEntries(t, mdb, victimID), 1)
				n, err := mdb.Collection("user_tombstones").CountDocuments(context.Background(), bson.D{{Key: "_id", Value: victimID}})
				require.NoError(t, err)
				assert.Equal(t, int64(1), n)
			},
		},
		{
			name: "Edge: an unknown account writes no tombstone and no event",
			run: func(t *testing.T, mdb *mongo.Database, cascade *accountdeletion.Cascade) {
				require.NoError(t, cascade.Remove(context.Background(), victimID))

				n, err := mdb.Collection("user_tombstones").CountDocuments(context.Background(), bson.D{{Key: "_id", Value: victimID}})
				require.NoError(t, err)
				assert.Zero(t, n)
				assert.Empty(t, userDeletedEntries(t, mdb, victimID))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			tc.run(t, mdb, newCascade(mdb, newOutboxRepo(mdb)))
		})
	}
}

// TestCascade_Remove_Error pins that a failure while the event is written reaches
// the caller, and that under the transactor the whole removal rolls back, so the
// account stays intact and a later run can finish it.
func TestCascade_Remove_Error(t *testing.T) {
	tests := []struct {
		name    string
		failure error
	}{
		{name: "Error: an outbox write failure aborts the removal and leaves the account intact", failure: errors.New("write refused")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := dbtest.Database(t)
			seedAccount(t, mdb, victimID)

			outboxRepo := outboxmocks.NewMockRepository(t)
			outboxRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(tc.failure).Once()
			cascade := newCascade(mdb, outboxRepo)
			tx := db.NewTransactor(mdb.Client(), nil)

			err := tx.WithTransaction(context.Background(), func(txCtx context.Context) error {
				return cascade.Remove(txCtx, victimID)
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.failure)

			assert.Equal(t, int64(1), countOwned(t, mdb, "sheets", "owner_id", victimID), "a rolled-back removal keeps the Sheets")
			assert.Equal(t, int64(1), countOwned(t, mdb, "users", "_id", victimID), "a rolled-back removal keeps the record")
			n, err := mdb.Collection("user_tombstones").CountDocuments(context.Background(), bson.D{{Key: "_id", Value: victimID}})
			require.NoError(t, err)
			assert.Zero(t, n, "a rolled-back removal writes no tombstone")
		})
	}
}

func keys(m bson.M) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
