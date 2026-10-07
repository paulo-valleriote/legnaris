package sheet

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/shared/logger"
)

type Repository interface {
	Create(ctx context.Context, sheet *Sheet) error
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	FetchByOwnerID(ctx context.Context, ownerID string) (*Sheet, error)
	PartialUpdate(ctx context.Context, id string, dbUpdates map[string]any) error
	DeleteByID(ctx context.Context, id string) error
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

// idField is the Mongo primary-key field name, shared by the by-id filters so
// the literal stays in one place.
const idField = "_id"

// ErrSheetNotFound reports a sheet lookup that matched no document.
// Services translate it to apperr.CodeNotFound at the layer boundary.
var ErrSheetNotFound = errors.New("sheet not found")

// ErrSheetOwnerExists reports a create that collided with the unique
// sheets.owner_id index, meaning the owner already has a sheet. It is the
// race-safe signal services translate to a sheet-limit conflict.
var ErrSheetOwnerExists = errors.New("sheet owner already has a sheet")

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

func (r *RepositoryImpl) PartialUpdate(ctx context.Context, id string, dbUpdates map[string]any) error {
	filter := bson.D{{Key: idField, Value: id}}
	update := bson.M{"$set": dbUpdates}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("error updating sheet: %w", err)
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
