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

func TestEnsureIndexes_Subscriptions(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	require.NoError(t, db.EnsureIndexes(ctx, mdb))

	subs := listIndexes(t, mdb, "subscriptions")

	t.Run("Happy path: the live subscription index is unique per user", func(t *testing.T) {
		idx, ok := subs["subscriptions_user_live_unique"]
		require.True(t, ok, "the live subscription index must exist, got %v", subs)
		assert.True(t, indexIsUnique(idx))
	})

	coll := mdb.Collection("subscriptions")
	insert := func(id, status string) error {
		_, err := coll.InsertOne(ctx, bson.M{"_id": id, "user_id": "u-1", "status": status})
		return err
	}

	t.Run("Error: a second pending row for one user is rejected", func(t *testing.T) {
		require.NoError(t, insert("s-1", "pending"))
		err := insert("s-2", "pending")
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err))
	})

	t.Run("Edge: an active row alongside a canceled row never collides", func(t *testing.T) {
		coll2 := mdb.Collection("subscriptions")
		_, err := coll2.InsertOne(ctx, bson.M{"_id": "s-3", "user_id": "u-2", "status": "canceled"})
		require.NoError(t, err)
		_, err = coll2.InsertOne(ctx, bson.M{"_id": "s-4", "user_id": "u-2", "status": "active"})
		require.NoError(t, err)
	})

	t.Run("Error: an active row blocks a second pending row for the same user", func(t *testing.T) {
		_, err := coll.InsertOne(ctx, bson.M{"_id": "s-5", "user_id": "u-3", "status": "active"})
		require.NoError(t, err)
		_, err = coll.InsertOne(ctx, bson.M{"_id": "s-6", "user_id": "u-3", "status": "pending"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err))
	})
}
