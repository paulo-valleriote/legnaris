package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// authChallengesCollection is the collection that holds the durable challenge
// records (data-model.md §1).
const authChallengesCollection = "auth_challenges"

// Stored field names and update operators shared by every filter in this file.
const (
	challengeFieldID         = "_id"
	challengeFieldUserID     = "user_id"
	challengeFieldPurpose    = "purpose"
	challengeFieldCodeHash   = "code_hash"
	challengeFieldExpiresAt  = "expires_at"
	challengeFieldConsumedAt = "consumed_at"
	challengeFieldCreatedAt  = "created_at"
	challengeFieldAttempts   = "attempts"
	challengeFieldLastSentAt = "last_sent_at"

	challengeSetOperator = "$set"
	challengeIncOperator = "$inc"
	challengeGTOperator  = "$gt"
	challengeLTOperator  = "$lt"
	challengeLTEOperator = "$lte"
	challengeGTEOperator = "$gte"
)

// Challenge-store sentinels. They are plain errors.New, never apperr: this is a
// store, not an HTTP boundary, exactly like user.ErrUserNotFound (contract
// challenge.contract.md).
var (
	ErrChallengeNotFound = errors.New("challenge not found")
	ErrChallengeUnusable = errors.New("challenge is not usable")
	ErrChallengeActive   = errors.New("account already has an active challenge for this purpose")
)

// ChallengeRepository is the durable contract of the challenge record: one
// active row per account and purpose, a single-document atomic consumption, a
// resend that extends the deadline inside a cool-down, and the per-account
// window count that backs the durable bound (OTP-01, OTP-04…OTP-06, OTP-14).
type ChallengeRepository interface {
	// CloseActive invalidates the account's active challenge for the purpose.
	CloseActive(ctx context.Context, userID string, purpose Purpose, at time.Time) error
	// Insert writes a new active challenge; a concurrent active row for the
	// same pair is rejected with ErrChallengeActive.
	Insert(ctx context.Context, challenge Challenge) error
	// ClaimAttempt increments the attempt count and returns the post-image only
	// while the row is usable; an unusable or unknown row is ErrChallengeUnusable.
	ClaimAttempt(ctx context.Context, id string, now time.Time, maxAttempts int) (*Challenge, error)
	// Consume claims the row by its digest and reports whether this caller won.
	Consume(ctx context.Context, id, codeHash string, at time.Time) (bool, error)
	// Extend stamps a resend: it moves the deadline to newExpiry and updates the
	// last-sent instant, only while the row is usable and its cool-down has
	// elapsed. It reports false when the resend must be suppressed.
	Extend(ctx context.Context, id string, now time.Time, cooldown time.Duration, newExpiry time.Time) (bool, error)
	// Fetch reads one row without mutating it.
	Fetch(ctx context.Context, id string) (*Challenge, error)
	// CountForUserSince counts the account's challenges created since an instant.
	CountForUserSince(ctx context.Context, userID string, since time.Time) (int, error)
}

// ChallengeRepositoryImpl is the MongoDB-backed challenge store.
type ChallengeRepositoryImpl struct {
	collection *mongo.Collection
}

// NewChallengeRepository builds the challenge store over the application
// database.
func NewChallengeRepository(db *mongo.Database) ChallengeRepository {
	return &ChallengeRepositoryImpl{collection: db.Collection(authChallengesCollection)}
}

// CloseActive consumes the account's active challenge for the purpose in one
// update. It never touches an already-consumed row (OTP-01).
func (r *ChallengeRepositoryImpl) CloseActive(ctx context.Context, userID string, purpose Purpose, at time.Time) error {
	if _, err := r.collection.UpdateMany(
		ctx,
		bson.D{
			{Key: challengeFieldUserID, Value: userID},
			{Key: challengeFieldPurpose, Value: purpose},
			{Key: challengeFieldConsumedAt, Value: nil},
		},
		bson.D{{Key: challengeSetOperator, Value: bson.D{{Key: challengeFieldConsumedAt, Value: at}}}},
	); err != nil {
		return fmt.Errorf("close active challenge: %w", err)
	}
	return nil
}

