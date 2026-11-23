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
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/shared/logger"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	FetchByOwnerID(ctx context.Context, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, id string, expectedVersion int64, dbUpdates map[string]any) (int64, error)
	UpdateLayout(ctx context.Context, id string, layout *Layout) error
	LinkCampaign(ctx context.Context, id string, expectedCampaignID *string, campaignID string) (bool, error)
	UnlinkOwnerCampaign(ctx context.Context, ownerID, campaignID string) (int64, error)
	UnlinkCampaignBatch(ctx context.Context, campaignID string, limit int) (int64, error)
	ListByCampaignID(ctx context.Context, campaignID string) ([]Sheet, error)
	ListByOwnerKind(ctx context.Context, ownerID string, kind plan.SheetKind) ([]Sheet, error)
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

const lastResetRequestIDField = "last_reset_request_id"

const sheetIDLogKey = "sheet_id"

const (
	campaignIDField    = "campaign_id"
	layoutVersionField = "layout_version"
	unsetOperator      = "$unset"
	inOperator         = "$in"
)

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
		return sheeterr.ErrSheetLimitReached
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
			return nil, sheeterr.ErrSheetNotFound
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
			return nil, sheeterr.ErrSheetNotFound
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
	r.logger.Info(ctx, "sheet created", logger.Attribute{Key: sheetIDLogKey, Value: result.InsertedID.(string)})
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
		return sheeterr.ErrSheetNotFound
	}
	return sheeterr.ErrVersionConflict
}

func versionMatcher(expectedVersion int64) any {
	if expectedVersion == 0 {
		return bson.M{"$in": bson.A{nil, int64(0)}}
	}
	return expectedVersion
}

func (r *RepositoryImpl) UpdateLayout(ctx context.Context, id string, layout *Layout) error {
	filter := bson.D{{Key: idField, Value: id}}
	update := bson.M{
		setOperator: bson.M{layoutField: layout},
		incOperator: bson.M{layoutVersionField: 1},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating sheet layout: %w", err)
	}
	if result.MatchedCount == 0 {
		return sheeterr.ErrSheetNotFound
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
		return sheeterr.ErrSheetNotFound
	}
	return nil
}

func (r *RepositoryImpl) LinkCampaign(ctx context.Context, id string, expectedCampaignID *string, campaignID string) (bool, error) {
	var expected any
	if expectedCampaignID != nil {
		expected = *expectedCampaignID
	}
	filter := bson.D{
		{Key: idField, Value: id},
		{Key: campaignIDField, Value: expected},
	}
	update := bson.D{{Key: setOperator, Value: bson.D{{Key: campaignIDField, Value: campaignID}}}}
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("error linking sheet to campaign: %w", err)
	}
	return result.MatchedCount == 1, nil
}

func (r *RepositoryImpl) UnlinkOwnerCampaign(ctx context.Context, ownerID, campaignID string) (int64, error) {
	filter := bson.D{
		{Key: ownerIDField, Value: ownerID},
		{Key: campaignIDField, Value: campaignID},
	}
	update := bson.D{{Key: unsetOperator, Value: bson.D{{Key: campaignIDField, Value: ""}}}}
	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("error unlinking owner sheets from campaign: %w", err)
	}
	return result.ModifiedCount, nil
}

func (r *RepositoryImpl) UnlinkCampaignBatch(ctx context.Context, campaignID string, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	filter := bson.D{{Key: campaignIDField, Value: campaignID}}
	opts := options.Find().SetProjection(bson.D{{Key: idField, Value: 1}}).SetLimit(int64(limit))
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("error listing campaign sheets: %w", err)
	}
	var ids []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &ids); err != nil {
		return 0, fmt.Errorf("error decoding campaign sheets: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	keys := make(bson.A, 0, len(ids))
	for _, doc := range ids {
		keys = append(keys, doc.ID)
	}
	update := bson.D{{Key: unsetOperator, Value: bson.D{{Key: campaignIDField, Value: ""}}}}
	result, err := r.collection.UpdateMany(ctx, bson.D{{Key: idField, Value: bson.D{{Key: inOperator, Value: keys}}}}, update)
	if err != nil {
		return 0, fmt.Errorf("error unlinking campaign sheets: %w", err)
	}
	return result.ModifiedCount, nil
}

func (r *RepositoryImpl) ListByCampaignID(ctx context.Context, campaignID string) ([]Sheet, error) {
	filter := bson.D{{Key: campaignIDField, Value: campaignID}}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: idField, Value: 1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing campaign sheets: %w", err)
	}
	var found []Sheet
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("error decoding campaign sheets: %w", err)
	}
	return found, nil
}

func (r *RepositoryImpl) ListByOwnerKind(ctx context.Context, ownerID string, kind plan.SheetKind) ([]Sheet, error) {
	filter := bson.D{{Key: ownerIDField, Value: ownerID}, {Key: kindField, Value: kind}}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: idField, Value: 1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing owner sheets: %w", err)
	}
	var found []Sheet
	if err := cursor.All(ctx, &found); err != nil {
		return nil, fmt.Errorf("error decoding owner sheets: %w", err)
	}
	return found, nil
}
