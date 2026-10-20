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

// Stored field names shared by every filter and update in this file.
const (
	emailOutboxCollection = "email_outbox"
	fieldID               = "_id"
	fieldStatus           = "status"
	fieldAttempts         = "attempts"
	fieldNextAttemptAt    = "next_attempt_at"
	fieldCreatedAt        = "created_at"
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

// ErrIntentNotFound reports a Clone targeting an identity the store does not
// hold — for example one the retention window already retired.
var ErrIntentNotFound = errors.New("intent not found")

// Repository is the durable-intention contract consumed by the relay; the
// API-side caller will use Create (EML-01, EML-03, EML-04, EML-17).
type Repository interface {
	Create(ctx context.Context, intent Intent) error
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Intent, error)
	MarkPublished(ctx context.Context, id string, at time.Time) error
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error
	MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error
	Backlog(ctx context.Context) (int, error)
	// Clone creates a new pending intention from an existing one's recipient,
	// template and payload, with a fresh canonical identity, attempts = 0 and
	// next_attempt_at = now. An unknown identity is ErrIntentNotFound (OTP-10).
	Clone(ctx context.Context, id string, now time.Time) (Intent, error)
	// CountSince counts intentions recorded at or after since; it backs the
	// daily send cap (OTP-13).
	CountSince(ctx context.Context, since time.Time) (int, error)
}

// RepositoryImpl is the MongoDB-backed outbox store.
type RepositoryImpl struct {
	collection *mongo.Collection
	logger     logger.StructuredLogger
}

// NewRepository builds the outbox store over the application database.
func NewRepository(db *mongo.Database, log logger.StructuredLogger) Repository {
	return &RepositoryImpl{
		collection: db.Collection(emailOutboxCollection),
		logger:     log,
	}
}

// Create writes one document and never publishes; the request-serving caller
// returns only after it succeeds (EML-01).
func (r *RepositoryImpl) Create(ctx context.Context, intent Intent) error {
	if err := validateIntentID(intent.ID); err != nil {
		return err
	}
	if _, err := r.collection.InsertOne(ctx, intent); err != nil {
		return fmt.Errorf("create intent: %w", err)
	}
	return nil
}

// Claim leases up to limit eligible rows with one conditional findOneAndUpdate
// per row, so concurrent publishers never share a row and a stopped publisher's
// lease expires (EML-03). It does not change status or attempts: a claim is not
// a failed publication.
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

// MarkPublished sets the terminal published state and is the only writer of
// published_at; the write is idempotent by identity (EML-02).
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

// Reschedule returns the row to pending with an incremented attempt count and a
// later eligibility, so a failed publication is retried with backoff (EML-04).
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

// MarkTerminal fails the row so it is surfaced for operator attention and never
// claimed again (EML-04).
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

// Backlog counts the pending intentions so a stalled pipeline can be alerted on
// (EML-23).
func (r *RepositoryImpl) Backlog(ctx context.Context) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{{Key: fieldStatus, Value: StatusPending}})
	if err != nil {
		return 0, fmt.Errorf("count pending intents: %w", err)
	}
	return int(count), nil
}

// Clone records a resend: it reads the identified intention and writes a new
// pending row with a fresh canonical identity, the same recipient, template and
// payload, attempts = 0 and next_attempt_at = now. The fresh identity is what
// keeps the provider's idempotency key from suppressing the second send
// (R3, EML-18, OTP-10).
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
	if err := r.Create(ctx, fresh); err != nil {
		return Intent{}, err
	}
	return fresh, nil
}

// CountSince counts every intention recorded at or after since, the daily
// send-cap's only input; the {created_at: 1} index backs it (OTP-13).
func (r *RepositoryImpl) CountSince(ctx context.Context, since time.Time) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{
		{Key: fieldCreatedAt, Value: bson.D{{Key: operandGTE, Value: since}}},
	})
	if err != nil {
		return 0, fmt.Errorf("count intents since: %w", err)
	}
	return int(count), nil
}

// claimFilter selects a pending row whose eligibility has arrived and whose
// lease is absent or expired (EML-03).
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
