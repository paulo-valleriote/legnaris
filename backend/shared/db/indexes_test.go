package db_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func newIndexesTestDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	return dbtest.Database(t)
}

func listIndexes(t *testing.T, mdb *mongo.Database, collection string) map[string]bson.M {
	t.Helper()

	cursor, err := mdb.Collection(collection).Indexes().List(context.Background())
	require.NoError(t, err)

	var listed []bson.M
	require.NoError(t, cursor.All(context.Background(), &listed))

	indexes := make(map[string]bson.M, len(listed))
	for _, idx := range listed {
		name, ok := idx["name"].(string)
		require.True(t, ok, "every index must carry a string name")
		indexes[name] = idx
	}
	return indexes
}

func indexIsUnique(idx bson.M) bool {
	unique, ok := idx["unique"].(bool)
	return ok && unique
}

func indexExpireAfterSeconds(t *testing.T, idx bson.M) int32 {
	t.Helper()
	val, ok := idx["expireAfterSeconds"]
	require.True(t, ok, "TTL index must carry expireAfterSeconds")
	seconds, ok := val.(int32)
	require.True(t, ok, "expireAfterSeconds must be int32")
	return seconds
}

func TestEnsureIndexes(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()

	t.Run("Happy path: creates the auth indexes", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexes(ctx, mdb))

		users := listIndexes(t, mdb, "users")
		usersEmail, ok := users["users_email_unique"]
		require.True(t, ok, "users.email unique index must exist, got %v", users)
		assert.True(t, indexIsUnique(usersEmail))

		usersUsername, ok := users["users_username_unique"]
		require.True(t, ok, "users.username unique index must exist, got %v", users)
		assert.True(t, indexIsUnique(usersUsername))

		refresh := listIndexes(t, mdb, "refresh_tokens")
		tokenHash, ok := refresh["refresh_tokens_token_hash_unique"]
		require.True(t, ok, "token_hash unique index must exist, got %v", refresh)
		assert.True(t, indexIsUnique(tokenHash))

		_, ok = refresh["refresh_tokens_family_id"]
		assert.True(t, ok, "family_id index must exist")

		ttl, ok := refresh["refresh_tokens_expires_at_ttl"]
		require.True(t, ok, "TTL index must exist")
		assert.Equal(t, int32(0), indexExpireAfterSeconds(t, ttl), "TTL index must expire at expires_at")

		sheets := listIndexes(t, mdb, "sheets")
		ownerKind, ok := sheets["sheets_owner_id_kind"]
		require.True(t, ok, "sheets (owner_id, kind) index must exist, got %v", sheets)
		assert.False(t, indexIsUnique(ownerKind), "one owner may hold several kinds, so the index is not unique")

		counters := listIndexes(t, mdb, "sheet_counts")
		counterKey, ok := counters["sheet_counts_owner_id_kind_unique"]
		require.True(t, ok, "sheet_counts (owner_id, kind) unique index must exist, got %v", counters)
		assert.True(t, indexIsUnique(counterKey), "one counter per owner and kind")

		campaignID, ok := sheets["sheets_campaign_id"]
		require.True(t, ok, "sheets campaign_id index must exist, got %v", sheets)
		assert.False(t, indexIsUnique(campaignID), "many sheets may link to one campaign")

		links := listIndexes(t, mdb, "campaign_sheet_links")
		linkPair, ok := links["campaign_sheet_links_campaign_sheet_unique"]
		require.True(t, ok, "campaign_sheet_links (campaign_id, sheet_id) unique index must exist, got %v", links)
		assert.True(t, indexIsUnique(linkPair), "a sheet links to a campaign at most once")
		_, ok = links["campaign_sheet_links_campaign_id"]
		assert.True(t, ok, "campaign_sheet_links campaign_id index must exist")
		_, ok = links["campaign_sheet_links_sheet_id"]
		assert.True(t, ok, "campaign_sheet_links sheet_id index must exist")
	})

	t.Run("Edge: calling twice is idempotent", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexes(ctx, mdb))
		require.NoError(t, db.EnsureIndexes(ctx, mdb))
	})

	t.Run("Edge: unique email index rejects a duplicate", func(t *testing.T) {
		users := mdb.Collection("users")
		_, err := users.InsertOne(ctx, bson.M{"_id": "user-1", "email": "dup@example.com"})
		require.NoError(t, err)

		_, err = users.InsertOne(ctx, bson.M{"_id": "user-2", "email": "dup@example.com"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "duplicate email must be a duplicate key error")
	})

	t.Run("Edge: unique token hash index rejects a duplicate", func(t *testing.T) {
		tokens := mdb.Collection("refresh_tokens")
		_, err := tokens.InsertOne(ctx, bson.M{
			"_id":        "token-1",
			"token_hash": "abc",
			"expires_at": time.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		_, err = tokens.InsertOne(ctx, bson.M{
			"_id":        "token-2",
			"token_hash": "abc",
			"expires_at": time.Now().Add(time.Hour),
		})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "duplicate token hash must be a duplicate key error")
	})

	t.Run("Edge: a second sheet of another kind for the owner is allowed", func(t *testing.T) {
		sheets := mdb.Collection("sheets")
		_, err := sheets.InsertOne(ctx, bson.M{"_id": "sheet-1", "owner_id": "owner-1", "kind": "character"})
		require.NoError(t, err)

		_, err = sheets.InsertOne(ctx, bson.M{"_id": "sheet-2", "owner_id": "owner-1", "kind": "npc"})
		require.NoError(t, err)
	})

	t.Run("Edge: unique sheet_counts index rejects a second counter for the owner and kind", func(t *testing.T) {
		counters := mdb.Collection("sheet_counts")
		_, err := counters.InsertOne(ctx, bson.M{"owner_id": "owner-2", "kind": "npc", "count": 0})
		require.NoError(t, err)

		_, err = counters.InsertOne(ctx, bson.M{"owner_id": "owner-2", "kind": "npc", "count": 0})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "a second counter for the owner and kind must be a duplicate key error")
	})

	t.Run("Edge: a second link for the same campaign and sheet is rejected", func(t *testing.T) {
		links := mdb.Collection("campaign_sheet_links")
		_, err := links.InsertOne(ctx, bson.M{"_id": "link-1", "campaign_id": "campaign-1", "sheet_id": "sheet-1"})
		require.NoError(t, err)

		_, err = links.InsertOne(ctx, bson.M{"_id": "link-2", "campaign_id": "campaign-1", "sheet_id": "sheet-1"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "one link per campaign and sheet")
	})
}

func TestEnsureIndexesWithRetention(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()

	t.Run("Happy path: the configured window is applied", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 24*time.Hour))

		emailOutbox := listIndexes(t, mdb, "email_outbox")
		ttl, ok := emailOutbox["email_outbox_published_at_ttl"]
		require.True(t, ok, "the outbox TTL index must exist, got %v", emailOutbox)
		assert.Equal(t, int32((24 * time.Hour).Seconds()), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Edge: a changed window reconciles the existing index", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 48*time.Hour))

		emailOutbox := listIndexes(t, mdb, "email_outbox")
		ttl, ok := emailOutbox["email_outbox_published_at_ttl"]
		require.True(t, ok, "the outbox TTL index must exist, got %v", emailOutbox)
		assert.Equal(t, int32((48 * time.Hour).Seconds()), indexExpireAfterSeconds(t, ttl),
			"MongoDB refuses createIndexes with changed options, so the window must be reconciled in place")
	})

	t.Run("Edge: re-applying the same window is idempotent", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 48*time.Hour))
	})
}

