package user_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/internal/user"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const repositoryTestDatabase = "legnaris_user_repo_test"

func newUserRepository(t *testing.T) user.Repository {
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
	_ = mdb.Collection("users").Drop(context.Background())
	t.Cleanup(func() {
		_ = mdb.Collection("users").Drop(context.Background())
	})

	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))
	return user.NewRepository(mdb, log)
}

func sampleUser() *user.User {
	return &user.User{
		ID:           "01900000-0000-7000-8000-000000000001",
		Email:        "player@example.com",
		Username:     "player",
		PasswordHash: "hash",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestUserRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: create then fetch by id and email", func(t *testing.T) {
		repo := newUserRepository(t)
		u := sampleUser()
		require.NoError(t, repo.Create(ctx, u))

		byID, err := repo.FetchByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, u.Email, byID.Email)
		assert.Equal(t, u.PasswordHash, byID.PasswordHash)

		byEmail, err := repo.FetchByEmail(ctx, u.Email)
		require.NoError(t, err)
		assert.Equal(t, u.ID, byEmail.ID)
	})

	t.Run("Error: duplicate email is rejected by the unique index", func(t *testing.T) {
		repo := newUserRepository(t)
		require.NoError(t, repo.Create(ctx, sampleUser()))

		dup := sampleUser()
		dup.ID = "01900000-0000-7000-8000-000000000002"

		err := repo.Create(ctx, dup)
		require.ErrorIs(t, err, user.ErrEmailAlreadyExists)
	})

	t.Run("Error: fetch by unknown email reports not found", func(t *testing.T) {
		repo := newUserRepository(t)

		_, err := repo.FetchByEmail(ctx, "nobody@example.com")
		require.ErrorIs(t, err, user.ErrUserNotFound)
	})

	t.Run("Error: fetch by unknown id reports not found", func(t *testing.T) {
		repo := newUserRepository(t)

		_, err := repo.FetchByID(ctx, "01900000-0000-7000-8000-00000000dead")
		require.ErrorIs(t, err, user.ErrUserNotFound)
	})
}
