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

const (
	usersEmailUniqueIndex           = "users_email_unique"
	usersUsernameUniqueIndex        = "users_username_unique"
	refreshTokensHashUniqueIndex    = "refresh_tokens_token_hash_unique"
	refreshTokensFamilyIndex        = "refresh_tokens_family_id"
	refreshTokensExpiresAtTTLIndex  = "refresh_tokens_expires_at_ttl"
	sheetsOwnerKindIndex            = "sheets_owner_id_kind"
	sheetCountsUniqueIndex          = "sheet_counts_owner_id_kind_unique"
	emailOutboxStatusNextAttempt    = "email_outbox_status_next_attempt"
	emailOutboxCreatedAt            = "email_outbox_created_at"
	emailOutboxPublishedAtTTL       = "email_outbox_published_at_ttl"
	authChallengesActiveUnique      = "auth_challenges_active_unique"
	authChallengesExpiresAtTTL      = "auth_challenges_expires_at_ttl"
	emailNoticesClaimedAtTTL        = "email_notices_claimed_at_ttl"
	eventOutboxStatusNextAttempt    = "event_outbox_status_next_attempt"
	eventOutboxPublishedAtTTL       = "event_outbox_published_at_ttl"
	generationUsageUserDayUnique    = "generation_usage_user_day_unique"
	generationUsageExpiresAtTTL     = "generation_usage_expires_at_ttl"
	generationsActiveUserUnique     = "generations_active_user_id_unique"
	generationsExpiresAtTTL         = "generations_expires_at_ttl"
	generationsStatusCreatedAt      = "generations_status_created_at"
	generationCapExpiresAtTTL       = "generation_cap_expires_at_ttl"
	subscriptionsUserLiveUnique     = "subscriptions_user_live_unique"
	subscriptionsUserIDField        = "user_id"
	subscriptionsProviderSubField   = "provider_subscription_id"
	subscriptionsProviderSubUnique  = "subscriptions_provider_subscription_unique"
	subscriptionsRefundUsedUnique   = "subscriptions_refund_used_unique"
	subscriptionsCompExpiryIndex    = "subscriptions_comp_expiry"
	subscriptionsLegacyExpiresAtTTL = "subscriptions_expires_at_ttl"
	billingEventsProviderUnique     = "billing_events_provider_event_unique"
	billingEventsStatusNextAttempt  = "billing_events_status_next_attempt"
	billingEventsExpiresAtTTL       = "billing_events_expires_at_ttl"
	billingEventsNextAttemptField   = "next_attempt_at"
	createdAtField                  = "created_at"
	expiresAtField                  = "expires_at"
	statusField                     = "status"
	campaignsMembersUserIDField     = "members.user_id"
	campaignsMembersUserStatusIndex = "campaigns_members_user_id_status"
	sheetsCampaignIDIndex           = "sheets_campaign_id"
	campaignsInviteCodeUnique       = "campaigns_invite_code_unique"
	campaignsInviteCodeField        = "invite_code"
	campaignSheetLinksUnique        = "campaign_sheet_links_campaign_sheet_unique"
	campaignSheetLinksCampaignIndex = "campaign_sheet_links_campaign_id"
	campaignSheetLinksSheetIndex    = "campaign_sheet_links_sheet_id"
	campaignSheetLinksOwnerIndex    = "campaign_sheet_links_owner_id"
	adminAuditCreatedAtIndex        = "admin_audit_created_at"
	adminAuditTargetCreatedAtIndex  = "admin_audit_target_created_at"
	adminAuditActorCreatedAtIndex   = "admin_audit_actor_created_at"
	campaignIDField                 = "campaign_id"
	sheetIDField                    = "sheet_id"
	ownerIDField                    = "owner_id"
)

