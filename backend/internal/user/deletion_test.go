package user_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/logger"
)

const (
	deletionUserA = "01900000-0000-7000-8000-0000000000a1"
	deletionUserB = "01900000-0000-7000-8000-0000000000b2"
)

var (
	markedAt = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	laterAt  = markedAt.Add(time.Hour)
)

func seedUser(t *testing.T, mdb *mongo.Database, doc bson.D) {
	t.Helper()
	_, err := mdb.Collection("users").InsertOne(context.Background(), withIdentity(doc))
	require.NoError(t, err)
}

func withIdentity(doc bson.D) bson.D {
	id := doc[0].Value.(string)
	if !hasKey(doc, "email") {
		doc = append(doc, bson.E{Key: "email", Value: id + "@example.com"})
	}
	if !hasKey(doc, "username") {
		doc = append(doc, bson.E{Key: "username", Value: id})
	}
	return doc
}

func hasKey(doc bson.D, key string) bool {
	for _, e := range doc {
		if e.Key == key {
			return true
		}
	}
	return false
}

func deletionStore(mdb *mongo.Database) user.DeletionStore {
	return user.NewDeletionStore(mdb, logger.NewLogger(slog.LevelError))
}

func TestUser_IsDeleting(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "Happy path: an active account is not deleting", status: "", want: false},
		{name: "Happy path: a deleting account is deleting", status: user.StatusDeleting, want: true},
		{name: "Edge: a dead deletion keeps the account blocked", status: user.StatusDeletionDead, want: true},
		{name: "Edge: an unknown status is not treated as deleting", status: "something_else", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			account := user.User{Status: tc.status}
			assert.Equal(t, tc.want, account.IsDeleting())
		})
	}
}

func TestUserRepository_MarkDeleting(t *testing.T) {
	tests := []struct {
		name       string
		seed       bson.D
		id         string
		repeat     bool
		wantErr    error
		wantStatus string
		wantAt     time.Time
	}{
		{
			name:       "Happy path: an active account is marked deleting at the request instant",
			seed:       bson.D{{Key: "_id", Value: deletionUserA}},
			id:         deletionUserA,
			wantStatus: user.StatusDeleting,
			wantAt:     markedAt,
		},
		{
			name:       "Edge: a repeated request keeps the first deleting_at",
			seed:       bson.D{{Key: "_id", Value: deletionUserA}},
			id:         deletionUserA,
			repeat:     true,
			wantStatus: user.StatusDeleting,
			wantAt:     markedAt,
		},
		{
			name:       "Edge: a dead deletion stays dead and is not reset to deleting",
			seed:       bson.D{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeletionDead}},
			id:         deletionUserA,
			wantStatus: user.StatusDeletionDead,
		},
		{
			name:    "Error: an unknown account reports not found and is never created",
			seed:    bson.D{{Key: "_id", Value: deletionUserB}},
			id:      deletionUserA,
			wantErr: user.ErrUserNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, collection := newUserRepositoryFixture(t)
			seedUser(t, collection.Database(), tc.seed)

			err := repo.MarkDeleting(context.Background(), tc.id, markedAt)
			if tc.repeat {
				err = repo.MarkDeleting(context.Background(), tc.id, laterAt)
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				n, countErr := collection.CountDocuments(context.Background(), bson.D{{Key: "_id", Value: tc.id}})
				require.NoError(t, countErr)
				assert.Zero(t, n, "an unknown account must not be created")
				return
			}
			require.NoError(t, err)

			var stored user.User
			require.NoError(t, collection.FindOne(context.Background(), bson.D{{Key: "_id", Value: tc.id}}).Decode(&stored))
			assert.Equal(t, tc.wantStatus, stored.Status)
			if !tc.wantAt.IsZero() {
				require.NotNil(t, stored.DeletingAt)
				assert.True(t, stored.DeletingAt.Equal(tc.wantAt), "got %s", stored.DeletingAt)
			}
		})
	}
}

