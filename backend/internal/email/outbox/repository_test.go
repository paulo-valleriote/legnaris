package outbox_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const repositoryTestDatabase = "legnaris_email_outbox_test"

// outboxTestRetention is a retention window distinct from the default so the
// suite proves the configured window reaches the TTL index (EML-17).
const outboxTestRetention = 24 * time.Hour

// newOutboxFixture connects to MongoDB, gives the test a clean email_outbox
// collection with the production indexes, and self-skips when MongoDB is
// unavailable so the unit run stays hermetic (EML-20).
func newOutboxFixture(t *testing.T) (outbox.Repository, *mongo.Collection) {
	t.Helper()

	log := logger.NewLogger(slog.LevelError)
	cfg := env.NewEnv(log)

	database, err := db.InitMongo(cfg)
	if err != nil {
		t.Skipf("mongo unavailable — skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
		defer cancel()
		_ = database.Close(closeCtx)
	})

	mdb := database.Client.Database(repositoryTestDatabase)
	_ = mdb.Collection("email_outbox").Drop(context.Background())
	t.Cleanup(func() {
		_ = mdb.Collection("email_outbox").Drop(context.Background())
	})

	require.NoError(t, db.EnsureIndexesWithRetention(context.Background(), mdb, outboxTestRetention))
	return outbox.NewRepository(mdb, logger.NewLogger(slog.LevelError)), mdb.Collection("email_outbox")
}

func mustIntent(t *testing.T, to string, now time.Time) outbox.Intent {
	t.Helper()

	intent, err := outbox.NewIntent("", to, "login_otp", samplePayload(), now)
	require.NoError(t, err)
	return intent
}

// TestRepository_Create writes the intention durably before any acceptance is
// reported and never publishes (EML-01).
func TestRepository_Create(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	intent := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, intent))

	t.Run("Happy path: one document is stored", func(t *testing.T) {
		count, err := collection.CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("Happy path: the stored shape round-trips", func(t *testing.T) {
		var stored outbox.Intent
		require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: intent.ID}}).Decode(&stored))
		assert.Equal(t, intent, stored)
	})

	t.Run("Error: a non-canonical identity is rejected and not stored", func(t *testing.T) {
		bad := intent
		bad.ID = "not-a-uuid"

		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.ErrorIs(t, err, outbox.ErrInvalidIntentID)

		count, countErr := collection.CountDocuments(ctx, bson.D{})
		require.NoError(t, countErr)
		assert.Equal(t, int64(1), count, "a rejected identity must not add a document")
	})
}

// TestRepository_Claim_Concurrency proves two publishers polling in the same
// window never share a row (EML-03, SC-002).
func TestRepository_Claim_Concurrency(t *testing.T) {
	repo, _ := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	const rows = 6
	for i := 0; i < rows; i++ {
		require.NoError(t, repo.Create(ctx, mustIntent(t, fmt.Sprintf("player%d@example.com", i), now)))
	}

	type result struct {
		ids []string
		err error
	}

	var wg sync.WaitGroup
	claims := make(chan result, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(claimer string) {
			defer wg.Done()
			claimed, err := repo.Claim(ctx, claimer, now, 30*time.Second, rows)
			ids := make([]string, 0, len(claimed))
			for _, intent := range claimed {
				ids = append(ids, intent.ID)
			}
			claims <- result{ids: ids, err: err}
		}(fmt.Sprintf("relay-%d", i))
	}
	wg.Wait()
	close(claims)

	seen := make(map[string]int)
	for claimed := range claims {
		require.NoError(t, claimed.err)
		for _, id := range claimed.ids {
			seen[id]++
		}
	}

	t.Run("Happy path: every pending row is claimed", func(t *testing.T) {
		assert.Len(t, seen, rows)
	})

	t.Run("Edge: no row is claimed by both publishers", func(t *testing.T) {
		for id, count := range seen {
			assert.Equalf(t, 1, count, "row %s must be claimed exactly once", id)
		}
	})
}

