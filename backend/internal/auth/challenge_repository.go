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

const authChallengesCollection = "auth_challenges"

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

var (
	ErrChallengeNotFound = errors.New("challenge not found")
	ErrChallengeUnusable = errors.New("challenge is not usable")
	ErrChallengeActive   = errors.New("account already has an active challenge for this purpose")
)

type ChallengeRepository interface {
	CloseActive(ctx context.Context, userID string, purpose Purpose, at time.Time) error
	Insert(ctx context.Context, challenge Challenge) error
	ClaimAttempt(ctx context.Context, id string, now time.Time, maxAttempts int) (*Challenge, error)
	Consume(ctx context.Context, id, codeHash string, at time.Time) (bool, error)
	Extend(ctx context.Context, id string, now time.Time, cooldown time.Duration, newExpiry time.Time) (bool, error)
	Fetch(ctx context.Context, id string) (*Challenge, error)
	CountForUserSince(ctx context.Context, userID string, since time.Time) (int, error)
}

type ChallengeRepositoryImpl struct {
	collection *mongo.Collection
}

func NewChallengeRepository(db *mongo.Database) ChallengeRepository {
	return &ChallengeRepositoryImpl{collection: db.Collection(authChallengesCollection)}
}

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

func (r *ChallengeRepositoryImpl) Insert(ctx context.Context, challenge Challenge) error {
	if _, err := r.collection.InsertOne(ctx, challenge); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrChallengeActive
		}
		return fmt.Errorf("insert challenge: %w", err)
	}
	return nil
}

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