func TestDeletionStore_ClaimDeleting(t *testing.T) {
	tests := []struct {
		name       string
		seed       []bson.D
		now        time.Time
		limit      int
		wantIDs    []string
		claimTwice bool
	}{
		{
			name: "Happy path: a deleting account is claimed with its attempt count",
			seed: []bson.D{
				{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}},
			},
			now:     laterAt,
			limit:   10,
			wantIDs: []string{deletionUserA},
		},
		{
			name: "Edge: an active or dead account is never claimed",
			seed: []bson.D{
				{{Key: "_id", Value: deletionUserA}},
				{{Key: "_id", Value: deletionUserB}, {Key: "status", Value: user.StatusDeletionDead}},
			},
			now:   laterAt,
			limit: 10,
		},
		{
			name: "Edge: a second claim inside the lease returns nothing",
			seed: []bson.D{
				{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}},
			},
			now:        laterAt,
			limit:      10,
			wantIDs:    []string{deletionUserA},
			claimTwice: true,
		},
		{
			name: "Edge: the limit caps the batch",
			seed: []bson.D{
				{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}},
				{{Key: "_id", Value: deletionUserB}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}},
			},
			now:   laterAt,
			limit: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb := newUserRepositoryDatabase(t)
			for _, doc := range tc.seed {
				seedUser(t, mdb, doc)
			}
			store := deletionStore(mdb)

			claimed, err := store.ClaimDeleting(context.Background(), "claimer", tc.now, time.Minute, tc.limit)
			require.NoError(t, err)
			var ids []string
			for _, account := range claimed {
				ids = append(ids, account.ID)
			}
			if tc.limit == 1 {
				assert.Len(t, ids, 1)
			} else {
				assert.ElementsMatch(t, tc.wantIDs, ids)
			}

			if tc.claimTwice {
				again, err := store.ClaimDeleting(context.Background(), "other", tc.now, time.Minute, tc.limit)
				require.NoError(t, err)
				assert.Empty(t, again, "a live lease must block a second claim")
			}
		})
	}
}

func TestDeletionStore_Transitions(t *testing.T) {
	t.Run("Happy path: a reschedule counts the attempt and releases the lease", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		seedUser(t, mdb, bson.D{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}})
		store := deletionStore(mdb)

		require.NoError(t, store.RescheduleDeleting(context.Background(), deletionUserA, laterAt, "boom"))

		var stored user.User
		require.NoError(t, mdb.Collection("users").FindOne(context.Background(), bson.D{{Key: "_id", Value: deletionUserA}}).Decode(&stored))
		assert.Equal(t, 1, stored.DeletionAttempts)
		assert.Equal(t, "boom", stored.DeletionLastError)
		assert.Nil(t, stored.DeletionLockedUntil)
	})

	t.Run("Edge: a future retry is not claimed before its time", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		seedUser(t, mdb, bson.D{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}})
		store := deletionStore(mdb)
		require.NoError(t, store.RescheduleDeleting(context.Background(), deletionUserA, laterAt.Add(time.Hour), "boom"))

		claimed, err := store.ClaimDeleting(context.Background(), "c", laterAt, time.Minute, 10)
		require.NoError(t, err)
		assert.Empty(t, claimed)

		claimed, err = store.ClaimDeleting(context.Background(), "c", laterAt.Add(2*time.Hour), time.Minute, 10)
		require.NoError(t, err)
		assert.Len(t, claimed, 1)
	})

	t.Run("Error: a reschedule of an account that left the queue reports the lost claim", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		store := deletionStore(mdb)

		err := store.RescheduleDeleting(context.Background(), deletionUserA, laterAt, "boom")
		require.ErrorIs(t, err, user.ErrDeletionClaimLost)
	})

	t.Run("Happy path: a dead account is never claimed again", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		seedUser(t, mdb, bson.D{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}})
		store := deletionStore(mdb)

		require.NoError(t, store.MarkDeletionDead(context.Background(), deletionUserA, "gave up"))

		claimed, err := store.ClaimDeleting(context.Background(), "c", laterAt, time.Minute, 10)
		require.NoError(t, err)
		assert.Empty(t, claimed)
	})

	t.Run("Error: marking a missing account dead reports the lost claim", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		err := deletionStore(mdb).MarkDeletionDead(context.Background(), deletionUserA, "gone")
		require.ErrorIs(t, err, user.ErrDeletionClaimLost)
	})

	t.Run("Edge: the oldest-pending read reports none when no account is deleting", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		_, pending, err := deletionStore(mdb).OldestDeleting(context.Background())
		require.NoError(t, err)
		assert.False(t, pending)
	})

	t.Run("Happy path: the oldest-pending read returns the earliest deleting_at", func(t *testing.T) {
		mdb := newUserRepositoryDatabase(t)
		seedUser(t, mdb, bson.D{{Key: "_id", Value: deletionUserA}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: laterAt}})
		seedUser(t, mdb, bson.D{{Key: "_id", Value: deletionUserB}, {Key: "status", Value: user.StatusDeleting}, {Key: "deleting_at", Value: markedAt}})

		at, pending, err := deletionStore(mdb).OldestDeleting(context.Background())
		require.NoError(t, err)
		require.True(t, pending)
		assert.True(t, at.Equal(markedAt))
	})
}
