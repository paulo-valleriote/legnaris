package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/shared/env"
)

// Index names are explicit so a failing deploy points at the offending index
// instead of an anonymous auto-generated name.
const (
	usersEmailUniqueIndex          = "users_email_unique"
	refreshTokensHashUniqueIndex   = "refresh_tokens_token_hash_unique"
	refreshTokensFamilyIndex       = "refresh_tokens_family_id"
	refreshTokensExpiresAtTTLIndex = "refresh_tokens_expires_at_ttl"
	sheetsOwnerIDUniqueIndex       = "sheets_owner_id_unique"
	emailOutboxStatusNextAttempt   = "email_outbox_status_next_attempt"
	emailOutboxCreatedAt           = "email_outbox_created_at"
	emailOutboxPublishedAtTTL      = "email_outbox_published_at_ttl"
	authChallengesActiveUnique     = "auth_challenges_active_unique"
	authChallengesExpiresAtTTL     = "auth_challenges_expires_at_ttl"
)

// EnsureIndexes creates the indexes required by the application, retiring
// published send intentions after the documented default retention window. It
// runs at boot, before the server accepts traffic, and is safe to call
// repeatedly: MongoDB treats an identical createIndex as a no-op. The TTL index
// has expireAfterSeconds=0, so documents are removed as soon as expires_at
// passes. The unique index on sheets.owner_id is the durable
// one-sheet-per-user invariant: it rejects a second sheet for an owner even
// under concurrent creates that both pass the service's pre-check.
func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	return EnsureIndexesWithRetention(ctx, database, env.DefaultEmailOutboxRetention)
}

// EnsureIndexesWithRetention creates every index, using retention as the
// email_outbox published_at TTL window so the operator's configured
// EMAIL_OUTBOX_RETENTION actually governs how long a published intention is
// kept (EML-17). Callers that have no configured window use EnsureIndexes.
func EnsureIndexesWithRetention(ctx context.Context, database *mongo.Database, retention time.Duration) error {
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

	// The outbox queue index backs the claim query (status + eligibility); the
	// TTL on published_at retires published rows after the retention window and
	// never touches a pending or failed row, whose published_at is null
	// (EML-17).
	if err := ensureEmailOutboxIndexes(ctx, database, retention); err != nil {
		return err
	}

	if err := ensureAuthChallengeIndexes(ctx, database); err != nil {
		return err
	}

	return nil
}

// ensureAuthChallengeIndexes creates the challenge collection's two indexes: the
// unique partial index that makes at most one active challenge per account and
// purpose a store-enforced invariant, and the TTL that retires old rows as
// hygiene only — expiry is always decided by comparing the stored instant with
// the clock, never by the row's absence (OTP-01, OTP-06).
func ensureAuthChallengeIndexes(ctx context.Context, database *mongo.Database) error {
	challenges := database.Collection("auth_challenges")
	if _, err := challenges.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "purpose", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.D{{Key: "consumed_at", Value: bson.D{{Key: "$eq", Value: nil}}}}).
				SetName(authChallengesActiveUnique),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(authChallengesExpiresAtTTL),
		},
	}); err != nil {
		return fmt.Errorf("create auth_challenges indexes: %w", err)
	}
	return nil
}

// ensureEmailOutboxIndexes creates the claim index and reconciles the TTL
// index with the configured retention window.
func ensureEmailOutboxIndexes(ctx context.Context, database *mongo.Database, retention time.Duration) error {
	emailOutbox := database.Collection("email_outbox")
	if _, err := emailOutbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: "next_attempt_at", Value: 1}},
		Options: options.Index().SetName(emailOutboxStatusNextAttempt),
	}); err != nil {
		return fmt.Errorf("create email_outbox index: %w", err)
	}

	// The created_at index backs the daily send-cap count over recorded
	// intentions (OTP-13).
	if _, err := emailOutbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetName(emailOutboxCreatedAt),
	}); err != nil {
		return fmt.Errorf("create email_outbox created_at index: %w", err)
	}

	if err := ensureEmailOutboxTTL(ctx, database, retention); err != nil {
		return fmt.Errorf("create email_outbox TTL index: %w", err)
	}
	return nil
}

// ensureEmailOutboxTTL creates the published_at TTL index, or updates it in
// place when the retention window changed. MongoDB refuses createIndexes when an
// index name carries different options, so a changed window must reconcile
// through collMod instead of failing the boot (EML-17).
func ensureEmailOutboxTTL(ctx context.Context, database *mongo.Database, retention time.Duration) error {
	emailOutbox := database.Collection("email_outbox")
	seconds := int32(retention.Seconds())

	specs, err := emailOutbox.Indexes().ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("list email_outbox indexes: %w", err)
	}

	for _, spec := range specs {
		if spec.Name != emailOutboxPublishedAtTTL {
			continue
		}
		if spec.ExpireAfterSeconds != nil && *spec.ExpireAfterSeconds == seconds {
			return nil
		}
		return updateEmailOutboxTTL(ctx, database, seconds)
	}

	if _, err := emailOutbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "published_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(seconds).SetName(emailOutboxPublishedAtTTL),
	}); err != nil {
		return err
	}
	return nil
}

// updateEmailOutboxTTL applies a new retention window to the existing TTL index.
func updateEmailOutboxTTL(ctx context.Context, database *mongo.Database, seconds int32) error {
	return database.RunCommand(ctx, bson.D{
		{Key: "collMod", Value: "email_outbox"},
		{Key: "index", Value: bson.D{
			{Key: "name", Value: emailOutboxPublishedAtTTL},
			{Key: "expireAfterSeconds", Value: seconds},
		}},
	}).Err()
}
