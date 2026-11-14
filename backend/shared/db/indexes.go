package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"legnaris/backend/shared/env"
)

// Index names are explicit so a failing deploy points at the offending index
// instead of an anonymous auto-generated name.
const (
	usersEmailUniqueIndex          = "users_email_unique"
	usersUsernameUniqueIndex       = "users_username_unique"
	refreshTokensHashUniqueIndex   = "refresh_tokens_token_hash_unique"
	refreshTokensFamilyIndex       = "refresh_tokens_family_id"
	refreshTokensExpiresAtTTLIndex = "refresh_tokens_expires_at_ttl"
	sheetsOwnerKindIndex           = "sheets_owner_id_kind"
	sheetCountsUniqueIndex         = "sheet_counts_owner_id_kind_unique"
	emailOutboxStatusNextAttempt   = "email_outbox_status_next_attempt"
	emailOutboxCreatedAt           = "email_outbox_created_at"
	emailOutboxPublishedAtTTL      = "email_outbox_published_at_ttl"
	authChallengesActiveUnique     = "auth_challenges_active_unique"
	authChallengesExpiresAtTTL     = "auth_challenges_expires_at_ttl"
	emailNoticesClaimedAtTTL       = "email_notices_claimed_at_ttl"
	generationUsageUserDayUnique   = "generation_usage_user_day_unique"
	generationUsageExpiresAtTTL    = "generation_usage_expires_at_ttl"
	generationsActiveUserUnique    = "generations_active_user_id_unique"
	generationsExpiresAtTTL        = "generations_expires_at_ttl"
	generationsStatusCreatedAt     = "generations_status_created_at"
	generationCapExpiresAtTTL      = "generation_cap_expires_at_ttl"
	// expiresAtField is the TTL field shared by every collection that retires
	// rows by expiry.
	expiresAtField = "expires_at"
	// statusField is the document field holding a job or outbox state.
	statusField = "status"
)

// EnsureIndexes creates the indexes required by the application, retiring
// published send intentions after the documented default retention window. It
// runs at boot, before the server accepts traffic, and is safe to call
// repeatedly: MongoDB treats an identical createIndex as a no-op. The TTL index
// has expireAfterSeconds=0, so documents are removed as soon as expires_at
// passes. The unique index on sheet_counts (owner_id, kind) keeps one counter per
// owner and kind, which the Sheet limit reads inside its transaction.
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
	if _, err := users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(usersUsernameUniqueIndex),
	}); err != nil {
		return fmt.Errorf("create users.username index: %w", err)
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
			Keys:    bson.D{{Key: expiresAtField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(refreshTokensExpiresAtTTLIndex),
		},
	}); err != nil {
		return fmt.Errorf("create refresh_tokens indexes: %w", err)
	}

	// The per-kind limit is enforced by sheet_counts, so sheets only needs a
	// lookup index. The unique owner index it replaces is dropped by the kind
	// migration (LGN-183).
	sheets := database.Collection("sheets")
	if _, err := sheets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}, {Key: "kind", Value: 1}},
		Options: options.Index().SetName(sheetsOwnerKindIndex),
	}); err != nil {
		return fmt.Errorf("create sheets.owner_id_kind index: %w", err)
	}

	// One counter per owner and kind. Its uniqueness is what keeps the upsert in
	// the reservation from creating a second row.
	counters := database.Collection("sheet_counts")
	if _, err := counters.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner_id", Value: 1}, {Key: "kind", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(sheetCountsUniqueIndex),
	}); err != nil {
		return fmt.Errorf("create sheet_counts index: %w", err)
	}

	// The outbox queue index backs the claim query (status + eligibility); the
	// TTL on published_at retires published rows after the retention window and
	// never touches a pending or failed row, whose published_at is null
	// (EML-17).
	if err := ensureEmailOutboxIndexes(ctx, database, retention); err != nil {
		return err
	}

	if err := ensureEventOutboxIndexes(ctx, database, retention); err != nil {
		return err
	}

	if err := ensureAuthChallengeIndexes(ctx, database); err != nil {
		return err
	}

	if err := ensureEmailNoticeIndexes(ctx, database); err != nil {
		return err
	}

	if err := ensureGenerationUsageIndexes(ctx, database); err != nil {
		return err
	}
	return ensureGenerationIndexes(ctx, database)
}

