package generation_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	jobA = "0190a1b2-0000-7000-8000-0000000000a1"
	jobB = "0190a1b2-0000-7000-8000-0000000000b2"
	jobC = "0190a1b2-0000-7000-8000-0000000000c3"
)

func newStore(t *testing.T) (*generation.MongoStore, *mongo.Database) {
	t.Helper()
	mdb := dbtest.Database(t)
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return generation.NewMongoStore(mdb), mdb
}

func queuedDoc(id, user string, created time.Time) generation.Document {
	return generation.Document{
		ID: id, UserID: user, TemplateID: templateID, Kind: "character", Concept: validText,
		Status: generation.StatusQueued, ActiveUserID: user,
		Quota:     generation.Reservation{Kind: generation.ReservationDaily, DayUTC: "2026-11-14"},
		CreatedAt: created, ExpiresAt: created.Add(generation.Retention),
	}
}

func TestMongoStore_Insert(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *generation.MongoStore)
		doc     generation.Document
		wantErr error
	}{
		{
			name:  "Happy path: the first job of a player is stored",
			doc:   queuedDoc(jobA, userID, now),
			setup: func(*testing.T, *generation.MongoStore) {},
		},
		{
			name: "Error: a second in-progress job of the same player is ErrActiveExists",
			setup: func(t *testing.T, s *generation.MongoStore) {
				require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now)))
			},
			doc:     queuedDoc(jobB, userID, now),
			wantErr: generation.ErrActiveExists,
		},
		{
			name: "Edge: another player is not blocked by a job in progress",
			setup: func(t *testing.T, s *generation.MongoStore) {
				require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now)))
			},
			doc: queuedDoc(jobB, otherUser, now),
		},
		{
			name: "Edge: a player may queue again once the first job left the in-progress slot",
			setup: func(t *testing.T, s *generation.MongoStore) {
				require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now)))
				_, ok, err := s.FailQueued(context.Background(), jobA, "provider_unavailable")
				require.NoError(t, err)
				require.True(t, ok)
			},
			doc: queuedDoc(jobB, userID, now),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			tc.setup(t, s)
			err := s.Insert(context.Background(), tc.doc)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMongoStore_FindOwned(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		user    string
		id      string
		wantErr error
	}{
		{name: "Happy path: the owner reads the document", user: userID, id: jobA},
		{name: "Error: another player gets not found", user: otherUser, id: jobA, wantErr: generation.ErrNotFound},
		{name: "Error: a missing document is not found", user: userID, id: jobC, wantErr: generation.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now)))

			doc, err := s.FindOwned(context.Background(), tc.user, tc.id)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, jobA, doc.ID)
			assert.Equal(t, userID, doc.UserID)
		})
	}
}

func TestMongoStore_MarkAccepted(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		status generation.Status
		user   string
		want   bool
	}{
		{name: "Happy path: a succeeded document is accepted once", status: generation.StatusSucceeded, user: userID, want: true},
		{name: "Edge: a queued document is not accepted", status: generation.StatusQueued, user: userID, want: false},
		{name: "Edge: a failed document is not accepted", status: generation.StatusFailed, user: userID, want: false},
		{name: "Edge: a foreign document is not accepted", status: generation.StatusSucceeded, user: otherUser, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			doc := queuedDoc(jobA, userID, now)
			doc.Status = tc.status
			_, err := mdb.Collection("generations").InsertOne(context.Background(), doc)
			require.NoError(t, err)

			ok, err := s.MarkAccepted(context.Background(), tc.user, jobA, jobB)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)

			if tc.want {
				again, err := s.MarkAccepted(context.Background(), tc.user, jobA, jobC)
				require.NoError(t, err)
				assert.False(t, again, "a second accept must not win")
			}
		})
	}
}

func TestMongoStore_ReserveCap(t *testing.T) {
	expires := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		limit    int
		reserves int
		wantUsed []int
		wantOK   []bool
	}{
		{name: "Edge: a zero cap refuses every job", limit: 0, reserves: 1, wantUsed: []int{0}, wantOK: []bool{false}},
		{name: "Edge: the last unit is accepted and the next is refused", limit: 2, reserves: 3, wantUsed: []int{1, 2, 2}, wantOK: []bool{true, true, false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			for i := range tc.reserves {
				used, ok, err := s.ReserveCap(context.Background(), "2026-11-14", tc.limit, expires)
				require.NoError(t, err)
				assert.Equal(t, tc.wantUsed[i], used, "count after attempt %d", i+1)
				assert.Equal(t, tc.wantOK[i], ok, "reservation %d", i+1)
			}
		})
	}
}

