package db_test

import (
	"context"
	"testing"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestEnsureIndexes_Generations(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	gens := listIndexes(t, mdb, "generations")

	t.Run("Happy path: the active user index is unique", func(t *testing.T) {
		idx, ok := gens["generations_active_user_id_unique"]
		require.True(t, ok, "the active user unique index must exist, got %v", gens)
		assert.True(t, indexIsUnique(idx))
	})

	t.Run("Edge: documents without an active user never collide", func(t *testing.T) {
		coll := mdb.Collection("generations")
		_, err := coll.InsertOne(ctx, bson.M{"_id": "g-1", "user_id": "u-1", "status": "succeeded"})
		require.NoError(t, err)
		_, err = coll.InsertOne(ctx, bson.M{"_id": "g-2", "user_id": "u-1", "status": "succeeded"})
		require.NoError(t, err)
	})

	t.Run("Error: a second in-progress document for one player is rejected", func(t *testing.T) {
		coll := mdb.Collection("generations")
		_, err := coll.InsertOne(ctx, bson.M{"_id": "g-3", "user_id": "u-2", "active_user_id": "u-2", "status": "queued"})
		require.NoError(t, err)
		_, err = coll.InsertOne(ctx, bson.M{"_id": "g-4", "user_id": "u-2", "active_user_id": "u-2", "status": "queued"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err))
	})

	t.Run("Happy path: the generation expiry TTL has expireAfterSeconds 0", func(t *testing.T) {
		ttl, ok := gens["generations_expires_at_ttl"]
		require.True(t, ok, "the expiry TTL index must exist, got %v", gens)
		assert.Equal(t, int32(0), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Happy path: the cap counter collection carries its own expiry TTL", func(t *testing.T) {
		caps := listIndexes(t, mdb, "generation_cap")
		ttl, ok := caps["generation_cap_expires_at_ttl"]
		require.True(t, ok, "the cap counter TTL must exist, got %v", caps)
		assert.Equal(t, int32(0), indexExpireAfterSeconds(t, ttl))
	})
}