// TestRepository_Claim_Lease covers the lease semantics a crash depends on: a
// live lease hides a row and an expired lease makes it eligible again
// (EML-03, SC-001).
func TestRepository_Claim_Lease(t *testing.T) {
	repo, _ := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()
	lease := 30 * time.Second

	intent := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, intent))

	t.Run("Happy path: an eligible row is claimed", func(t *testing.T) {
		claimed, err := repo.Claim(ctx, "relay-a", now, lease, 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		assert.Equal(t, intent.ID, claimed[0].ID)
		assert.NotNil(t, claimed[0].LockedUntil, "a claim must hold a lease")
		assert.Equal(t, "relay-a", claimed[0].LockedBy)
	})

	t.Run("Edge: a live lease hides the row from another publisher", func(t *testing.T) {
		claimed, err := repo.Claim(ctx, "relay-b", now.Add(time.Second), lease, 1)
		require.NoError(t, err)
		assert.Empty(t, claimed, "a leased row must not be claimable while its lease is live")
	})

	t.Run("Edge: a row is claimable exactly when its lease expires", func(t *testing.T) {
		claimed, err := repo.Claim(ctx, "relay-b", now.Add(lease), lease, 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1, "locked_until == now must be eligible ($lte)")
		assert.Equal(t, intent.ID, claimed[0].ID)
	})
}

// TestRepository_Reschedule proves a failed publication increments the attempt
// count and defers eligibility, without ever losing the row (EML-04).
func TestRepository_Reschedule(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()
	nextAt := now.Add(5 * time.Second)

	intent := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, intent))
	_, err := repo.Claim(ctx, "relay-a", now, 30*time.Second, 1)
	require.NoError(t, err)

	require.NoError(t, repo.Reschedule(ctx, intent.ID, now, nextAt, "broker nacked"))

	var stored outbox.Intent
	require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: intent.ID}}).Decode(&stored))

	t.Run("Happy path: the attempt count is incremented and eligibility deferred", func(t *testing.T) {
		assert.Equal(t, 1, stored.Attempts)
		assert.Equal(t, outbox.StatusPending, stored.Status)
		assert.Equal(t, nextAt, stored.NextAttemptAt)
		assert.Equal(t, "broker nacked", stored.LastError)
	})

	t.Run("Edge: the rescheduled row is not eligible before next_attempt_at", func(t *testing.T) {
		claimed, claimErr := repo.Claim(ctx, "relay-b", now.Add(time.Second), 30*time.Second, 1)
		require.NoError(t, claimErr)
		assert.Empty(t, claimed)
	})

	t.Run("Edge: the rescheduled row is eligible again at next_attempt_at", func(t *testing.T) {
		claimed, claimErr := repo.Claim(ctx, "relay-b", nextAt, 30*time.Second, 1)
		require.NoError(t, claimErr)
		require.Len(t, claimed, 1)
		assert.Equal(t, 1, claimed[0].Attempts)
	})

	t.Run("Error: a non-canonical identity is rejected", func(t *testing.T) {
		assert.ErrorIs(t, repo.Reschedule(ctx, "not-a-uuid", now, nextAt, "x"), outbox.ErrInvalidIntentID)
	})
}

// TestRepository_MarkPublished proves a published row is terminal and hides
// from future claims (EML-02).
func TestRepository_MarkPublished(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()
	publishedAt := now.Add(time.Second)

	intent := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, intent))
	require.NoError(t, repo.MarkPublished(ctx, intent.ID, publishedAt))

	var stored outbox.Intent
	require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: intent.ID}}).Decode(&stored))

	t.Run("Happy path: the row is published with a timestamp", func(t *testing.T) {
		assert.Equal(t, outbox.StatusPublished, stored.Status)
		require.NotNil(t, stored.PublishedAt)
		assert.Equal(t, publishedAt, *stored.PublishedAt)
	})

	t.Run("Edge: a published row is never claimed again", func(t *testing.T) {
		claimed, err := repo.Claim(ctx, "relay-a", publishedAt.Add(time.Hour), 30*time.Second, 1)
		require.NoError(t, err)
		assert.Empty(t, claimed)
	})

	t.Run("Error: a non-canonical identity is rejected", func(t *testing.T) {
		assert.ErrorIs(t, repo.MarkPublished(ctx, "not-a-uuid", now), outbox.ErrInvalidIntentID)
	})
}

