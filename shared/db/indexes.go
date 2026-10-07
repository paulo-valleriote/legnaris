package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Index names are explicit so a failing deploy points at the offending index
// instead of an anonymous auto-generated name.
const (
	usersEmailUniqueIndex          = "users_email_unique"
	refreshTokensHashUniqueIndex   = "refresh_tokens_token_hash_unique"
	refreshTokensFamilyIndex       = "refresh_tokens_family_id"
	refreshTokensExpiresAtTTLIndex = "refresh_tokens_expires_at_ttl"
	sheetsOwnerIDUniqueIndex       = "sheets_owner_id_unique"
)

// EnsureIndexes creates the indexes required by the application. It runs at
// boot, before the server accepts traffic, and is safe to call repeatedly:
// MongoDB treats an identical createIndex as a no-op. The TTL index has
// expireAfterSeconds=0, so documents are removed as soon as expires_at passes.
// The unique index on sheets.owner_id is the durable one-sheet-per-user
// invariant: it rejects a second sheet for an owner even under concurrent
// creates that both pass the service's pre-check.
func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	users := database.Collection("users")
	if _, err := users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(usersEmailUniqueIndex),
	}); err != nil {
		return fmt.Errorf("create users.email index: %w", err)
	}

	refreshTokens := database.Collection("refresh_tokens")
	if _, err := refreshTokens.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token_hash", Value: 1}},
			Options: options.Index().SetUnique(true).SetName(refreshTokensHashUniqueIndex),
		},
		{
			Keys:    bson.D{{Key: "family_id", Value: 1}},
			Options: options.Index().SetName(refreshTokensFamilyIndex),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(refreshTokensExpiresAtTTLIndex),
		},
	}); err != nil {
		return fmt.Errorf("create refresh_tokens indexes: %w", err)
	}

	sheets := database.Collection("sheets")
	if _, err := sheets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(sheetsOwnerIDUniqueIndex),
	}); err != nil {
		return fmt.Errorf("create sheets.owner_id index: %w", err)
	}

	return nil
}
