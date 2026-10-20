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

	mdb := newUserRepositoryDatabase(t)
	return user.NewRepository(mdb, logger.NewLogger(slog.LevelError)), mdb.Collection("users")
}

// newEmailMigrationFixture exposes the operator seam over the same isolated
// database, so the migration tests can inspect persisted documents.
func newEmailMigrationFixture(t *testing.T) (user.EmailMigrationStore, *mongo.Collection) {
	t.Helper()

	mdb := newUserRepositoryDatabase(t)
	return user.NewEmailMigrationStore(mdb, logger.NewLogger(slog.LevelError)), mdb.Collection("users")
}

func newUserRepositoryDatabase(t *testing.T) *mongo.Database {
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
	return mdb
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

// rawTime reads a stored date field from a raw document. Decoding into bson.M
// keeps MongoDB dates as bson.DateTime, so assertions compare the instant, not
// the Go representation.
func rawTime(t *testing.T, raw bson.M, field string) time.Time {
	t.Helper()
	stored, ok := raw[field].(interface{ Time() time.Time })
	require.True(t, ok, "%s must decode as a date", field)
	return stored.Time()
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
		before := rawDocument(t, collection, u.ID)

		require.NoError(t, repo.MarkEmailVerified(ctx, u.ID, verifiedAt))

		raw := rawDocument(t, collection, u.ID)
		assert.Equal(t, u.ID, raw["_id"])
		assert.Equal(t, u.Email, raw["email"])
		assert.Equal(t, u.Username, raw["username"])
		assert.Equal(t, u.PasswordHash, raw["password_hash"])
		assert.Equal(t, before["created_at"], raw["created_at"], "creation time must never move")
		assert.True(t, verifiedAt.Equal(rawTime(t, raw, "email_verified_at")))
		assert.True(t, verifiedAt.Equal(rawTime(t, raw, "updated_at")))
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
		assert.True(t, verifiedAt.Equal(rawTime(t, after, "email_verified_at")), "the first verification instant wins")
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

// TestUserRepository_EmailMigration covers the operator seam: the listing reads
// every row's identifier and address, and the update touches only email and
// updated_at without creating a document (PRQ-11).
func TestUserRepository_EmailMigration(t *testing.T) {
	ctx := context.Background()
	migratedAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	rawDocument := func(t *testing.T, collection *mongo.Collection, id string) bson.M {
		t.Helper()
		var raw bson.M
		require.NoError(t, collection.FindOne(ctx, bson.M{"_id": id}).Decode(&raw))
		return raw
	}

	t.Run("Edge: an empty collection lists no rows", func(t *testing.T) {
		store, _ := newEmailMigrationFixture(t)

		identities, err := store.ListEmailIdentities(ctx)
		require.NoError(t, err)
		assert.Empty(t, identities)
	})

	t.Run("Happy path: listing returns every row's identifier and address", func(t *testing.T) {
		store, collection := newEmailMigrationFixture(t)

		first := sampleUser()
		second := sampleUser()
		second.ID = "01900000-0000-7000-8000-000000000002"
		second.Email = "Second@Example.COM"
		_, err := collection.InsertOne(ctx, first)
		require.NoError(t, err)
		_, err = collection.InsertOne(ctx, second)
		require.NoError(t, err)

		identities, err := store.ListEmailIdentities(ctx)
		require.NoError(t, err)
		assert.ElementsMatch(t, []user.EmailIdentity{
			{ID: first.ID, Email: first.Email},
			{ID: second.ID, Email: second.Email},
		}, identities)
	})

	t.Run("Happy path: the update sets only email and updated_at", func(t *testing.T) {
		store, collection := newEmailMigrationFixture(t)

		u := sampleUser()
		_, err := collection.InsertOne(ctx, u)
		require.NoError(t, err)
		before := rawDocument(t, collection, u.ID)

		const canonical = "canonical@example.com"
		require.NoError(t, store.UpdateEmail(ctx, u.ID, canonical, migratedAt))

		after := rawDocument(t, collection, u.ID)
		assert.Equal(t, canonical, after["email"])
		assert.True(t, migratedAt.Equal(rawTime(t, after, "updated_at")))
		assert.Equal(t, u.ID, after["_id"], "the identifier must never change")
		assert.Equal(t, u.Username, after["username"])
		assert.Equal(t, u.PasswordHash, after["password_hash"])
		assert.Equal(t, before["created_at"], after["created_at"])

		_, verifiedPresent := after["email_verified_at"]
		assert.False(t, verifiedPresent, "the migration must not add a verification timestamp")
	})

	t.Run("Edge: the updated row is visible to the next listing", func(t *testing.T) {
		store, collection := newEmailMigrationFixture(t)

		u := sampleUser()
		_, err := collection.InsertOne(ctx, u)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEmail(ctx, u.ID, "canonical@example.com", migratedAt))

		identities, err := store.ListEmailIdentities(ctx)
		require.NoError(t, err)
		assert.Equal(t, []user.EmailIdentity{{ID: u.ID, Email: "canonical@example.com"}}, identities)
	})

	t.Run("Error: an unknown identifier reports not found and creates nothing", func(t *testing.T) {
		store, collection := newEmailMigrationFixture(t)

		const unknownID = "01900000-0000-7000-8000-00000000dead"
		err := store.UpdateEmail(ctx, unknownID, "canonical@example.com", migratedAt)
		require.ErrorIs(t, err, user.ErrUserNotFound)

		count, err := collection.CountDocuments(ctx, bson.M{})
		require.NoError(t, err)
		assert.Zero(t, count, "a missing identifier must never become an upsert")
	})
}