// TestRepository_MarkTerminal proves a failed row is surfaced and never
// republished (EML-04).
func TestRepository_MarkTerminal(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	intent := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, intent))
	require.NoError(t, repo.MarkTerminal(ctx, intent.ID, now, "attempt cap reached"))

	var stored outbox.Intent
	require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: intent.ID}}).Decode(&stored))

	t.Run("Happy path: the row is failed for operator attention", func(t *testing.T) {
		assert.Equal(t, outbox.StatusFailed, stored.Status)
		assert.Equal(t, "attempt cap reached", stored.LastError)
		assert.Nil(t, stored.PublishedAt)
	})

	t.Run("Edge: a failed row is never claimed again", func(t *testing.T) {
		claimed, err := repo.Claim(ctx, "relay-a", now.Add(time.Hour), 30*time.Second, 1)
		require.NoError(t, err)
		assert.Empty(t, claimed)
	})

	t.Run("Error: a non-canonical identity is rejected", func(t *testing.T) {
		assert.ErrorIs(t, repo.MarkTerminal(ctx, "not-a-uuid", now, "x"), outbox.ErrInvalidIntentID)
	})
}

// TestRepository_Backlog counts pending rows only (EML-23).
func TestRepository_Backlog(t *testing.T) {
	repo, _ := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	t.Run("Edge: an empty outbox has a zero backlog", func(t *testing.T) {
		backlog, err := repo.Backlog(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, backlog)
	})

	pendingA := mustIntent(t, "a@example.com", now)
	pendingB := mustIntent(t, "b@example.com", now)
	published := mustIntent(t, "c@example.com", now)
	failed := mustIntent(t, "d@example.com", now)
	require.NoError(t, repo.Create(ctx, pendingA))
	require.NoError(t, repo.Create(ctx, pendingB))
	require.NoError(t, repo.Create(ctx, published))
	require.NoError(t, repo.Create(ctx, failed))
	require.NoError(t, repo.MarkPublished(ctx, published.ID, now))
	require.NoError(t, repo.MarkTerminal(ctx, failed.ID, now, "x"))

	t.Run("Happy path: only pending rows are counted", func(t *testing.T) {
		backlog, err := repo.Backlog(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, backlog)
	})
}

// TestRepository_Indexes pins the idempotently created indexes the claim query
// and the retention janitor depend on (EML-17).
func TestRepository_Indexes(t *testing.T) {
	_, collection := newOutboxFixture(t)

	cursor, err := collection.Indexes().List(context.Background())
	require.NoError(t, err)

	var listed []bson.M
	require.NoError(t, cursor.All(context.Background(), &listed))

	byName := make(map[string]bson.M, len(listed))
	for _, index := range listed {
		name, ok := index["name"].(string)
		require.True(t, ok, "every index must carry a string name")
		byName[name] = index
	}

	t.Run("Happy path: the claim index exists", func(t *testing.T) {
		_, ok := byName["email_outbox_status_next_attempt"]
		assert.True(t, ok, "the {status, next_attempt_at} index must exist, got %v", byName)
	})

	t.Run("Happy path: the retention TTL index exists with the documented window", func(t *testing.T) {
		ttl, ok := byName["email_outbox_published_at_ttl"]
		require.True(t, ok, "the published_at TTL index must exist, got %v", byName)

		seconds, ok := ttl["expireAfterSeconds"].(int32)
		require.True(t, ok, "the TTL index must carry expireAfterSeconds")
		assert.Equal(t, int32(outboxTestRetention.Seconds()), seconds)
	})
}

