package generation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"legnaris/backend/internal/generation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// jobStart is the claim time of the job store tests.
var jobStart = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// otherUserID is a second player, for tests that hold two in-progress jobs.
const otherUserID = "0190a1b2-0000-7000-8000-000000000002"

// readRaw returns the stored document of id as written, for field-level assertions.
func readRaw(t *testing.T, mdb *mongo.Database, id string) bson.M {
	t.Helper()
	var raw bson.M
	require.NoError(t, mdb.Collection("generations").FindOne(context.Background(), bson.M{"_id": id}).Decode(&raw))
	return raw
}

// runningDoc stores a running document claimed at jobStart with the given lease end.
func runningDoc(t *testing.T, s *generation.MongoStore, mdb *mongo.Database, id string, leaseUntil time.Time) {
	t.Helper()
	runningDocFor(t, s, id, userID, leaseUntil)
}

// runningDocFor is runningDoc for a given player. Each player has one in-progress
// job at a time, so a second running job needs a second player.
func runningDocFor(t *testing.T, s *generation.MongoStore, id, user string, leaseUntil time.Time) {
	t.Helper()
	doc := queuedDoc(id, user, jobStart)
	require.NoError(t, s.Insert(context.Background(), doc))
	_, _, err := s.ClaimQueued(context.Background(), id, jobStart, leaseUntil.Sub(jobStart))
	require.NoError(t, err)
}

// TestMongoStore_ClaimQueued pins the queued-to-running transition and its idempotency.
func TestMongoStore_ClaimQueued(t *testing.T) {
	t.Run("Happy path: a queued job becomes running under a lease and counts one attempt", func(t *testing.T) {
		s, mdb := newStore(t)
		require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, jobStart)))

		doc, claimed, err := s.ClaimQueued(context.Background(), jobA, jobStart, generation.LeaseDuration)
		require.NoError(t, err)
		require.True(t, claimed)
		assert.Equal(t, generation.StatusRunning, doc.Status)
		assert.Equal(t, 1, doc.Attempts)

		raw := readRaw(t, mdb, jobA)
		assert.Equal(t, "running", raw["status"])
		assert.Equal(t, jobStart.Add(generation.LeaseDuration).UTC(), raw["lease_until"].(bson.DateTime).Time().UTC())
	})

	tests := []struct {
		name  string
		setup func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database)
		id    string
	}{
		{
			name: "Edge: a duplicate delivery of a running job is a no-op",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
			},
			id: jobA,
		},
		{
			name: "Edge: a succeeded job is never claimed again",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, err := s.Complete(context.Background(), jobA, map[string]any{"name": "x"})
				require.NoError(t, err)
			},
			id: jobA,
		},
		{
			name:  "Edge: a document removed by account deletion or consent revocation is a no-op",
			setup: func(*testing.T, *generation.MongoStore, *mongo.Database) {},
			id:    jobB,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			tc.setup(t, s, mdb)
			_, claimed, err := s.ClaimQueued(context.Background(), tc.id, jobStart, generation.LeaseDuration)
			require.NoError(t, err)
			assert.False(t, claimed)
		})
	}
}