func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	return EnsureIndexesWithRetention(ctx, database, env.DefaultEmailOutboxRetention)
}

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

	sheets := database.Collection("sheets")
	if _, err := sheets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: ownerIDField, Value: 1}, {Key: "kind", Value: 1}},
		Options: options.Index().SetName(sheetsOwnerKindIndex),
	}); err != nil {
		return fmt.Errorf("create sheets.owner_id_kind index: %w", err)
	}

	if _, err := sheets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: campaignIDField, Value: 1}},
		Options: options.Index().SetName(sheetsCampaignIDIndex).
			SetPartialFilterExpression(bson.D{{Key: "campaign_id", Value: bson.D{{Key: "$type", Value: "string"}}}}),
	}); err != nil {
		return fmt.Errorf("create sheets.campaign_id index: %w", err)
	}

	counters := database.Collection("sheet_counts")
	if _, err := counters.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: ownerIDField, Value: 1}, {Key: "kind", Value: 1}},
		Options: options.Index().SetUnique(true).SetName(sheetCountsUniqueIndex),
	}); err != nil {
		return fmt.Errorf("create sheet_counts index: %w", err)
	}

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
	if err := ensureGenerationIndexes(ctx, database); err != nil {
		return err
	}
	if err := ensureBillingIndexes(ctx, database); err != nil {
		return err
	}
	if err := ensureAdminIndexes(ctx, database); err != nil {
		return err
	}
	return ensureCampaignIndexes(ctx, database)
}

func ensureAdminIndexes(ctx context.Context, database *mongo.Database) error {
	audit := database.Collection("admin_audit")
	if _, err := audit.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: createdAtField, Value: -1}},
			Options: options.Index().SetName(adminAuditCreatedAtIndex),
		},
		{
			Keys:    bson.D{{Key: "target_user_id", Value: 1}, {Key: createdAtField, Value: -1}},
			Options: options.Index().SetName(adminAuditTargetCreatedAtIndex),
		},
		{
			Keys:    bson.D{{Key: "actor_user_id", Value: 1}, {Key: createdAtField, Value: -1}},
			Options: options.Index().SetName(adminAuditActorCreatedAtIndex),
		},
	}); err != nil {
		return fmt.Errorf("create admin_audit indexes: %w", err)
	}
	return nil
}

func ensureCampaignIndexes(ctx context.Context, database *mongo.Database) error {
	campaigns := database.Collection("campaigns")
	if _, err := campaigns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: campaignsMembersUserIDField, Value: 1}, {Key: statusField, Value: 1}},
		Options: options.Index().SetName(campaignsMembersUserStatusIndex),
	}); err != nil {
		return fmt.Errorf("create campaigns indexes: %w", err)
	}
	if _, err := campaigns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: campaignsInviteCodeField, Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true).SetName(campaignsInviteCodeUnique),
	}); err != nil {
		return fmt.Errorf("create campaigns invite code index: %w", err)
	}
	links := database.Collection("campaign_sheet_links")
	if _, err := links.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: campaignIDField, Value: 1}, {Key: sheetIDField, Value: 1}},
			Options: options.Index().SetUnique(true).SetName(campaignSheetLinksUnique),
		},
		{
			Keys:    bson.D{{Key: campaignIDField, Value: 1}},
			Options: options.Index().SetName(campaignSheetLinksCampaignIndex),
		},
		{
			Keys:    bson.D{{Key: sheetIDField, Value: 1}},
			Options: options.Index().SetName(campaignSheetLinksSheetIndex),
		},
		{
			Keys:    bson.D{{Key: ownerIDField, Value: 1}},
			Options: options.Index().SetName(campaignSheetLinksOwnerIndex),
		},
	}); err != nil {
		return fmt.Errorf("create campaign_sheet_links indexes: %w", err)
	}
	return nil
}

