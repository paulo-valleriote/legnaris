package db_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestDatabase_Ping(t *testing.T) {
	t.Run("Error: an unreachable server fails the ping", func(t *testing.T) {
		client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=50"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

		assert.Error(t, (&db.Database{Client: client}).Ping(context.Background()))
	})

	t.Run("Edge: a canceled context fails the ping without waiting", func(t *testing.T) {
		client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=5000"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		start := time.Now()
		assert.Error(t, (&db.Database{Client: client}).Ping(ctx))
		assert.Less(t, time.Since(start), time.Second)
	})
}
