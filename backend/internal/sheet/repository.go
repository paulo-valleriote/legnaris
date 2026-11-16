package sheet

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/logger"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	FetchByOwnerID(ctx context.Context, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, id string, expectedVersion int64, dbUpdates map[string]any) (int64, error)
	UpdateLayout(ctx context.Context, id string, layout *Layout) error
	DeleteByID(ctx context.Context, id string) error
	ReserveSlot(ctx context.Context, ownerID string, kind plan.SheetKind, limit int) error
	ReleaseSlot(ctx context.Context, ownerID string, kind plan.SheetKind) error
	CountOf(ctx context.Context, ownerID string, kind plan.SheetKind) (int, error)
}

type RepositoryImpl struct {
	collection *mongo.Collection
	counts     *mongo.Collection
	logger     logger.StructuredLogger
}

const countsCollection = "sheet_counts"

const countField = "count"

const (
	ownerIDField = "owner_id"
	kindField    = "kind"
	incOperator  = "$inc"
)

const detailKindKey = "kind"

const idField = "_id"

const layoutField = "layout"

const setOperator = "$set"

const diceField = "dice"

const versionField = "version"

var ErrSheetNotFound = errors.New("sheet not found")

var ErrVersionConflict = errors.New("sheet version conflict")

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection("sheets"),
		counts:     db.Collection(countsCollection, options.Collection().SetReadPreference(readpref.Primary())),
		logger:     log,
	}
}

func (r *RepositoryImpl) ReserveSlot(ctx context.Context, ownerID string, kind plan.SheetKind, limit int) error {
	key := countKey(ownerID, kind)
	open := bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: countField, Value: 0}}}}
	if _, err := r.counts.UpdateOne(ctx, key, open, options.UpdateOne().SetUpsert(true)); err != nil {
		return fmt.Errorf("error opening sheet counter: %w", err)
	}

	below := append(key, bson.E{Key: countField, Value: bson.D{{Key: "$lt", Value: limit}}})
	result, err := r.counts.UpdateOne(ctx, below, bson.D{{Key: incOperator, Value: bson.D{{Key: countField, Value: 1}}}})
	if err != nil {
		return fmt.Errorf("error reserving sheet slot: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrSheetLimitReached
	}
	return nil
}

func (r *RepositoryImpl) ReleaseSlot(ctx context.Context, ownerID string, kind plan.SheetKind) error {
	positive := append(countKey(ownerID, kind), bson.E{Key: countField, Value: bson.D{{Key: "$gt", Value: 0}}})
	if _, err := r.counts.UpdateOne(ctx, positive, bson.D{{Key: incOperator, Value: bson.D{{Key: countField, Value: -1}}}}); err != nil {
		return fmt.Errorf("error releasing sheet slot: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) CountOf(ctx context.Context, ownerID string, kind plan.SheetKind) (int, error) {
	var doc struct {
		Count int `bson:"count"`
	}
	err := r.counts.FindOne(ctx, countKey(ownerID, kind)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("error counting sheets: %w", err)
	}
	return doc.Count, nil
}

func countKey(ownerID string, kind plan.SheetKind) bson.D {
	return bson.D{
		{Key: ownerIDField, Value: ownerID},
		{Key: kindField, Value: string(kind)},
	}
}

func (r *RepositoryImpl) FetchByID(ctx context.Context, id string) (*Sheet, error) {
	var result Sheet

	filter := bson.D{{Key: idField, Value: id}}
	opts := options.FindOne()
	err := r.collection.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrSheetNotFound
		}
		return nil, err
	}

	return &result, nil
}

func (r *RepositoryImpl) FetchByOwnerID(ctx context.Context, ownerID string) (*Sheet, error) {
	var result Sheet

	filter := bson.D{{Key: ownerIDField, Value: ownerID}}
	if err := r.collection.FindOne(ctx, filter).Decode(&result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrSheetNotFound
		}
		return nil, err
	}

	return &result, nil
}

func (r *RepositoryImpl) Create(ctx context.Context, sheet *Sheet) error {
	result, err := r.collection.InsertOne(ctx, sheet)
	if err != nil {
		return fmt.Errorf("error creating sheet: %w", err)
	}
	r.logger.Info(ctx, "sheet created", logger.Attribute{Key: "sheet_id", Value: result.InsertedID.(string)})
	return nil
}

func (r *RepositoryImpl) PartialUpdate(ctx context.Context, id string, expectedVersion int64, dbUpdates map[string]any) (int64, error) {
	filter := bson.D{
		{Key: idField, Value: id},
		{Key: versionField, Value: versionMatcher(expectedVersion)},
	}
	update := bson.M{
		setOperator: dbUpdates,
		"$inc":      bson.M{versionField: 1},
	}
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetProjection(bson.M{versionField: 1})

	var stored struct {
		Version int64 `bson:"version"`
	}
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&stored)
	switch {
	case err == nil:
		return stored.Version, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return 0, r.unmatchedUpdateErr(ctx, id)
	default:
		return 0, fmt.Errorf("error updating sheet: %w", err)
	}
}

func (r *RepositoryImpl) unmatchedUpdateErr(ctx context.Context, id string) error {
	count, err := r.collection.CountDocuments(ctx, bson.D{{Key: idField, Value: id}}, options.Count().SetLimit(1))
	if err != nil {
		return fmt.Errorf("error checking sheet existence: %w", err)
	}
	if count == 0 {
		return ErrSheetNotFound
	}
	return ErrVersionConflict
}

func versionMatcher(expectedVersion int64) any {
	if expectedVersion == 0 {
		return bson.M{"$in": bson.A{nil, int64(0)}}
	}
	return expectedVersion
}

func (r *RepositoryImpl) UpdateLayout(ctx context.Context, id string, layout *Layout) error {
	filter := bson.D{{Key: idField, Value: id}}
	update := bson.M{setOperator: bson.M{layoutField: layout}}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating sheet layout: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrSheetNotFound
	}
	return nil
}

func (r *RepositoryImpl) DeleteByID(ctx context.Context, id string) error {
	filter := bson.D{{Key: idField, Value: id}}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("error deleting sheet: %w", err)
	}
	if result.DeletedCount == 0 {
		return ErrSheetNotFound
	}
	return nil
}
