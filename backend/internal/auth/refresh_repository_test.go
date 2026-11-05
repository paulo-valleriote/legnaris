package auth_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refreshRepositoryTestDatabase = "legnaris_auth_repo_test"

func newRefreshTokenRepository(t *testing.T) auth.RefreshTokenRepository {
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

	mdb := database.Client.Database(refreshRepositoryTestDatabase)
	_ = mdb.Collection("refresh_tokens").Drop(context.Background())
	t.Cleanup(func() {
		_ = mdb.Collection("refresh_tokens").Drop(context.Background())
	})

	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return auth.NewRefreshTokenRepository(mdb)
}

func sampleToken(id, hash, family string) *auth.RefreshToken {
	return &auth.RefreshToken{
		ID:        id,
		TokenHash: hash,
		UserID:    "01900000-0000-7000-8000-0000000000aa",
		FamilyID:  family,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
}

func TestRefreshTokenRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: create then fetch by hash", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))

		fetched, err := repo.FetchByHash(ctx, "hash-1")
		require.NoError(t, err)
		assert.Equal(t, "tok-1", fetched.ID)
		assert.False(t, fetched.Revoked)
	})

	t.Run("Error: unknown hash reports not found", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)

		_, err := repo.FetchByHash(ctx, "missing")
		require.ErrorIs(t, err, auth.ErrRefreshTokenNotFound)
	})

	t.Run("Happy path: RevokeByID claims once and records the replacement", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))

		claimed, err := repo.RevokeByID(ctx, "tok-1", "tok-2")
		require.NoError(t, err)
		assert.True(t, claimed)

		revoked, err := repo.FetchByHash(ctx, "hash-1")
		require.NoError(t, err)
		assert.True(t, revoked.Revoked)
		assert.Equal(t, "tok-2", revoked.ReplacedBy)
	})

	t.Run("Edge: a second RevokeByID for the same token returns false", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))

		first, err := repo.RevokeByID(ctx, "tok-1", "tok-2")
		require.NoError(t, err)
		require.True(t, first)

		second, err := repo.RevokeByID(ctx, "tok-1", "tok-3")
		require.NoError(t, err)
		assert.False(t, second, "an already revoked token must not be claimable again")
	})

	t.Run("Edge: FetchByHash returns an already-revoked token", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))

		claimed, err := repo.RevokeByID(ctx, "tok-1", "tok-2")
		require.NoError(t, err)
		require.True(t, claimed)

		fetched, err := repo.FetchByHash(ctx, "hash-1")
		require.NoError(t, err)
		assert.True(t, fetched.Revoked, "logout must see rotated tokens to revoke their family")
	})

	t.Run("Happy path: RevokeFamily revokes every live token in the family", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))
		require.NoError(t, repo.Create(ctx, sampleToken("tok-2", "hash-2", "fam-1")))
		require.NoError(t, repo.Create(ctx, sampleToken("tok-3", "hash-3", "fam-2")))

		require.NoError(t, repo.RevokeFamily(ctx, "fam-1"))

		first, err := repo.FetchByHash(ctx, "hash-1")
		require.NoError(t, err)
		assert.True(t, first.Revoked)

		second, err := repo.FetchByHash(ctx, "hash-2")
		require.NoError(t, err)
		assert.True(t, second.Revoked)

		other, err := repo.FetchByHash(ctx, "hash-3")
		require.NoError(t, err)
		assert.False(t, other.Revoked, "another family must be untouched")
	})
}

func TestRefreshTokenRepository_RevokeAllForUser(t *testing.T) {
	ctx := context.Background()
	const otherUser = "01900000-0000-7000-8000-0000000000bb"

	t.Run("Happy path: revokes every live token of the account across families", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))
		require.NoError(t, repo.Create(ctx, sampleToken("tok-2", "hash-2", "fam-2")))

		require.NoError(t, repo.RevokeAllForUser(ctx, "01900000-0000-7000-8000-0000000000aa"))

		for _, hash := range []string{"hash-1", "hash-2"} {
			fetched, err := repo.FetchByHash(ctx, hash)
			require.NoError(t, err)
			assert.True(t, fetched.Revoked, "token %s must be revoked", hash)
		}
	})

	t.Run("Edge: another account's tokens are untouched", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		foreign := sampleToken("tok-3", "hash-3", "fam-3")
		foreign.UserID = otherUser
		require.NoError(t, repo.Create(ctx, foreign))

		require.NoError(t, repo.RevokeAllForUser(ctx, "01900000-0000-7000-8000-0000000000aa"))

		fetched, err := repo.FetchByHash(ctx, "hash-3")
		require.NoError(t, err)
		assert.False(t, fetched.Revoked)
	})

	t.Run("Edge: an account with no tokens, and a repeated call, are no-ops", func(t *testing.T) {
		repo := newRefreshTokenRepository(t)
		require.NoError(t, repo.RevokeAllForUser(ctx, "01900000-0000-7000-8000-0000000000cc"))

		require.NoError(t, repo.Create(ctx, sampleToken("tok-1", "hash-1", "fam-1")))
		require.NoError(t, repo.RevokeAllForUser(ctx, "01900000-0000-7000-8000-0000000000aa"))
		require.NoError(t, repo.RevokeAllForUser(ctx, "01900000-0000-7000-8000-0000000000aa"))

		fetched, err := repo.FetchByHash(ctx, "hash-1")
		require.NoError(t, err)
		assert.True(t, fetched.Revoked)
	})
}
