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
	// ReserveSlot increments the owner's counter for kind while it is below
	// limit, and returns ErrSheetLimitReached otherwise. It must run inside the
	// transaction that inserts the Sheet.
	ReserveSlot(ctx context.Context, ownerID string, kind plan.SheetKind, limit int) error
	// ReleaseSlot decrements the owner's counter for kind, never below zero. It
	// must run inside the transaction that deletes the Sheet.
	ReleaseSlot(ctx context.Context, ownerID string, kind plan.SheetKind) error
	// CountOf reads the owner's counter for kind from the primary. A missing
	// counter reads as zero.
	CountOf(ctx context.Context, ownerID string, kind plan.SheetKind) (int, error)
}

type RepositoryImpl struct {
	collection *mongo.Collection
	counts     *mongo.Collection
	logger     logger.StructuredLogger
}

// countsCollection holds one counter per owner and kind. The limit is enforced
// by a conditional $inc on this collection inside the Sheet transaction.
const countsCollection = "sheet_counts"

// countField is the Mongo field holding a counter value.
const countField = "count"

// ownerIDField and kindField are the Mongo fields that identify a counter, and
// incOperator is the Mongo operator that moves it.
const (
	ownerIDField = "owner_id"
	kindField    = "kind"
	incOperator  = "$inc"
)

// detailKindKey is the apperr details key that names the refused Sheet kind.
const detailKindKey = "kind"

// idField is the Mongo primary-key field name, shared by the by-id filters so
// the literal stays in one place.
const idField = "_id"

// layoutField is the Mongo field holding the sheet's Layout.
const layoutField = "layout"

// setOperator is the Mongo update operator every sheet write goes through.
const setOperator = "$set"

// diceField is the Mongo field holding the sheet's Dice block.
const diceField = "dice"

// versionField is the Mongo field holding the sheet's compare-and-swap version.
const versionField = "version"

// ErrSheetNotFound reports a sheet lookup that matched no document.
// Services translate it to apperr.CodeNotFound at the layer boundary.
var ErrSheetNotFound = errors.New("sheet not found")

// ErrVersionConflict reports a compare-and-swap write whose expected version no
// longer matches the stored sheet, or whose sheet was deleted in between. The
// service re-reads the sheet and retries on it.
var ErrVersionConflict = errors.New("sheet version conflict")

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection("sheets"),
		counts:     db.Collection(countsCollection, options.Collection().SetReadPreference(readpref.Primary())),
		logger:     log,
	}
}

// ReserveSlot opens the counter for owner and kind, then increments it only while
// it is below limit. The two statements are separate because an upsert whose
// filter carries the limit would collide with the unique index once the counter
// is full, which inside a transaction aborts it instead of refusing the write.
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

// ReleaseSlot decrements the counter, and the $gt filter stops it going negative
// when a counter was never built for an old Sheet.
func (r *RepositoryImpl) ReleaseSlot(ctx context.Context, ownerID string, kind plan.SheetKind) error {
	positive := append(countKey(ownerID, kind), bson.E{Key: countField, Value: bson.D{{Key: "$gt", Value: 0}}})
	if _, err := r.counts.UpdateOne(ctx, positive, bson.D{{Key: incOperator, Value: bson.D{{Key: countField, Value: -1}}}}); err != nil {
		return fmt.Errorf("error releasing sheet slot: %w", err)
	}
	return nil
}

// CountOf reads the counter from the primary, since the value decides whether a
// creation is allowed.
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

// countKey is the filter that identifies one owner's counter for one kind.
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

// PartialUpdate applies dbUpdates and increments the version, but only while the
// stored version still equals expectedVersion. It returns the version the write
// stored, read back from the same atomic operation, so callers log the value
// the database holds instead of recomputing it. A document without the version
// field matches expectedVersion 0, since that is what a pre-versioning sheet
// reads as. It returns ErrSheetNotFound when the sheet does not exist and
// ErrVersionConflict when it exists but its version moved.
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

// unmatchedUpdateErr explains a CAS write that matched no document. The filter
// cannot tell a missing sheet from a stale version, so the id is counted alone:
// a missing sheet is ErrSheetNotFound, which must not be retried as a conflict.
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

// versionMatcher builds the version filter for a CAS write. Expecting 0 also
// matches a null or missing field, because Mongo's null equality covers absent
// fields and that is how sheets written before versioning look.
func versionMatcher(expectedVersion int64) any {
	if expectedVersion == 0 {
		return bson.M{"$in": bson.A{nil, int64(0)}}
	}
	return expectedVersion
}

// UpdateLayout replaces the stored layout of one sheet with a $set on that
// single field, so a concurrent write to data or any other field is neither
// read nor overwritten. A nil layout stores null, which reads back as no
// layout. It returns ErrSheetNotFound when no sheet matches.
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
