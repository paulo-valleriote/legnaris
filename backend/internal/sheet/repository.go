package sheet

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/logger"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	FetchByOwnerID(ctx context.Context, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, id string, expectedVersion int64, dbUpdates map[string]any) (int64, error)
	UpdateLayout(ctx context.Context, id string, layout *Layout) error
	DeleteByID(ctx context.Context, id string) error
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

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

// ErrSheetOwnerExists reports a create that collided with the unique
// sheets.owner_id index, meaning the owner already has a sheet. It is the
// race-safe signal services translate to a sheet-limit conflict.
var ErrSheetOwnerExists = errors.New("sheet owner already has a sheet")

// ErrVersionConflict reports a compare-and-swap write whose expected version no
// longer matches the stored sheet, or whose sheet was deleted in between. The
// service re-reads the sheet and retries on it.
var ErrVersionConflict = errors.New("sheet version conflict")

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection("sheets"),
		logger:     log,
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

	filter := bson.D{{Key: "owner_id", Value: ownerID}}
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
		// The sheets.owner_id unique index is the one-sheet-per-user guard, so a
		// duplicate-key insert means the owner already has a sheet. Surfacing it
		// as a typed error lets the service answer 409 instead of a raw 500.
		if mongo.IsDuplicateKeyError(err) {
			return ErrSheetOwnerExists
		}
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
