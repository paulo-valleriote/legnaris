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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const repositoryTestDatabase = "legnaris_user_repo_test"

func newUserRepository(t *testing.T) user.Repository {
	t.Helper()

	repo, _ := newUserRepositoryFixture(t)
	return repo
}

// newUserRepositoryFixture also exposes the users collection so a test can
// assert the persisted document shape, not only the mapped struct.
func newUserRepositoryFixture(t *testing.T) (user.Repository, *mongo.Collection) {
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
	return user.NewRepository(mdb, log), mdb.Collection("users")
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

// TestUserRepository_MarkEmailVerified covers the store-only verification
// operation: one conditional write, first timestamp wins, unknown account is
// not created (PRQ-13, PRQ-14).
func TestUserRepository_MarkEmailVerified(t *testing.T) {
	ctx := context.Background()

	verifiedAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	rawDocument := func(t *testing.T, collection *mongo.Collection, id string) bson.M {
		t.Helper()
		var raw bson.M
		require.NoError(t, collection.FindOne(ctx, bson.M{"_id": id}).Decode(&raw))
		return raw
	}

	t.Run("Error: an unknown account is reported as not found and never created", func(t *testing.T) {
		repo, collection := newUserRepositoryFixture(t)

		const unknownID = "01900000-0000-7000-8000-00000000dead"
		err := repo.MarkEmailVerified(ctx, unknownID, verifiedAt)
		require.ErrorIs(t, err, user.ErrUserNotFound)

		count, err := collection.CountDocuments(ctx, bson.M{})
		require.NoError(t, err)
		assert.Zero(t, count, "the operation must never create a document")
	})

	t.Run("Edge: a freshly created account carries no verification timestamp", func(t *testing.T) {
		repo, collection := newUserRepositoryFixture(t)
		u := sampleUser()
		require.NoError(t, repo.Create(ctx, u))

		raw := rawDocument(t, collection, u.ID)
		_, present := raw["email_verified_at"]
		assert.False(t, present, "registration must not add the field")

		fetched, err := repo.FetchByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Nil(t, fetched.EmailVerifiedAt)
	})

	t.Run("Happy path: the first call sets only email_verified_at and updated_at", func(t *testing.T) {
		repo, collection := newUserRepositoryFixture(t)
		u := sampleUser()
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID, verifiedAt))

		raw := rawDocument(t, collection, u.ID)
		assert.Equal(t, u.ID, raw["_id"])
		assert.Equal(t, u.Email, raw["email"])
		assert.Equal(t, u.Username, raw["username"])
		assert.Equal(t, u.PasswordHash, raw["password_hash"])
		assert.Equal(t, u.CreatedAt, raw["created_at"], "creation time must never move")
		assert.Equal(t, verifiedAt, raw["email_verified_at"])
		assert.Equal(t, verifiedAt, raw["updated_at"])
	})

	t.Run("Edge: a second call is a no-op that preserves the first timestamp and updated_at", func(t *testing.T) {
		repo, collection := newUserRepositoryFixture(t)
		u := sampleUser()
		require.NoError(t, repo.Create(ctx, u))
		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID, verifiedAt))

		before := rawDocument(t, collection, u.ID)

		later := verifiedAt.Add(24 * time.Hour)
		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID, later))

		after := rawDocument(t, collection, u.ID)
		assert.Equal(t, verifiedAt, after["email_verified_at"], "the first verification instant wins")
		assert.Equal(t, before["updated_at"], after["updated_at"], "a no-op must not touch updated_at")
		assert.Equal(t, before, after)
	})

	t.Run("Edge: concurrent calls keep exactly one timestamp", func(t *testing.T) {
		repo, collection := newUserRepositoryFixture(t)
		u := sampleUser()
		require.NoError(t, repo.Create(ctx, u))

		instants := []time.Time{verifiedAt, verifiedAt.Add(time.Second)}
		errs := make(chan error, len(instants))
		start := make(chan struct{})

		for _, at := range instants {
			go func(at time.Time) {
				<-start
				errs <- repo.MarkEmailVerified(ctx, u.ID, at)
			}(at)
		}
		close(start)

		for range instants {
			require.NoError(t, <-errs)
		}

		raw := rawDocument(t, collection, u.ID)
		stored, ok := raw["email_verified_at"].(interface{ Time() time.Time })
		require.True(t, ok, "the stored timestamp must decode as a time value")

		var matches int
		for _, at := range instants {
			if stored.Time().Equal(at) {
				matches++
			}
		}
		assert.Equal(t, 1, matches, "the stored value must be one of the submitted instants")
		assert.Equal(t, raw["email_verified_at"], raw["updated_at"])
	})
}
