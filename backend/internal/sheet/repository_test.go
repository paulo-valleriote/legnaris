package sheet_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

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

func TestRepository_UpdateLayout(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()

	const (
		sheetID   = "01900000-0000-7000-8000-0000000000b1"
		missingID = "01900000-0000-7000-8000-0000000000b2"
	)
	seed := func(t *testing.T) {
		t.Helper()
		_ = repo.DeleteByID(ctx, sheetID)
		require.NoError(t, repo.Create(ctx, &sheet.Sheet{
			ID:         sheetID,
			TemplateID: "fantasy_v1",
			OwnerID:    "owner-layout",
			Name:       "Layout",
			Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
		}))
	}
	layoutA := &sheet.Layout{
		SectionOrder: []string{"stats", "identity"},
		Pinned:       []string{"stats.dexterity"},
	}

	t.Run("Edge: a sheet that never had a layout reads back nil", func(t *testing.T) {
		seed(t)
		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Nil(t, stored.Layout)
	})

	t.Run("Error: an unknown sheet is ErrSheetNotFound", func(t *testing.T) {
		err := repo.UpdateLayout(ctx, missingID, layoutA)
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)
	})

	t.Run("Happy path: the layout round-trips and no other field changes", func(t *testing.T) {
		seed(t)
		before, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)

		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))

		after, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, layoutA, after.Layout)
		assert.Equal(t, before.Data, after.Data)
		assert.Equal(t, before.Name, after.Name)
		assert.Equal(t, before.OwnerID, after.OwnerID)
		assert.Equal(t, before.UpdatedAt, after.UpdatedAt, "a layout write must not touch updated_at")
	})

	t.Run("Edge: re-putting the same layout is idempotent", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, layoutA, stored.Layout)
	})

	t.Run("Edge: a second layout replaces the first instead of merging", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, &sheet.Layout{Pinned: []string{"stats.strength"}}))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.strength"}}, stored.Layout)
	})

	t.Run("Edge: a nil layout resets to none", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, nil))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Nil(t, stored.Layout)
	})

	t.Run("Edge: a data PartialUpdate keeps the layout and a layout write keeps the data", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))
		require.NoError(t, repo.PartialUpdate(ctx, sheetID, map[string]any{
			"data": map[string]any{"stats": map[string]any{"dexterity": 18.0}},
		}))
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, layoutA))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, layoutA, stored.Layout)
		assert.Equal(t, map[string]any{"stats": map[string]any{"dexterity": 18.0}}, stored.Data)
	})

	t.Run("Edge: concurrent data and layout writes both land", func(t *testing.T) {
		seed(t)

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs <- repo.PartialUpdate(ctx, sheetID, map[string]any{
				"data": map[string]any{"stats": map[string]any{"dexterity": 20.0}},
			})
		}()
		go func() {
			defer wg.Done()
			errs <- repo.UpdateLayout(ctx, sheetID, layoutA)
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, layoutA, stored.Layout)
		assert.Equal(t, map[string]any{"stats": map[string]any{"dexterity": 20.0}}, stored.Data)
	})
}
