package sheet_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func seedLegacySheet(t *testing.T, mdb *mongo.Database, id, owner string) {
	t.Helper()
	_, err := mdb.Collection("sheets").InsertOne(context.Background(), bson.D{
		{Key: "_id", Value: id},
		{Key: "template_id", Value: "fantasy_v1"},
		{Key: "owner_id", Value: owner},
		{Key: "name", Value: "Legacy"},
		{Key: "data", Value: bson.D{}},
	})
	require.NoError(t, err)
}

func seedSheetKind(t *testing.T, mdb *mongo.Database, id, owner, kind string) {
	t.Helper()
	_, err := mdb.Collection("sheets").InsertOne(context.Background(), bson.D{
		{Key: "_id", Value: id},
		{Key: "template_id", Value: "fantasy_v1"},
		{Key: "owner_id", Value: owner},
		{Key: "kind", Value: kind},
		{Key: "name", Value: "Legacy"},
		{Key: "data", Value: bson.D{}},
	})
	require.NoError(t, err)
}

func countCounterDocs(t *testing.T, mdb *mongo.Database, owner, kind string) int64 {
	t.Helper()
	count, err := mdb.Collection("sheet_counts").CountDocuments(context.Background(), bson.D{
		{Key: "owner_id", Value: owner},
		{Key: "kind", Value: kind},
	})
	require.NoError(t, err)
	return count
}

func countSheetDocs(t *testing.T, mdb *mongo.Database, kind string) int64 {
	t.Helper()
	count, err := mdb.Collection("sheets").CountDocuments(context.Background(), bson.D{{Key: "kind", Value: kind}})
	require.NoError(t, err)
	return count
}

func restoreLegacyOwnerIndex(t *testing.T, mdb *mongo.Database) {
	t.Helper()
	_, err := mdb.Collection("sheets").Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("sheets_owner_id_unique"),
	})
	require.NoError(t, err)
}

func hasIndex(t *testing.T, mdb *mongo.Database, name string) bool {
	t.Helper()
	specs, err := mdb.Collection("sheets").Indexes().ListSpecifications(context.Background())
	require.NoError(t, err)
	for _, spec := range specs {
		if spec.Name == name {
			return true
		}
	}
	return false
}

func TestMigrateKinds(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: legacy sheets become characters and counters are built", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		restoreLegacyOwnerIndex(t, mdb)
		seedLegacySheet(t, mdb, "01900000-0000-7000-8000-0000000000b1", "owner-m1")
		seedLegacySheet(t, mdb, "01900000-0000-7000-8000-0000000000b2", "owner-m2")

		report, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)
		assert.Equal(t, int64(2), report.BackfilledSheets)
		assert.Equal(t, 2, report.Counters)

		var stored sheet.Sheet
		require.NoError(t, mdb.Collection("sheets").FindOne(ctx, bson.D{{Key: "_id", Value: "01900000-0000-7000-8000-0000000000b1"}}).Decode(&stored))
		assert.Equal(t, plan.KindCharacter, stored.Kind)

		repo := sheet.NewRepository(mdb, nil)
		count, err := repo.CountOf(ctx, "owner-m1", plan.KindCharacter)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Happy path: legacy monster sheets become enemies and their counters follow", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		seedSheetKind(t, mdb, "01900000-0000-7000-8000-0000000000c1", "owner-e1", sheet.LegacyEnemyKind())
		seedSheetKind(t, mdb, "01900000-0000-7000-8000-0000000000c2", "owner-e1", sheet.LegacyEnemyKind())
		seedSheetKind(t, mdb, "01900000-0000-7000-8000-0000000000c3", "owner-e1", string(plan.KindNPC))

		report, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)
		assert.Equal(t, int64(2), report.RenamedEnemySheets)
		assert.Equal(t, int64(0), report.BackfilledSheets)

		assert.Zero(t, countSheetDocs(t, mdb, sheet.LegacyEnemyKind()), "no sheet keeps the legacy kind")
		assert.Equal(t, int64(2), countSheetDocs(t, mdb, string(plan.KindEnemy)))

		repo := sheet.NewRepository(mdb, nil)
		count, err := repo.CountOf(ctx, "owner-e1", plan.KindEnemy)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Zero(t, countCounterDocs(t, mdb, "owner-e1", sheet.LegacyEnemyKind()), "the legacy counter is gone")
		assert.Equal(t, int64(1), countCounterDocs(t, mdb, "owner-e1", string(plan.KindNPC)))
	})

	t.Run("Edge: a second run renames nothing and keeps the counters", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		seedSheetKind(t, mdb, "01900000-0000-7000-8000-0000000000c4", "owner-e2", sheet.LegacyEnemyKind())

		first, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)
		second, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)

		assert.Equal(t, int64(1), first.RenamedEnemySheets)
		assert.Equal(t, int64(0), second.RenamedEnemySheets)
		assert.Equal(t, first.Counters, second.Counters)

		repo := sheet.NewRepository(mdb, nil)
		count, err := repo.CountOf(ctx, "owner-e2", plan.KindEnemy)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Happy path: the unique owner index is replaced by the owner and kind index", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		restoreLegacyOwnerIndex(t, mdb)
		require.True(t, hasIndex(t, mdb, "sheets_owner_id_unique"))

		_, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)

		assert.False(t, hasIndex(t, mdb, "sheets_owner_id_unique"), "the one-sheet-per-user index is gone")
		assert.True(t, hasIndex(t, mdb, "sheets_owner_id_kind"), "the per-kind index replaces it")
	})

	t.Run("Edge: a second run changes nothing and reports nothing new", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		restoreLegacyOwnerIndex(t, mdb)
		seedLegacySheet(t, mdb, "01900000-0000-7000-8000-0000000000b3", "owner-m3")

		first, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)
		second, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)

		assert.Equal(t, int64(1), first.BackfilledSheets)
		assert.Equal(t, int64(0), second.BackfilledSheets)
		assert.Equal(t, first.Counters, second.Counters)
	})

	t.Run("Edge: an empty database migrates to an empty report", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)

		report, err := sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)
		assert.Equal(t, sheet.MigrationReport{}, report)
	})

	t.Run("Edge: a stale counter with no Sheet behind it is removed", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		_, err := mdb.Collection("sheet_counts").InsertOne(ctx, bson.D{
			{Key: "owner_id", Value: "owner-ghost"},
			{Key: "kind", Value: "npc"},
			{Key: "count", Value: 2},
		})
		require.NoError(t, err)

		_, err = sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)

		remaining, err := mdb.Collection("sheet_counts").CountDocuments(ctx, bson.D{{Key: "owner_id", Value: "owner-ghost"}})
		require.NoError(t, err)
		assert.Zero(t, remaining)
	})

	t.Run("Edge: a counter that drifted is rebuilt from the Sheets", func(t *testing.T) {
		_, mdb := newRepositoryTestWithDB(t)
		restoreLegacyOwnerIndex(t, mdb)
		seedLegacySheet(t, mdb, "01900000-0000-7000-8000-0000000000b4", "owner-drift")
		_, err := mdb.Collection("sheet_counts").InsertOne(ctx, bson.D{
			{Key: "owner_id", Value: "owner-drift"},
			{Key: "kind", Value: "character"},
			{Key: "count", Value: 7},
		})
		require.NoError(t, err)

		_, err = sheet.MigrateKinds(ctx, mdb)
		require.NoError(t, err)

		repo := sheet.NewRepository(mdb, nil)
		count, err := repo.CountOf(ctx, "owner-drift", plan.KindCharacter)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "the Sheets are the source of truth, not the stale counter")
	})
}