func TestMongoStore_ClaimUnpublished(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		created   time.Time
		limit     int
		wantCount int
	}{
		{name: "Edge: a job younger than 10 s is not republished yet", created: now.Add(-5 * time.Second), limit: 25, wantCount: 0},
		{name: "Happy path: an older queued job is claimed", created: now.Add(-time.Minute), limit: 25, wantCount: 1},
		{name: "Edge: the batch limit caps the claim", created: now.Add(-time.Minute), limit: 0, wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, tc.created)))

			claimed, err := s.ClaimUnpublished(context.Background(), "relay-1", now, time.Minute, tc.limit)
			require.NoError(t, err)
			assert.Len(t, claimed, tc.wantCount)

			if tc.wantCount == 0 {
				return
			}
			again, err := s.ClaimUnpublished(context.Background(), "relay-2", now.Add(30*time.Second), time.Minute, 25)
			require.NoError(t, err)
			assert.Empty(t, again, "a live lease must not be claimed by another sweeper")

			afterLease, err := s.ClaimUnpublished(context.Background(), "relay-2", now.Add(2*time.Minute), time.Minute, 25)
			require.NoError(t, err)
			assert.Len(t, afterLease, 1, "an expired lease is claimed again")
		})
	}
}

func TestMongoStore_ReschedulePublishAndFail(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	s, mdb := newStore(t)
	require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now.Add(-time.Minute))))

	require.NoError(t, s.ReschedulePublish(context.Background(), jobA, now))
	var stored generation.Document
	require.NoError(t, mdb.Collection("generations").FindOne(context.Background(), bson.M{"_id": jobA}).Decode(&stored))
	assert.Equal(t, 1, stored.PublishAttempts)

	before, ok, err := s.FailQueued(context.Background(), jobA, generation.ErrorCodePublishFailed)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, generation.StatusQueued, before.Status, "the move returns the document as it was")
	assert.Equal(t, generation.Reservation{Kind: generation.ReservationDaily, DayUTC: "2026-11-14"}, before.Quota)

	var failed generation.Document
	require.NoError(t, mdb.Collection("generations").FindOne(context.Background(), bson.M{"_id": jobA}).Decode(&failed))
	assert.Equal(t, generation.StatusFailed, failed.Status)
	assert.Equal(t, generation.ErrorCodePublishFailed, failed.ErrorCode)
	assert.Empty(t, failed.ActiveUserID, "a failed job releases the player's slot")

	_, again, err := s.FailQueued(context.Background(), jobA, generation.ErrorCodePublishFailed)
	require.NoError(t, err)
	assert.False(t, again, "a failed job is not failed twice")
}

func TestMongoStore_OldestQueued(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		seed   func(t *testing.T, s *generation.MongoStore)
		wantOK bool
		wantAt time.Time
	}{
		{name: "Edge: an empty store reports nothing pending", seed: func(*testing.T, *generation.MongoStore) {}, wantOK: false},
		{name: "Happy path: the oldest queued job is reported", seed: func(t *testing.T, s *generation.MongoStore) {
			require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, now.Add(-time.Hour))))
			require.NoError(t, s.Insert(context.Background(), queuedDoc(jobB, otherUser, now.Add(-time.Minute))))
		}, wantOK: true, wantAt: now.Add(-time.Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newStore(t)
			tc.seed(t, s)
			at, ok, err := s.OldestQueued(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.True(t, tc.wantAt.Equal(at))
			}
		})
	}
}

func TestMongoStore_ListBySheet(t *testing.T) {
	sheet := "0190a1b2-0000-7000-8000-0000000000aa"
	start := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	early := queuedDoc(jobA, userID, start.Add(-time.Hour))
	early.SheetID = sheet
	inside := queuedDoc(jobB, otherUser, start)
	inside.SheetID = sheet
	late := queuedDoc(jobC, userID, start.Add(time.Hour))
	late.SheetID = sheet
	other := queuedDoc("0190a1b2-0000-7000-8000-0000000000d4", userID, start)
	other.SheetID = "0190a1b2-0000-7000-8000-0000000000bb"

	s, _ := newStore(t)
	for _, doc := range []generation.Document{early, inside, late, other} {
		doc.ActiveUserID = ""
		require.NoError(t, s.Insert(context.Background(), doc))
	}

	found, err := s.ListBySheet(context.Background(), sheet, start, start.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, found, 2)
	assert.Equal(t, jobB, found[0].ID)
	assert.Equal(t, jobC, found[1].ID)
}

func TestMongoStore_ListBySheetRejectsMissingSheet(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.ListBySheet(context.Background(), "", time.Time{}, time.Time{})
	require.Error(t, err)
}

func TestMongoStore_CompleteRecordsMetadata(t *testing.T) {
	s, mdb := newStore(t)
	runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))

	landed, err := s.Complete(context.Background(), jobA, map[string]any{"name": "Aria"}, generation.Metadata{
		PromptVersion: "v1+123456789abc",
		Model:         "z-ai/glm-5.3-flash",
	})
	require.NoError(t, err)
	require.True(t, landed)

	raw := readRaw(t, mdb, jobA)
	assert.Equal(t, "v1+123456789abc", raw["prompt_version"])
	assert.Equal(t, "z-ai/glm-5.3-flash", raw["model"])
}

func TestMongoStore_OldDocumentsReadWithoutMetadata(t *testing.T) {
	s, _ := newStore(t)
	require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, jobStart)))

	doc, err := s.FindOwned(context.Background(), userID, jobA)
	require.NoError(t, err)
	assert.Empty(t, doc.PromptVersion)
	assert.Empty(t, doc.Model)
}