// TestMongoStore_Complete pins the running-to-succeeded write that only a live job may make.
func TestMongoStore_Complete(t *testing.T) {
	values := map[string]any{"identity": map[string]any{"name": "Aria"}}

	t.Run("Happy path: a running job stores its values, succeeds and releases the slot", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))

		landed, err := s.Complete(context.Background(), jobA, values)
		require.NoError(t, err)
		assert.True(t, landed)

		raw := readRaw(t, mdb, jobA)
		assert.Equal(t, "succeeded", raw["status"])
		assert.NotContains(t, raw, "active_user_id", "the in-progress slot is released")
		assert.NotContains(t, raw, "lease_until")
		assert.Contains(t, fmt.Sprint(raw["values"]), "Aria")
	})

	tests := []struct {
		name  string
		setup func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database)
		id    string
	}{
		{
			name: "Edge: a second success write on a succeeded job does nothing",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, err := s.Complete(context.Background(), jobA, values)
				require.NoError(t, err)
			},
			id: jobA,
		},
		{
			name: "Edge: a job whose lease was reaped is failed, so its late success is discarded",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, _, err := s.FailExpired(context.Background(), jobA, jobStart.Add(2*time.Minute), generation.ErrorCodeLeaseExpired)
				require.NoError(t, err)
			},
			id: jobA,
		},
		{
			name:  "Edge: a document removed by consent revocation is a no-op",
			setup: func(*testing.T, *generation.MongoStore, *mongo.Database) {},
			id:    jobB,
		},
		{
			name: "Edge: a queued document is not running yet and cannot be completed",
			setup: func(t *testing.T, s *generation.MongoStore, _ *mongo.Database) {
				require.NoError(t, s.Insert(context.Background(), queuedDoc(jobA, userID, jobStart)))
			},
			id: jobA,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			tc.setup(t, s, mdb)
			landed, err := s.Complete(context.Background(), tc.id, values)
			require.NoError(t, err)
			assert.False(t, landed)
		})
	}
}

// TestMongoStore_FailRunning pins the running-to-failed write and what it returns for a refund.
func TestMongoStore_FailRunning(t *testing.T) {
	t.Run("Happy path: a running job fails, releases the slot and returns its prior state", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))

		before, settled, err := s.FailRunning(context.Background(), jobA, generation.ErrorCodeProviderFailed)
		require.NoError(t, err)
		require.True(t, settled)
		assert.Equal(t, generation.StatusRunning, before.Status)
		assert.Equal(t, generation.ReservationTrial, before.Quota.Kind)

		raw := readRaw(t, mdb, jobA)
		assert.Equal(t, "failed", raw["status"])
		assert.Equal(t, generation.ErrorCodeProviderFailed, raw["error_code"])
		assert.NotContains(t, raw, "active_user_id")
	})

	tests := []struct {
		name  string
		setup func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database)
		id    string
	}{
		{
			name: "Edge: a failed job is not failed twice, so it is not refunded twice",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, _, err := s.FailRunning(context.Background(), jobA, generation.ErrorCodeProviderFailed)
				require.NoError(t, err)
			},
			id: jobA,
		},
		{
			name:  "Edge: a removed document is not settled and refunds nothing",
			setup: func(*testing.T, *generation.MongoStore, *mongo.Database) {},
			id:    jobB,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			tc.setup(t, s, mdb)
			_, settled, err := s.FailRunning(context.Background(), tc.id, generation.ErrorCodeProviderFailed)
			require.NoError(t, err)
			assert.False(t, settled)
		})
	}
}

// TestMongoStore_FailExpired pins the reap of a running job whose lease ended.
func TestMongoStore_FailExpired(t *testing.T) {
	t.Run("Happy path: a job whose lease ended at or before now is failed as lease_expired", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))

		before, failed, err := s.FailExpired(context.Background(), jobA, jobStart.Add(time.Minute), generation.ErrorCodeLeaseExpired)
		require.NoError(t, err)
		require.True(t, failed)
		assert.Equal(t, generation.StatusRunning, before.Status)
		assert.Equal(t, "failed", readRaw(t, mdb, jobA)["status"])
	})

	tests := []struct {
		name  string
		setup func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database)
		id    string
		now   time.Time
	}{
		{
			name: "Edge: a job with a live lease is never reaped",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
			},
			id:  jobA,
			now: jobStart.Add(time.Minute - time.Second),
		},
		{
			name: "Edge: a job already failed is not reaped twice",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, _, err := s.FailRunning(context.Background(), jobA, generation.ErrorCodeProviderFailed)
				require.NoError(t, err)
			},
			id:  jobA,
			now: jobStart.Add(time.Hour),
		},
		{
			name:  "Edge: a removed document is not reaped",
			setup: func(*testing.T, *generation.MongoStore, *mongo.Database) {},
			id:    jobB,
			now:   jobStart.Add(time.Hour),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			tc.setup(t, s, mdb)
			_, failed, err := s.FailExpired(context.Background(), tc.id, tc.now, generation.ErrorCodeLeaseExpired)
			require.NoError(t, err)
			assert.False(t, failed)
		})
	}
}