// TestRepository_Clone proves a resend re-records the same payload under a fresh
// identity — what keeps the provider's idempotency key from suppressing the
// second send — and leaves the original row untouched (OTP-10, EML-18).
func TestRepository_Clone(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	original := mustIntent(t, "player@example.com", now)
	require.NoError(t, repo.Create(ctx, original))

	t.Run("Happy path: the clone carries a fresh identity, the same message and a fresh eligibility", func(t *testing.T) {
		later := now.Add(time.Minute)
		cloned, err := repo.Clone(ctx, original.ID, later)
		require.NoError(t, err)

		assert.NotEqual(t, original.ID, cloned.ID, "the resend must not reuse the identity")
		assert.NoError(t, idgen.Validate(cloned.ID), "the clone must carry a canonical UUID v7")
		assert.Equal(t, original.To, cloned.To)
		assert.Equal(t, original.Template, cloned.Template)
		assert.Equal(t, original.Payload, cloned.Payload)
		assert.Equal(t, outbox.StatusPending, cloned.Status)
		assert.Zero(t, cloned.Attempts)
		assert.Equal(t, later, cloned.NextAttemptAt)

		count, err := collection.CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), count, "the clone must be a new row")
	})

	t.Run("Edge: the original row is untouched", func(t *testing.T) {
		var stored outbox.Intent
		require.NoError(t, collection.FindOne(ctx, bson.D{{Key: "_id", Value: original.ID}}).Decode(&stored))
		assert.Equal(t, original, stored)
	})

	t.Run("Error: an unknown identity is a typed not-found", func(t *testing.T) {
		_, err := repo.Clone(ctx, mustIntent(t, "ghost@example.com", now).ID, now)
		assert.ErrorIs(t, err, outbox.ErrIntentNotFound)
	})

	t.Run("Error: a non-canonical identity is rejected before any read", func(t *testing.T) {
		_, err := repo.Clone(ctx, "not-a-uuid", now)
		assert.ErrorIs(t, err, outbox.ErrInvalidIntentID)
	})
}

// TestRepository_CountSince proves the daily-cap count honours the window: only
// rows recorded at or after the instant are counted (OTP-13).
func TestRepository_CountSince(t *testing.T) {
	repo, collection := newOutboxFixture(t)
	ctx := context.Background()
	now := sampleNow()

	require.NoError(t, repo.Create(ctx, mustIntent(t, "one@example.com", now.Add(-2*time.Hour))))
	require.NoError(t, repo.Create(ctx, mustIntent(t, "two@example.com", now.Add(-time.Minute))))
	require.NoError(t, repo.Create(ctx, mustIntent(t, "three@example.com", now)))

	t.Run("Happy path: rows at or after the instant are counted", func(t *testing.T) {
		count, err := repo.CountSince(ctx, now.Add(-time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 2, count, "the boundary row and the later one are counted")
	})

	t.Run("Edge: an instant after every row counts nothing", func(t *testing.T) {
		count, err := repo.CountSince(ctx, now.Add(time.Hour))
		require.NoError(t, err)
		assert.Zero(t, count, "a future window fails open with headroom")
	})

	t.Run("Edge: an instant before every row counts them all", func(t *testing.T) {
		count, err := repo.CountSince(ctx, now.Add(-24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Happy path: the created_at index exists", func(t *testing.T) {
		cursor, err := collection.Indexes().List(ctx)
		require.NoError(t, err)

		var listed []bson.M
		require.NoError(t, cursor.All(ctx, &listed))

		names := make([]string, 0, len(listed))
		for _, index := range listed {
			if name, ok := index["name"].(string); ok {
				names = append(names, name)
			}
		}
		assert.Contains(t, names, "email_outbox_created_at", "the created_at index must exist")
	})
}
