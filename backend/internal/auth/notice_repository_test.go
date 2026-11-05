package auth_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const noticeTestDatabase = "legnaris_auth_notices_test"

func newNoticeStore(t *testing.T) auth.NoticeRepository {
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

	mdb := database.Client.Database(noticeTestDatabase)
	collection := mdb.Collection(auth.EmailNoticesCollection)
	_ = collection.Drop(context.Background())
	t.Cleanup(func() { _ = collection.Drop(context.Background()) })

	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return auth.NewNoticeRepository(mdb)
}

// TestNoticeRepository_Claim covers the pacing of code-free notices: one claim
// per purpose and address per window, even under concurrency.
func TestNoticeRepository_Claim(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	window := time.Hour
	const address = "player@example.com"

	t.Run("Happy path: the first claim wins", func(t *testing.T) {
		store := newNoticeStore(t)

		claimed, err := store.Claim(ctx, auth.PurposeAccountExists, address, now.Add(-window), now)
		require.NoError(t, err)
		assert.True(t, claimed)
	})

	t.Run("Edge: a second claim inside the window is throttled", func(t *testing.T) {
		store := newNoticeStore(t)
		_, err := store.Claim(ctx, auth.PurposeAccountExists, address, now.Add(-window), now)
		require.NoError(t, err)

		at := now.Add(window - time.Second)
		claimed, err := store.Claim(ctx, auth.PurposeAccountExists, address, at.Add(-window), at)
		require.NoError(t, err)
		assert.False(t, claimed)
	})

	t.Run("Edge: a claim after the window succeeds again", func(t *testing.T) {
		store := newNoticeStore(t)
		_, err := store.Claim(ctx, auth.PurposeAccountExists, address, now.Add(-window), now)
		require.NoError(t, err)

		at := now.Add(window + time.Nanosecond)
		claimed, err := store.Claim(ctx, auth.PurposeAccountExists, address, at.Add(-window), at)
		require.NoError(t, err)
		assert.True(t, claimed)
	})

	t.Run("Edge: purposes and addresses are paced independently", func(t *testing.T) {
		store := newNoticeStore(t)
		_, err := store.Claim(ctx, auth.PurposeAccountExists, address, now.Add(-window), now)
		require.NoError(t, err)

		otherPurpose, err := store.Claim(ctx, auth.PurposeUsernameTaken, address, now.Add(-window), now)
		require.NoError(t, err)
		assert.True(t, otherPurpose)

		otherAddress, err := store.Claim(ctx, auth.PurposeAccountExists, "other@example.com", now.Add(-window), now)
		require.NoError(t, err)
		assert.True(t, otherAddress)
	})

	t.Run("Edge: concurrent claims yield exactly one winner", func(t *testing.T) {
		store := newNoticeStore(t)
		const callers = 8

		results := make([]bool, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				claimed, err := store.Claim(ctx, auth.PurposeAccountExists, address, now.Add(-window), now)
				assert.NoError(t, err)
				results[i] = claimed
			}()
		}
		wg.Wait()

		winners := 0
		for _, claimed := range results {
			if claimed {
				winners++
			}
		}
		assert.Equal(t, 1, winners)
	})
}
