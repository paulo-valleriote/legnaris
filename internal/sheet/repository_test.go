package sheet_test

import (
	"context"
	"log/slog"
	"testing"

	"legnaris/internal/sheet"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const repositoryTestDatabase = "legnaris_sheet_repository_test"

// newRepositoryTest wires the real repository over a clean MongoDB database and
// skips when MongoDB is unavailable so unit runs stay hermetic.
func newRepositoryTest(t *testing.T) sheet.Repository {
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
	_ = mdb.Collection("sheets").Drop(context.Background())
	t.Cleanup(func() {
		_ = mdb.Collection("sheets").Drop(context.Background())
	})
	require.NoError(t, db.EnsureIndexes(context.Background(), mdb))

	return sheet.NewRepository(mdb, log)
}

func TestRepository_OwnerScopedReadsAndWrites(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()

	t.Run("Happy path: create then fetch by owner and by id", func(t *testing.T) {
		created := &sheet.Sheet{
			ID:         "01900000-0000-7000-8000-0000000000a1",
			TemplateID: "fantasy_v1",
			OwnerID:    "owner-a",
			Name:       "Owned",
			Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
		}
		require.NoError(t, repo.Create(ctx, created))

		byOwner, err := repo.FetchByOwnerID(ctx, "owner-a")
		require.NoError(t, err)
		assert.Equal(t, created.ID, byOwner.ID)
		assert.Equal(t, "owner-a", byOwner.OwnerID)

		byID, err := repo.FetchByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, byID.ID)
	})

	t.Run("Edge: an unknown owner has no sheet", func(t *testing.T) {
		found, err := repo.FetchByOwnerID(ctx, "owner-nobody")
		assert.Nil(t, found)
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)
	})

	t.Run("Error: a second sheet for the same owner maps to ErrSheetOwnerExists", func(t *testing.T) {
		duplicate := &sheet.Sheet{
			ID:         "01900000-0000-7000-8000-0000000000a2",
			TemplateID: "fantasy_v1",
			OwnerID:    "owner-a",
			Name:       "Second",
			Data:       map[string]any{},
		}
		err := repo.Create(ctx, duplicate)
		assert.ErrorIs(t, err, sheet.ErrSheetOwnerExists)
	})

	t.Run("Happy path: delete removes the owner's sheet and the second delete misses", func(t *testing.T) {
		require.NoError(t, repo.DeleteByID(ctx, "01900000-0000-7000-8000-0000000000a1"))

		_, err := repo.FetchByID(ctx, "01900000-0000-7000-8000-0000000000a1")
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)

		err = repo.DeleteByID(ctx, "01900000-0000-7000-8000-0000000000a1")
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)
	})
}
