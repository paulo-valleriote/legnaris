package db_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// indexKeyNames returns the field names of an index in declared order. The
// order is part of the contract: the claim query relies on status leading.
func indexKeyNames(t *testing.T, mdb *mongo.Database, collection, name string) []string {
	t.Helper()

	specs, err := mdb.Collection(collection).Indexes().ListSpecifications(context.Background())
	require.NoError(t, err)
	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		elements, err := spec.KeysDocument.Elements()
		require.NoError(t, err)
		names := make([]string, 0, len(elements))
		for _, element := range elements {
			names = append(names, element.Key())
		}
		return names
	}
	require.FailNow(t, "index not found", "index %q missing on %q", name, collection)
	return nil
}

// TestEnsureIndexes_EventOutbox covers the event_outbox indexes: the claim index
// and the published-row TTL that follows the configured retention window.
func TestEnsureIndexes_EventOutbox(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	ctx := context.Background()
	const retention = 24 * time.Hour
	require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, retention))

	outbox := listIndexes(t, mdb, "event_outbox")

	t.Run("Happy path: the claim index covers status and next attempt", func(t *testing.T) {
		_, ok := outbox["event_outbox_status_next_attempt"]
		require.True(t, ok, "the claim index must exist, got %v", outbox)
		assert.Equal(t, []string{"status", "next_attempt_at"},
			indexKeyNames(t, mdb, "event_outbox", "event_outbox_status_next_attempt"))
	})

	t.Run("Happy path: the published TTL follows the configured retention window", func(t *testing.T) {
		ttl, ok := outbox["event_outbox_published_at_ttl"]
		require.True(t, ok, "the published TTL index must exist, got %v", outbox)
		assert.Equal(t, int32(retention.Seconds()), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Edge: a changed retention reconciles the published TTL in place", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 48*time.Hour))

		ttl := listIndexes(t, mdb, "event_outbox")["event_outbox_published_at_ttl"]
		assert.Equal(t, int32((48 * time.Hour).Seconds()), indexExpireAfterSeconds(t, ttl))
	})

	t.Run("Edge: re-applying the same retention is idempotent", func(t *testing.T) {
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 48*time.Hour))
		require.NoError(t, db.EnsureIndexesWithRetention(ctx, mdb, 48*time.Hour))
	})
}