func ensureBillingIndexes(ctx context.Context, database *mongo.Database) error {
	subscriptions := database.Collection("subscriptions")
	if err := dropIndexIfExists(ctx, database, "subscriptions", subscriptionsLegacyExpiresAtTTL); err != nil {
		return fmt.Errorf("drop subscriptions legacy expiry index: %w", err)
	}
	if _, err := subscriptions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: subscriptionsUserIDField, Value: 1}},
		Options: options.Index().SetUnique(true).SetName(subscriptionsUserLiveUnique).
			SetPartialFilterExpression(bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: statusField, Value: "pending"}},
				bson.D{{Key: statusField, Value: "active"}},
				bson.D{{Key: statusField, Value: "past_due"}},
			}}}),
	}); err != nil {
		return fmt.Errorf("create subscriptions indexes: %w", err)
	}
	if _, err := subscriptions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: subscriptionsUserIDField, Value: 1}},
		Options: options.Index().SetUnique(true).SetName(subscriptionsRefundUsedUnique).
			SetPartialFilterExpression(bson.D{{Key: "refund_used_at", Value: bson.D{{Key: "$exists", Value: true}}}}),
	}); err != nil {
		return fmt.Errorf("create subscriptions refund index: %w", err)
	}
	if _, err := subscriptions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "provider", Value: 1},
			{Key: statusField, Value: 1},
			{Key: "current_period_end", Value: 1},
		},
		Options: options.Index().SetName(subscriptionsCompExpiryIndex),
	}); err != nil {
		return fmt.Errorf("create subscriptions comp expiry index: %w", err)
	}
	if _, err := subscriptions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: subscriptionsProviderSubField, Value: 1}},
		Options: options.Index().SetUnique(true).SetName(subscriptionsProviderSubUnique).
			SetPartialFilterExpression(bson.D{{Key: subscriptionsProviderSubField, Value: bson.D{{Key: "$exists", Value: true}}}}),
	}); err != nil {
		return fmt.Errorf("create subscriptions provider id index: %w", err)
	}

	events := database.Collection("billing_events")
	if _, err := events.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "provider", Value: 1}, {Key: "provider_event_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName(billingEventsProviderUnique),
		},
		{
			Keys:    bson.D{{Key: statusField, Value: 1}, {Key: billingEventsNextAttemptField, Value: 1}},
			Options: options.Index().SetName(billingEventsStatusNextAttempt),
		},
		{
			Keys:    bson.D{{Key: expiresAtField, Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName(billingEventsExpiresAtTTL),
		},
	}); err != nil {
		return fmt.Errorf("create billing_events indexes: %w", err)
	}
	return nil
}

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
			Keys:    bson.D{{Key: statusField, Value: 1}, {Key: createdAtField, Value: 1}},
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

func ensureEventOutboxIndexes(ctx context.Context, database *mongo.Database, retention time.Duration) error {
	outbox := database.Collection("event_outbox")
	if _, err := outbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: billingEventsNextAttemptField, Value: 1}},
		Options: options.Index().SetName(eventOutboxStatusNextAttempt),
	}); err != nil {
		return fmt.Errorf("create event_outbox index: %w", err)
	}

	if err := ensurePublishedAtTTL(ctx, database, "event_outbox", eventOutboxPublishedAtTTL, retention); err != nil {
		return fmt.Errorf("create event_outbox TTL index: %w", err)
	}
	return nil
}

const emailNoticeRetention = 7 * 24 * time.Hour

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

func ensureEmailOutboxIndexes(ctx context.Context, database *mongo.Database, retention time.Duration) error {
	emailOutbox := database.Collection("email_outbox")
	if _, err := emailOutbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: billingEventsNextAttemptField, Value: 1}},
		Options: options.Index().SetName(emailOutboxStatusNextAttempt),
	}); err != nil {
		return fmt.Errorf("create email_outbox index: %w", err)
	}

	if _, err := emailOutbox.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: createdAtField, Value: 1}},
		Options: options.Index().SetName(emailOutboxCreatedAt),
	}); err != nil {
		return fmt.Errorf("create email_outbox created_at index: %w", err)
	}

	if err := ensurePublishedAtTTL(ctx, database, "email_outbox", emailOutboxPublishedAtTTL, retention); err != nil {
		return fmt.Errorf("create email_outbox TTL index: %w", err)
	}
	return nil
}

func dropIndexIfExists(ctx context.Context, database *mongo.Database, collection, name string) error {
	indexes := database.Collection(collection).Indexes()
	specs, err := indexes.ListSpecifications(ctx)
	if err != nil {
		return fmt.Errorf("list %s indexes: %w", collection, err)
	}
	for _, spec := range specs {
		if spec.Name == name {
			return indexes.DropOne(ctx, name)
		}
	}
	return nil
}

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

func updatePublishedAtTTL(ctx context.Context, database *mongo.Database, collection, name string, seconds int32) error {
	return database.RunCommand(ctx, bson.D{
		{Key: "collMod", Value: collection},
		{Key: "index", Value: bson.D{
			{Key: "name", Value: name},
			{Key: "expireAfterSeconds", Value: seconds},
		}},
	}).Err()
}