// TestMongoStore_ClaimExpired pins the reaper's lease over expired running jobs.
func TestMongoStore_ClaimExpired(t *testing.T) {
	t.Run("Happy path: only the expired running jobs are claimed", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
		runningDocFor(t, s, jobB, otherUserID, jobStart.Add(time.Hour))

		claimed, err := s.ClaimExpired(context.Background(), jobStart.Add(2*time.Minute), time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		assert.Equal(t, jobA, claimed[0].ID)
	})

	tests := []struct {
		name  string
		setup func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database)
		limit int
		now   time.Time
		want  int
	}{
		{
			name: "Edge: a second sweep inside the reap lease claims nothing",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, err := s.ClaimExpired(context.Background(), jobStart.Add(2*time.Minute), time.Minute, 10)
				require.NoError(t, err)
			},
			limit: 10,
			now:   jobStart.Add(2 * time.Minute),
			want:  0,
		},
		{
			name: "Edge: a reap lease that has expired is claimed again",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				_, err := s.ClaimExpired(context.Background(), jobStart.Add(2*time.Minute), time.Minute, 10)
				require.NoError(t, err)
			},
			limit: 10,
			now:   jobStart.Add(4 * time.Minute),
			want:  1,
		},
		{
			name: "Edge: a zero limit claims nothing",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
			},
			limit: 0,
			now:   jobStart.Add(2 * time.Minute),
			want:  0,
		},
		{
			name: "Edge: a limit of one claims one of two expired jobs",
			setup: func(t *testing.T, s *generation.MongoStore, mdb *mongo.Database) {
				runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
				runningDocFor(t, s, jobB, otherUserID, jobStart.Add(time.Minute))
			},
			limit: 1,
			now:   jobStart.Add(2 * time.Minute),
			want:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, mdb := newStore(t)
			tc.setup(t, s, mdb)
			claimed, err := s.ClaimExpired(context.Background(), tc.now, time.Minute, tc.limit)
			require.NoError(t, err)
			assert.Len(t, claimed, tc.want)
		})
	}
}

// TestMongoStore_RescheduleExpired pins the retry bookkeeping of the reaper.
func TestMongoStore_RescheduleExpired(t *testing.T) {
	t.Run("Happy path: a rescheduled job counts one reap attempt and is due at nextAt", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
		nextAt := jobStart.Add(5 * time.Minute)

		require.NoError(t, s.RescheduleExpired(context.Background(), jobA, nextAt))
		raw := readRaw(t, mdb, jobA)
		assert.EqualValues(t, 1, raw["reap_attempts"])
		assert.Equal(t, nextAt.UTC(), raw["reap_lease_until"].(bson.DateTime).Time().UTC())
	})

	t.Run("Edge: rescheduling a job that is no longer running is a silent no-op", func(t *testing.T) {
		s, _ := newStore(t)
		require.NoError(t, s.RescheduleExpired(context.Background(), jobB, jobStart))
	})
}

// TestMongoStore_OldestRunning covers the backlog age of running jobs.
func TestMongoStore_OldestRunning(t *testing.T) {
	t.Run("Edge: no running job reports none", func(t *testing.T) {
		s, _ := newStore(t)
		_, ok, err := s.OldestRunning(context.Background())
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Happy path: the oldest running job is reported", func(t *testing.T) {
		s, mdb := newStore(t)
		runningDoc(t, s, mdb, jobA, jobStart.Add(time.Minute))
		require.NoError(t, s.Insert(context.Background(), queuedDoc(jobB, otherUserID, jobStart.Add(time.Second))))

		created, ok, err := s.OldestRunning(context.Background())
		require.NoError(t, err)
		require.True(t, ok)
		assert.True(t, created.Equal(jobStart))
	})
}
