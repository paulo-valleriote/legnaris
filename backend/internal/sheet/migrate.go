package sheet

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/internal/plan"
)

const legacySheetKind = plan.KindCharacter

const legacyEnemyKind = "monster"

func LegacyEnemyKind() string {
	return legacyEnemyKind
}

const sheetsOwnerIDUniqueIndex = "sheets_owner_id_unique"

const sheetsOwnerKindIndex = "sheets_owner_id_kind"

const (
	mongoCodeIndexNotFound     = 27
	mongoCodeNamespaceNotFound = 26
)

type MigrationReport struct {
	BackfilledSheets   int64
	RenamedEnemySheets int64
	Counters           int
}

func MigrateKinds(ctx context.Context, database *mongo.Database) (MigrationReport, error) {
	sheets := database.Collection("sheets")

	backfilled, err := sheets.UpdateMany(ctx,
		bson.D{{Key: kindField, Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: kindField, Value: string(legacySheetKind)}}}},
	)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("backfill sheet kind: %w", err)
	}

	renamed, err := sheets.UpdateMany(ctx,
		bson.D{{Key: kindField, Value: legacyEnemyKind}},
		bson.D{{Key: "$set", Value: bson.D{{Key: kindField, Value: string(plan.KindEnemy)}}}},
	)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("rename enemy sheet kind: %w", err)
	}

	groups, err := countSheetsByOwnerAndKind(ctx, sheets)
	if err != nil {
		return MigrationReport{}, err
	}
	if err := rebuildCounters(ctx, database.Collection(countsCollection), groups); err != nil {
		return MigrationReport{}, err
	}
	if err := swapOwnerIndex(ctx, sheets); err != nil {
		return MigrationReport{}, err
	}

	return MigrationReport{
		BackfilledSheets:   backfilled.ModifiedCount,
		RenamedEnemySheets: renamed.ModifiedCount,
		Counters:           len(groups),
	}, nil
}

type ownerKindGroup struct {
	OwnerID string
	Kind    plan.SheetKind
	Count   int64
}

func countSheetsByOwnerAndKind(ctx context.Context, sheets *mongo.Collection) ([]ownerKindGroup, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: ownerIDField, Value: "$owner_id"},
				{Key: kindField, Value: "$kind"},
			}},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cursor, err := sheets.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("count sheets by owner and kind: %w", err)
	}

	var rows []struct {
		Key struct {
			OwnerID string `bson:"owner_id"`
			Kind    string `bson:"kind"`
		} `bson:"_id"`
		Count int64 `bson:"n"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read sheet counts: %w", err)
	}

	groups := make([]ownerKindGroup, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, ownerKindGroup{OwnerID: row.Key.OwnerID, Kind: plan.SheetKind(row.Key.Kind), Count: row.Count})
	}
	return groups, nil
}

func rebuildCounters(ctx context.Context, counters *mongo.Collection, groups []ownerKindGroup) error {
	keep := make(bson.A, 0, len(groups))
	for _, group := range groups {
		key := countKey(group.OwnerID, group.Kind)
		doc := bson.D{
			{Key: ownerIDField, Value: group.OwnerID},
			{Key: kindField, Value: string(group.Kind)},
			{Key: countField, Value: group.Count},
		}
		if _, err := counters.ReplaceOne(ctx, key, doc, options.Replace().SetUpsert(true)); err != nil {
			return fmt.Errorf("write sheet counter: %w", err)
		}
		keep = append(keep, key)
	}

	stale := bson.D{}
	if len(keep) > 0 {
		stale = bson.D{{Key: "$nor", Value: keep}}
	}
	if _, err := counters.DeleteMany(ctx, stale); err != nil {
		return fmt.Errorf("remove stale sheet counters: %w", err)
	}
	return nil
}

func swapOwnerIndex(ctx context.Context, sheets *mongo.Collection) error {
	if err := sheets.Indexes().DropOne(ctx, sheetsOwnerIDUniqueIndex); err != nil && !isMissingIndexErr(err) {
		return fmt.Errorf("drop %s: %w", sheetsOwnerIDUniqueIndex, err)
	}
	if _, err := sheets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: ownerIDField, Value: 1}, {Key: kindField, Value: 1}},
		Options: options.Index().SetName(sheetsOwnerKindIndex),
	}); err != nil {
		return fmt.Errorf("create %s: %w", sheetsOwnerKindIndex, err)
	}
	return nil
}

func isMissingIndexErr(err error) bool {
	var cmdErr mongo.CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}
	return cmdErr.Code == mongoCodeIndexNotFound || cmdErr.Code == mongoCodeNamespaceNotFound
}
