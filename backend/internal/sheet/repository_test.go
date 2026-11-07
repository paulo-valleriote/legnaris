package sheet_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const repositoryTestDatabase = "legnaris_sheet_repository_test"

// newRepositoryTest wires the real repository over a clean MongoDB database and
// skips when MongoDB is unavailable so unit runs stay hermetic.
func newRepositoryTest(t *testing.T) sheet.Repository {
	t.Helper()

	repo, _ := newRepositoryTestWithDB(t)
	return repo
}

// newRepositoryTestWithDB also returns the database handle, for the tests that
// seed documents the repository itself would never write, such as a sheet that
// predates the version field.
func newRepositoryTestWithDB(t *testing.T) (sheet.Repository, *mongo.Database) {
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

	return sheet.NewRepository(mdb, log), mdb
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
		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 0, map[string]any{
			"data": map[string]any{"stats": map[string]any{"dexterity": 18.0}},
		})))
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
			errs <- putErr(repo.PartialUpdate(ctx, sheetID, 0, map[string]any{
				"data": map[string]any{"stats": map[string]any{"dexterity": 20.0}},
			}))
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

func TestRepository_UpdateDice(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()

	const (
		sheetID   = "01900000-0000-7000-8000-0000000000c1"
		missingID = "01900000-0000-7000-8000-0000000000c2"
	)
	seed := func(t *testing.T) {
		t.Helper()
		_ = repo.DeleteByID(ctx, sheetID)
		require.NoError(t, repo.Create(ctx, &sheet.Sheet{
			ID:         sheetID,
			TemplateID: "fantasy_v1",
			OwnerID:    "owner-dice",
			Name:       "Dice",
			Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
			Layout:     &sheet.Layout{Pinned: []string{"stats.dexterity"}},
		}))
	}
	diceA := &sheet.Dice{
		Section: map[string]*string{"stats": text("1d20 + @"), "config": nil},
		Row:     map[string]*string{"stats.dexterity": nil, "skills.stealth": text("1d20 + @.total")},
	}

	t.Run("Edge: a sheet that never had dice reads back nil", func(t *testing.T) {
		seed(t)
		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Nil(t, stored.Dice)
	})

	t.Run("Error: an unknown sheet is ErrSheetNotFound", func(t *testing.T) {
		err := repo.UpdateDice(ctx, missingID, diceA)
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)
	})

	t.Run("Happy path: overrides round-trip exactly, nulls included, and no other field changes", func(t *testing.T) {
		seed(t)
		before, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)

		require.NoError(t, repo.UpdateDice(ctx, sheetID, diceA))

		after, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, diceA, after.Dice)
		value, present := after.Dice.Row["stats.dexterity"]
		assert.True(t, present, "a null override must survive as a present key")
		assert.Nil(t, value)
		assert.Equal(t, before.Data, after.Data)
		assert.Equal(t, before.Layout, after.Layout)
		assert.Equal(t, before.Version, after.Version)
		assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
	})

	t.Run("Edge: re-putting the same block is idempotent", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateDice(ctx, sheetID, diceA))
		require.NoError(t, repo.UpdateDice(ctx, sheetID, diceA))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, diceA, stored.Dice)
	})

	t.Run("Edge: a second block replaces the first instead of merging", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateDice(ctx, sheetID, diceA))
		only := &sheet.Dice{Row: map[string]*string{"stats.strength": text("2d6")}}
		require.NoError(t, repo.UpdateDice(ctx, sheetID, only))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, only, stored.Dice)
	})

	t.Run("Edge: a nil block clears everything", func(t *testing.T) {
		seed(t)
		require.NoError(t, repo.UpdateDice(ctx, sheetID, diceA))
		require.NoError(t, repo.UpdateDice(ctx, sheetID, nil))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Nil(t, stored.Dice)
	})

	t.Run("Edge: concurrent data, layout and dice writes all land", func(t *testing.T) {
		seed(t)

		var wg sync.WaitGroup
		errs := make(chan error, 3)
		wg.Add(3)
		go func() {
			defer wg.Done()
			errs <- putErr(repo.PartialUpdate(ctx, sheetID, 0, map[string]any{
				"data": map[string]any{"stats": map[string]any{"dexterity": 20.0}},
			}))
		}()
		go func() {
			defer wg.Done()
			errs <- repo.UpdateLayout(ctx, sheetID, &sheet.Layout{Pinned: []string{"stats.strength"}})
		}()
		go func() {
			defer wg.Done()
			errs <- repo.UpdateDice(ctx, sheetID, diceA)
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, diceA, stored.Dice)
		assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.strength"}}, stored.Layout)
		assert.Equal(t, map[string]any{"stats": map[string]any{"dexterity": 20.0}}, stored.Data)
	})

	t.Run("Edge: concurrent dice writes leave one whole valid block", func(t *testing.T) {
		seed(t)
		blockB := &sheet.Dice{Row: map[string]*string{"stats.strength": text("2d6")}}

		var wg sync.WaitGroup
		for _, block := range []*sheet.Dice{diceA, blockB, diceA, blockB} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NoError(t, repo.UpdateDice(ctx, sheetID, block))
			}()
		}
		wg.Wait()

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Contains(t, []*sheet.Dice{diceA, blockB}, stored.Dice)
	})
}

// putErr adapts PartialUpdate's (version, error) result for call sites that only
// check the error.
func putErr(_ int64, err error) error {
	return err
}