// Insert writes one active challenge. The unique partial index on
// {user_id, purpose} filtered to unconsumed rows rejects a second active row for
// the pair; the duplicate-key error is translated to ErrChallengeActive
// (OTP-01).
func (r *ChallengeRepositoryImpl) Insert(ctx context.Context, challenge Challenge) error {
	if _, err := r.collection.InsertOne(ctx, challenge); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrChallengeActive
		}
		return fmt.Errorf("insert challenge: %w", err)
	}
	return nil
}

// ClaimAttempt increments the attempt count and returns the post-image in one
// conditional findOneAndUpdate, so a wrong guess is counted durably and a
// consumed, expired, or at-cap row is never mutated. It does not compare the
// digest — the service does (OTP-04, OTP-05, OTP-06).
func (r *ChallengeRepositoryImpl) ClaimAttempt(ctx context.Context, id string, now time.Time, maxAttempts int) (*Challenge, error) {
	var claimed Challenge
	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.D{
			{Key: challengeFieldID, Value: id},
			{Key: challengeFieldConsumedAt, Value: nil},
			{Key: challengeFieldExpiresAt, Value: bson.D{{Key: challengeGTOperator, Value: now}}},
			{Key: challengeFieldAttempts, Value: bson.D{{Key: challengeLTOperator, Value: maxAttempts}}},
		},
		bson.D{{Key: challengeIncOperator, Value: bson.D{{Key: challengeFieldAttempts, Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&claimed)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrChallengeUnusable
		}
		return nil, fmt.Errorf("claim challenge attempt: %w", err)
	}
	return &claimed, nil
}

// Consume claims the row by its digest: exactly one of two racing correct codes
// matches, and a second call reports false (OTP-04).
func (r *ChallengeRepositoryImpl) Consume(ctx context.Context, id, codeHash string, at time.Time) (bool, error) {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: challengeFieldID, Value: id},
			{Key: challengeFieldConsumedAt, Value: nil},
			{Key: challengeFieldCodeHash, Value: codeHash},
		},
		bson.D{{Key: challengeSetOperator, Value: bson.D{{Key: challengeFieldConsumedAt, Value: at}}}},
	)
	if err != nil {
		return false, fmt.Errorf("consume challenge: %w", err)
	}
	return result.MatchedCount == 1, nil
}

// Extend stamps a resend in one conditional write: the deadline moves to
// newExpiry and last_sent_at to now, only while the row is usable and the
// cool-down has elapsed. No match reports false, which is both the cool-down
// suppression and the concurrent-resend suppression (OTP-10).
func (r *ChallengeRepositoryImpl) Extend(ctx context.Context, id string, now time.Time, cooldown time.Duration, newExpiry time.Time) (bool, error) {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{
			{Key: challengeFieldID, Value: id},
			{Key: challengeFieldConsumedAt, Value: nil},
			{Key: challengeFieldExpiresAt, Value: bson.D{{Key: challengeGTOperator, Value: now}}},
			{Key: challengeFieldLastSentAt, Value: bson.D{{Key: challengeLTEOperator, Value: now.Add(-cooldown)}}},
		},
		bson.D{{Key: challengeSetOperator, Value: bson.D{
			{Key: challengeFieldExpiresAt, Value: newExpiry},
			{Key: challengeFieldLastSentAt, Value: now},
		}}},
	)
	if err != nil {
		return false, fmt.Errorf("extend challenge: %w", err)
	}
	return result.MatchedCount == 1, nil
}

// Fetch reads one row without mutating it. The service uses it to classify a
// failure for metrics and to read the stored intention identity and the ceiling
// for a resend (OTP-16, R4).
func (r *ChallengeRepositoryImpl) Fetch(ctx context.Context, id string) (*Challenge, error) {
	var challenge Challenge
	if err := r.collection.FindOne(ctx, bson.D{{Key: challengeFieldID, Value: id}}).Decode(&challenge); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("fetch challenge: %w", err)
	}
	return &challenge, nil
}

// CountForUserSince counts the account's challenges created at or after since,
// backing the durable per-account bound over the configured window (OTP-14).
func (r *ChallengeRepositoryImpl) CountForUserSince(ctx context.Context, userID string, since time.Time) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{
		{Key: challengeFieldUserID, Value: userID},
		{Key: challengeFieldCreatedAt, Value: bson.D{{Key: challengeGTEOperator, Value: since}}},
	})
	if err != nil {
		return 0, fmt.Errorf("count challenges for user: %w", err)
	}
	return int(count), nil
}