// ensureGenerationIndexes creates the job collection's indexes. The sparse
// unique key on active_user_id is the race guard of "one queued or running
// generation per user": the field exists only while a job is in progress, so a
// second insert for the same player fails with a duplicate key. The TTL retires
// a generation 24 h after creation (ADR 0008). The status index serves the
// publish sweeper, and the cap collection's TTL retires counters a day after
// their day closes.
func ensureGenerationIndexes(ctx context.Context, database *mongo.Database) error {
	generations := database.Collection("generations")
	if _, err := generations.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "active_user_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true).SetName(generationsActiveUserUnique),
		},
		{
			Keys:    bson.D{{Key: expiresAtField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(generationsExpiresAtTTL),
		},
		{
			Keys:    bson.D{{Key: statusField, Value: 1}, {Key: "created_at", Value: 1}},
			Options: options.Index().SetName(generationsStatusCreatedAt),
		},
	}); err != nil {
		return fmt.Errorf("create generations indexes: %w", err)
	}

	capCounters := database.Collection("generation_cap")
	if _, err := capCounters.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: expiresAtField, Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0).SetName(generationCapExpiresAtTTL),
	}); err != nil {
		return fmt.Errorf("create generation_cap indexes: %w", err)
	}
	return nil
}

// ensureGenerationUsageIndexes creates the daily quota counter's unique key and
// its TTL. The unique key on (user_id, day_utc) makes the day-opening upsert
// race-safe; the TTL retires counter rows once their day is long past.
func ensureGenerationUsageIndexes(ctx context.Context, database *mongo.Database) error {
	usage := database.Collection("generation_usage")
	if _, err := usage.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "day_utc", Value: 1}},
			Options: options.Index().SetUnique(true).SetName(generationUsageUserDayUnique),
		},
		{
			Keys:    bson.D{{Key: expiresAtField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(generationUsageExpiresAtTTL),
		},
	}); err != nil {
		return fmt.Errorf("create generation_usage indexes: %w", err)
	}
	return nil
}

// ensureEventOutboxIndexes creates the claim index for event_outbox and the TTL
// that retires published rows after the retention window. Pending and failed
// rows keep published_at null, which the TTL never matches (ADR 0013).
func ensureEventOutboxIndexes(ctx context.Context, database *mongo.Database, retention time.Duration) error {
	outbox := database.Collection("event_outbox")
	if _, err := outbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: "next_attempt_at", Value: 1}},
		Options: options.Index().SetName(eventOutboxStatusNextAttempt),
	}); err != nil {
		return fmt.Errorf("create event_outbox index: %w", err)
	}

	if err := ensurePublishedAtTTL(ctx, database, "event_outbox", eventOutboxPublishedAtTTL, retention); err != nil {
		return fmt.Errorf("create event_outbox TTL index: %w", err)
	}
	return nil
}

// emailNoticeRetention outlives any configured notice window, so a claim is
// never dropped while it can still throttle.
const emailNoticeRetention = 7 * 24 * time.Hour

// ensureEmailNoticeIndexes retires notice claims after emailNoticeRetention.
func ensureEmailNoticeIndexes(ctx context.Context, database *mongo.Database) error {
	notices := database.Collection("email_notices")
	if _, err := notices.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "claimed_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(emailNoticeRetention.Seconds())).SetName(emailNoticesClaimedAtTTL),
	}); err != nil {
		return fmt.Errorf("create email_notices indexes: %w", err)
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
			Keys:    bson.D{{Key: expiresAtField, Value: 1}},
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

	if err := ensurePublishedAtTTL(ctx, database, "email_outbox", emailOutboxPublishedAtTTL, retention); err != nil {
		return fmt.Errorf("create email_outbox TTL index: %w", err)
	}
	return nil
}

// ensurePublishedAtTTL creates the published_at TTL index named name on
// collection, or reconciles its window in place when retention changed. MongoDB
// refuses createIndexes when an index name carries different options, so a
// changed window must go through collMod instead of failing the boot (EML-17).
func ensurePublishedAtTTL(ctx context.Context, database *mongo.Database, collection, name string, retention time.Duration) error {
	indexes := database.Collection(collection).Indexes()
	seconds := int32(retention.Seconds())

	specs, err := indexes.ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("list %s indexes: %w", collection, err)
	}

	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		if spec.ExpireAfterSeconds != nil && *spec.ExpireAfterSeconds == seconds {
			return nil
		}
		return updatePublishedAtTTL(ctx, database, collection, name, seconds)
	}

	_, err = indexes.CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "published_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(seconds).SetName(name),
	})
	return err
}

// updatePublishedAtTTL applies a new retention window to an existing TTL index.
func updatePublishedAtTTL(ctx context.Context, database *mongo.Database, collection, name string, seconds int32) error {
	return database.RunCommand(ctx, bson.D{
		{Key: "collMod", Value: collection},
		{Key: "index", Value: bson.D{
			{Key: "name", Value: name},
			{Key: "expireAfterSeconds", Value: seconds},
		}},
	}).Err()
}