func TestEnsureIndexes_AuthChallenges(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	challenges := listIndexes(t, mdb, "auth_challenges")

	t.Run("Happy path: the active unique index exists with a partial filter", func(t *testing.T) {
		active, ok := challenges["auth_challenges_active_unique"]
		require.True(t, ok, "the active-challenge unique index must exist, got %v", challenges)
		assert.True(t, indexIsUnique(active), "the active-challenge index must be unique")
		_, hasFilter := active["partialFilterExpression"]
		assert.True(t, hasFilter, "the unique index must be partial on the consumed marker")
	})

	t.Run("Happy path: the expiry TTL index exists with expireAfterSeconds 0", func(t *testing.T) {
		ttl, ok := challenges["auth_challenges_expires_at_ttl"]
		require.True(t, ok, "the expiry TTL index must exist, got %v", challenges)
		assert.Equal(t, int32(0), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Happy path: the notice claim TTL index retires claims after a week", func(t *testing.T) {
		notices := listIndexes(t, mdb, "email_notices")
		ttl, ok := notices["email_notices_claimed_at_ttl"]
		require.True(t, ok, "the notice TTL index must exist, got %v", notices)
		assert.Equal(t, int32(7*24*60*60), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Happy path: the outbox created_at index exists", func(t *testing.T) {
		outbox := listIndexes(t, mdb, "email_outbox")
		_, ok := outbox["email_outbox_created_at"]
		assert.True(t, ok, "the outbox created_at index must exist, got %v", outbox)
	})

	t.Run("Edge: a second active row for one account and purpose is rejected", func(t *testing.T) {
		coll := mdb.Collection("auth_challenges")
		row := bson.M{
			"user_id":     "u1",
			"purpose":     "login_otp",
			"consumed_at": nil,
			"expires_at":  time.Now().Add(time.Hour),
		}
		row["_id"] = "c1"
		_, err := coll.InsertOne(ctx, row)
		require.NoError(t, err)

		row["_id"] = "c2"
		_, err = coll.InsertOne(ctx, row)
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "a second active row must be a duplicate key error")
	})

	t.Run("Edge: a consumed row is admitted alongside a new active one", func(t *testing.T) {
		coll := mdb.Collection("auth_challenges")
		_, err := coll.InsertOne(ctx, bson.M{
			"_id":         "c3",
			"user_id":     "u2",
			"purpose":     "login_otp",
			"consumed_at": time.Now(),
			"expires_at":  time.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		_, err = coll.InsertOne(ctx, bson.M{
			"_id":         "c4",
			"user_id":     "u2",
			"purpose":     "login_otp",
			"consumed_at": nil,
			"expires_at":  time.Now().Add(time.Hour),
		})
		require.NoError(t, err, "a consumed row must not block a new active one")
	})
}

func TestEnsureIndexes_GenerationUsage(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	usage := listIndexes(t, mdb, "generation_usage")

	t.Run("Happy path: the user and day index is unique", func(t *testing.T) {
		idx, ok := usage["generation_usage_user_day_unique"]
		require.True(t, ok, "the user and day unique index must exist, got %v", usage)
		assert.True(t, indexIsUnique(idx))
	})

	t.Run("Happy path: the expiry TTL index has expireAfterSeconds 0", func(t *testing.T) {
		ttl, ok := usage["generation_usage_expires_at_ttl"]
		require.True(t, ok, "the expiry TTL index must exist, got %v", usage)
		assert.Equal(t, int32(0), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Edge: a second counter row for the same user and day is rejected", func(t *testing.T) {
		coll := mdb.Collection("generation_usage")
		_, err := coll.InsertOne(ctx, bson.M{"user_id": "u-1", "day_utc": "2026-11-14", "count": 1})
		require.NoError(t, err)

		_, err = coll.InsertOne(ctx, bson.M{"user_id": "u-1", "day_utc": "2026-11-14", "count": 1})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err))
	})
}
