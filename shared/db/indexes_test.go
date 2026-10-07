package db_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const indexesTestDatabase = "legnaris_indexes_test"

// newIndexesTestDatabase connects to MongoDB and gives the test a clean
// database. It skips when MongoDB is unavailable so unit runs stay hermetic.
func newIndexesTestDatabase(t *testing.T) *mongo.Database {
	t.Helper()

	log := logger.NewLogger(slog.LevelError)
	cfg := env.NewEnv(log)

	database, err := db.InitMongo(cfg)
	if err != nil {
		t.Skipf("mongo unavailable — skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
		defer cancel()
		_ = database.Close(closeCtx)
	})

	mdb := database.Client.Database(indexesTestDatabase)
	_ = mdb.Collection("users").Drop(context.Background())
	_ = mdb.Collection("refresh_tokens").Drop(context.Background())
	_ = mdb.Collection("sheets").Drop(context.Background())
	t.Cleanup(func() {
		_ = mdb.Collection("users").Drop(context.Background())
		_ = mdb.Collection("refresh_tokens").Drop(context.Background())
		_ = mdb.Collection("sheets").Drop(context.Background())
	})

	return mdb
}

// listIndexes reads a collection's indexes once into a name-keyed map. A cursor
// can only be consumed once, so every assertion reads from this map.
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
		ownerID, ok := sheets["sheets_owner_id_unique"]
		require.True(t, ok, "sheets.owner_id unique index must exist, got %v", sheets)
		assert.True(t, indexIsUnique(ownerID), "sheets.owner_id index must be unique")
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

	t.Run("Edge: unique sheets.owner_id index rejects a second sheet for the owner", func(t *testing.T) {
		sheets := mdb.Collection("sheets")
		_, err := sheets.InsertOne(ctx, bson.M{"_id": "sheet-1", "owner_id": "owner-1"})
		require.NoError(t, err)

		_, err = sheets.InsertOne(ctx, bson.M{"_id": "sheet-2", "owner_id": "owner-1"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "a second sheet for the owner must be a duplicate key error")
	})
}
