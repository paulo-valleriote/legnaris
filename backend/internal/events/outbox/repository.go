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
	operandSet            = "$set"
	operandInc            = "$inc"
	operandLTE            = "$lte"
	operandOr             = "$or"
)

// Repository is the event outbox store. Insert must run inside the transaction
// that changes the entity: pass the session-bound context given by
// db.Transactor.WithTransaction. The other methods belong to the relay.
type Repository interface {
	// Insert stores a pending entry. Called with a session-bound context it joins
	// that transaction, so a rolled-back entity write leaves no entry behind.
	Insert(ctx context.Context, entry Entry) error
	// Claim leases up to limit eligible entries for claimer until now+lease.
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Entry, error)
	// MarkPublished sets the terminal published state.
	MarkPublished(ctx context.Context, id string, at time.Time) error
	// Reschedule returns the entry to pending with a later eligibility and one
	// more attempt.
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error
	// MarkTerminal fails the entry so it is never claimed again.
	MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error
}

// RepositoryImpl is the MongoDB-backed event outbox store.
type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

// NewRepository builds the event outbox store over the application database.
func NewRepository(database *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{collection: database.Collection(eventOutboxCollection), logger: log}
}

// Insert validates the entry and writes it. The write uses ctx, so a
// session-bound context places it in the caller's transaction.
func (r *RepositoryImpl) Insert(ctx context.Context, entry Entry) error {
	if err := entry.validate(); err != nil {
		return err
	}
	if _, err := r.collection.InsertOne(ctx, entry); err != nil {
		return fmt.Errorf("insert event outbox entry: %w", err)
	}
	return nil
}

// Claim leases up to limit eligible entries, one conditional findOneAndUpdate
// per entry, so concurrent relays never share an entry and a stopped relay's
// lease expires. A claim does not change status or attempts.
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

// MarkPublished sets the published state and clears the lease. Repeating it on
// an already published entry rewrites the same values.
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

// Reschedule returns the entry to pending, clears the lease, records the error
// and increments attempts.
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

// MarkTerminal sets the failed state, clears the lease and records the error.
func (r *RepositoryImpl) MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error {
	if err := validateID(id); err != nil {
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
		return fmt.Errorf("mark event outbox entry terminal: %w", err)
	}
	return nil
}

// validateID screens an identity before it is used in a filter.
func validateID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return fmt.Errorf("%w: identity: %v", ErrInvalidEntry, err)
	}
	return nil
}

// claimFilter selects a pending entry whose eligibility has arrived and whose
// lease is absent or expired.
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