func TestRepository_PartialUpdateVersion(t *testing.T) {
	const (
		sheetID     = "01900000-0000-7000-8000-0000000000c1"
		legacyID    = "01900000-0000-7000-8000-0000000000c2"
		missingID   = "01900000-0000-7000-8000-0000000000c3"
		raceSheetID = "01900000-0000-7000-8000-0000000000c4"
	)
	repo, mdb := newRepositoryTestWithDB(t)
	ctx := context.Background()

	seed := func(t *testing.T, id string) {
		t.Helper()
		_ = repo.DeleteByID(ctx, id)
		require.NoError(t, repo.Create(ctx, &sheet.Sheet{
			ID:         id,
			TemplateID: "fantasy_v1",
			OwnerID:    "owner-version-" + id,
			Name:       "Version",
			Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
		}))
	}
	dexterityUpdate := func(value float64) map[string]any {
		return map[string]any{"data": map[string]any{"stats": map[string]any{"dexterity": value}}}
	}

	t.Run("Happy path: a created sheet starts at version 0 and a matching write bumps it to 1", func(t *testing.T) {
		seed(t, sheetID)
		before, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		require.Equal(t, int64(0), before.Version)

		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(18))))

		after, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), after.Version)
		assert.Equal(t, map[string]any{"stats": map[string]any{"dexterity": 18.0}}, after.Data)
	})

	t.Run("Edge: a stale expected version is ErrVersionConflict and writes nothing", func(t *testing.T) {
		seed(t, sheetID)
		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(18))))

		err := putErr(repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(20)))
		require.ErrorIs(t, err, sheet.ErrVersionConflict)

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stored.Version)
		assert.Equal(t, map[string]any{"stats": map[string]any{"dexterity": 18.0}}, stored.Data)
	})

	t.Run("Edge: replaying an already-applied write with its old version conflicts", func(t *testing.T) {
		seed(t, sheetID)
		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(18))))
		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 1, dexterityUpdate(19))))

		err := putErr(repo.PartialUpdate(ctx, sheetID, 1, dexterityUpdate(18)))
		assert.ErrorIs(t, err, sheet.ErrVersionConflict)
	})

	t.Run("Edge: a future expected version conflicts", func(t *testing.T) {
		seed(t, sheetID)

		err := putErr(repo.PartialUpdate(ctx, sheetID, 5, dexterityUpdate(18)))
		assert.ErrorIs(t, err, sheet.ErrVersionConflict)
	})

	t.Run("Edge: an unknown sheet reports ErrSheetNotFound, not a retryable conflict", func(t *testing.T) {
		_, err := repo.PartialUpdate(ctx, missingID, 0, dexterityUpdate(18))
		assert.ErrorIs(t, err, sheet.ErrSheetNotFound)
		assert.NotErrorIs(t, err, sheet.ErrVersionConflict)
	})

	t.Run("Happy path: PartialUpdate returns the version it stored", func(t *testing.T) {
		seed(t, sheetID)

		version, err := repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(18))
		require.NoError(t, err)
		assert.Equal(t, int64(1), version)

		version, err = repo.PartialUpdate(ctx, sheetID, 1, dexterityUpdate(19))
		require.NoError(t, err)
		assert.Equal(t, int64(2), version)
	})

	t.Run("Error: a failed CAS returns version 0 with the conflict error", func(t *testing.T) {
		seed(t, sheetID)

		version, err := repo.PartialUpdate(ctx, sheetID, 7, dexterityUpdate(18))
		require.ErrorIs(t, err, sheet.ErrVersionConflict)
		assert.Equal(t, int64(0), version)
	})

	t.Run("Edge: a legacy sheet without a version field matches expected 0 and becomes version 1", func(t *testing.T) {
		seed(t, legacyID)
		_, err := mdb.Collection("sheets").UpdateOne(ctx,
			bson.D{{Key: "_id", Value: legacyID}},
			bson.M{"$unset": bson.M{"version": ""}})
		require.NoError(t, err)

		require.NoError(t, putErr(repo.PartialUpdate(ctx, legacyID, 0, dexterityUpdate(18))))

		stored, err := repo.FetchByID(ctx, legacyID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stored.Version)
	})

	t.Run("Edge: a layout write leaves the version alone, so a PATCH expecting it still lands", func(t *testing.T) {
		seed(t, sheetID)
		require.NoError(t, repo.UpdateLayout(ctx, sheetID, &sheet.Layout{Pinned: []string{"stats.dexterity"}}))

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		require.Equal(t, int64(0), stored.Version)

		require.NoError(t, putErr(repo.PartialUpdate(ctx, sheetID, 0, dexterityUpdate(18))))
	})

	t.Run("Edge: concurrent writers with the same expected version let exactly one win", func(t *testing.T) {
		seed(t, raceSheetID)

		const writers = 8
		var wg sync.WaitGroup
		results := make(chan error, writers)
		for range writers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- putErr(repo.PartialUpdate(ctx, raceSheetID, 0, dexterityUpdate(18)))
			}()
		}
		wg.Wait()
		close(results)

		var wins, conflicts int
		for err := range results {
			switch {
			case err == nil:
				wins++
			case errors.Is(err, sheet.ErrVersionConflict):
				conflicts++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		assert.Equal(t, 1, wins)
		assert.Equal(t, writers-1, conflicts)

		stored, err := repo.FetchByID(ctx, raceSheetID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stored.Version)
	})
}
