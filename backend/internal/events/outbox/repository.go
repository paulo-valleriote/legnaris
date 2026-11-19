package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

const (
	eventOutboxCollection = "event_outbox"
	fieldID               = "_id"
	fieldStatus           = "status"
	fieldAttempts         = "attempts"
	fieldNextAttemptAt    = "next_attempt_at"
	fieldLockedUntil      = "locked_until"
	fieldLockedBy         = "locked_by"
	fieldPublishedAt      = "published_at"
	fieldLastError        = "last_error"
	fieldCreatedAt        = "created_at"
	operandSet            = "$set"
	operandInc            = "$inc"
	operandLTE            = "$lte"
	operandOr             = "$or"
)

type Repository interface {
	Insert(ctx context.Context, entry Entry) error
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Entry, error)
	MarkPublished(ctx context.Context, id string, at time.Time) error
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error
	MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error
	OldestPending(ctx context.Context) (time.Time, bool, error)
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

func NewRepository(database *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{collection: database.Collection(eventOutboxCollection), logger: log}
}

func (r *RepositoryImpl) Insert(ctx context.Context, entry Entry) error {
	if err := entry.validate(); err != nil {
		return err
	}
	if _, err := r.collection.InsertOne(ctx, entry); err != nil {
		return fmt.Errorf("insert event outbox entry: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Entry, error) {
	if limit <= 0 {
		return nil, nil
	}

	claimed := make([]Entry, 0, limit)
	for len(claimed) < limit {
		var entry Entry
		err := r.collection.FindOneAndUpdate(
			ctx,
			claimFilter(now),
			bson.D{{Key: operandSet, Value: bson.D{
				{Key: fieldLockedUntil, Value: now.Add(lease)},
				{Key: fieldLockedBy, Value: claimer},
			}}},
			options.FindOneAndUpdate().
				SetSort(bson.D{{Key: fieldNextAttemptAt, Value: 1}}).
				SetReturnDocument(options.After),
		).Decode(&entry)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				break
			}
			return nil, fmt.Errorf("claim event outbox entry: %w", err)
		}
		claimed = append(claimed, entry)
	}
	return claimed, nil
}

func (r *RepositoryImpl) MarkPublished(ctx context.Context, id string, at time.Time) error {
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{{Key: operandSet, Value: bson.D{
			{Key: fieldStatus, Value: StatusPublished},
			{Key: fieldPublishedAt, Value: at},
			{Key: fieldLockedUntil, Value: nil},
			{Key: fieldLockedBy, Value: ""},
		}}},
	); err != nil {
		return fmt.Errorf("mark event outbox entry published: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{
			{Key: operandSet, Value: bson.D{
				{Key: fieldStatus, Value: StatusPending},
				{Key: fieldNextAttemptAt, Value: nextAt},
				{Key: fieldLockedUntil, Value: nil},
				{Key: fieldLockedBy, Value: ""},
				{Key: fieldLastError, Value: lastError},
			}},
			{Key: operandInc, Value: bson.D{{Key: fieldAttempts, Value: 1}}},
		},
	); err != nil {
		return fmt.Errorf("reschedule event outbox entry: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{{Key: operandSet, Value: bson.D{
			{Key: fieldStatus, Value: StatusDead},
			{Key: fieldLockedUntil, Value: nil},
			{Key: fieldLockedBy, Value: ""},
			{Key: fieldLastError, Value: lastError},
		}}},
	); err != nil {
		return fmt.Errorf("mark event outbox entry terminal: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) OldestPending(ctx context.Context) (time.Time, bool, error) {
	var entry Entry
	err := r.collection.FindOne(ctx,
		bson.D{{Key: fieldStatus, Value: StatusPending}},
		options.FindOne().
			SetSort(bson.D{{Key: fieldCreatedAt, Value: 1}}).
			SetProjection(bson.D{{Key: fieldCreatedAt, Value: 1}}),
	).Decode(&entry)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("find oldest pending event outbox entry: %w", err)
	}
	return entry.CreatedAt, true, nil
}

func validateID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return fmt.Errorf("%w: identity: %w", ErrInvalidEntry, err)
	}
	return nil
}

func claimFilter(now time.Time) bson.D {
	return bson.D{
		{Key: fieldStatus, Value: StatusPending},
		{Key: fieldNextAttemptAt, Value: bson.D{{Key: operandLTE, Value: now}}},
		{Key: operandOr, Value: bson.A{
			bson.D{{Key: fieldLockedUntil, Value: nil}},
			bson.D{{Key: fieldLockedUntil, Value: bson.D{{Key: operandLTE, Value: now}}}},
		}},
	}
}
