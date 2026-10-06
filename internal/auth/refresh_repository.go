package auth

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	FetchByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	// RevokeByID atomically revokes a non-revoked token, pointing it at its
	// replacement. It reports false when the token was already revoked (or
	// gone), which is how rotation detects a concurrent reuse without a
	// separate read-check-write window.
	RevokeByID(ctx context.Context, id string, replacedBy string) (bool, error)
	RevokeByHash(ctx context.Context, tokenHash string) (bool, error)
	RevokeFamily(ctx context.Context, familyID string) error
}

type RefreshTokenRepositoryImpl struct {
	collection *mongo.Collection
}

// ErrRefreshTokenNotFound reports a lookup that matched no refresh token.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

const (
	fieldRevoked = "revoked"
	setOperator  = "$set"
)

func NewRefreshTokenRepository(db *mongo.Database) RefreshTokenRepository {
	return &RefreshTokenRepositoryImpl{
		collection: db.Collection("refresh_tokens"),
	}
}

func (r *RefreshTokenRepositoryImpl) Create(ctx context.Context, token *RefreshToken) error {
	if _, err := r.collection.InsertOne(ctx, token); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepositoryImpl) FetchByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var result RefreshToken
	if err := r.collection.FindOne(ctx, bson.D{{Key: "token_hash", Value: tokenHash}}).Decode(&result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("fetch refresh token: %w", err)
	}
	return &result, nil
}

func (r *RefreshTokenRepositoryImpl) RevokeByID(ctx context.Context, id string, replacedBy string) (bool, error) {
	set := bson.M{fieldRevoked: true}
	if replacedBy != "" {
		set["replaced_by"] = replacedBy
	}
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{{Key: "_id", Value: id}, {Key: fieldRevoked, Value: false}},
		bson.M{setOperator: set},
	)
	if err != nil {
		return false, fmt.Errorf("revoke refresh token: %w", err)
	}
	return result.MatchedCount == 1, nil
}

func (r *RefreshTokenRepositoryImpl) RevokeByHash(ctx context.Context, tokenHash string) (bool, error) {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.D{{Key: "token_hash", Value: tokenHash}, {Key: fieldRevoked, Value: false}},
		bson.M{setOperator: bson.M{fieldRevoked: true}},
	)
	if err != nil {
		return false, fmt.Errorf("revoke refresh token by hash: %w", err)
	}
	return result.MatchedCount == 1, nil
}

func (r *RefreshTokenRepositoryImpl) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := r.collection.UpdateMany(
		ctx,
		bson.D{{Key: "family_id", Value: familyID}, {Key: fieldRevoked, Value: false}},
		bson.M{setOperator: bson.M{fieldRevoked: true}},
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	return nil
}
