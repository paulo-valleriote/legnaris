package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/logger"
)

const (
	emailOutboxCollection = "email_outbox"
	fieldID               = "_id"
	fieldStatus           = "status"
	fieldAttempts         = "attempts"
	fieldNextAttemptAt    = "next_attempt_at"
	fieldCreatedAt        = "created_at"
	fieldKind             = "kind"
	fieldLockedUntil      = "locked_until"
	fieldLockedBy         = "locked_by"
	fieldPublishedAt      = "published_at"
	fieldLastError        = "last_error"
	operandSet            = "$set"
	operandInc            = "$inc"
	operandLTE            = "$lte"
	operandGTE            = "$gte"
	operandOr             = "$or"
)

var ErrIntentNotFound = errors.New("intent not found")

type Repository interface {
	Create(ctx context.Context, intent Intent) error
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Intent, error)
	MarkPublished(ctx context.Context, id string, at time.Time) error
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error
	MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error
	Backlog(ctx context.Context) (int, error)
	Clone(ctx context.Context, id string, now time.Time) (Intent, error)
	CountSince(ctx context.Context, since time.Time) (int, error)
	CountKindSince(ctx context.Context, kind Kind, since time.Time) (int, error)
}

type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection(emailOutboxCollection),
		logger:     log,
	}
}

func (r *RepositoryImpl) Create(ctx context.Context, intent Intent) error {
	if err := validateIntentID(intent.ID); err != nil {
		return err
	}
	if _, err := r.collection.InsertOne(ctx, intent); err != nil {
		return fmt.Errorf("create intent: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Intent, error) {
	if limit <= 0 {
		return nil, nil
	}

	claimed := make([]Intent, 0, limit)
	for len(claimed) < limit {
		var intent Intent
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
		).Decode(&intent)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				break
			}
			return nil, fmt.Errorf("claim intent: %w", err)
		}
		claimed = append(claimed, intent)
	}
	return claimed, nil
}

func (r *RepositoryImpl) MarkPublished(ctx context.Context, id string, at time.Time) error {
	if err := validateIntentID(id); err != nil {
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
		return fmt.Errorf("mark intent published: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error {
	if err := validateIntentID(id); err != nil {
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
		return fmt.Errorf("reschedule intent: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error {
	if err := validateIntentID(id); err != nil {
		return err
	}
	if _, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: fieldID, Value: id}},
		bson.D{{Key: operandSet, Value: bson.D{
			{Key: fieldStatus, Value: StatusFailed},
			{Key: fieldLockedUntil, Value: nil},
			{Key: fieldLockedBy, Value: ""},
			{Key: fieldLastError, Value: lastError},
		}}},
	); err != nil {
		return fmt.Errorf("mark intent terminal: %w", err)
	}
	return nil
}

func (r *RepositoryImpl) Backlog(ctx context.Context) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{{Key: fieldStatus, Value: StatusPending}})
	if err != nil {
		return 0, fmt.Errorf("count pending intents: %w", err)
	}
	return int(count), nil
}

func (r *RepositoryImpl) Clone(ctx context.Context, id string, now time.Time) (Intent, error) {
	if err := validateIntentID(id); err != nil {
		return Intent{}, err
	}

	var existing Intent
	if err := r.collection.FindOne(ctx, bson.D{{Key: fieldID, Value: id}}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Intent{}, ErrIntentNotFound
		}
		return Intent{}, fmt.Errorf("read intent to clone: %w", err)
	}

	fresh, err := NewIntent("", existing.To, existing.Template, existing.Payload, now)
	if err != nil {
		return Intent{}, err
	}
	fresh.Kind = KindResend
	if err := r.Create(ctx, fresh); err != nil {
		return Intent{}, err
	}
	return fresh, nil
}

func (r *RepositoryImpl) CountSince(ctx context.Context, since time.Time) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{
		{Key: fieldCreatedAt, Value: bson.D{{Key: operandGTE, Value: since}}},
	})
	if err != nil {
		return 0, fmt.Errorf("count intents since: %w", err)
	}
	return int(count), nil
}

func (r *RepositoryImpl) CountKindSince(ctx context.Context, kind Kind, since time.Time) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{
		{Key: fieldKind, Value: kind},
		{Key: fieldCreatedAt, Value: bson.D{{Key: operandGTE, Value: since}}},
	})
	if err != nil {
		return 0, fmt.Errorf("count intents of kind since: %w", err)
	}
	return int(count), nil
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
