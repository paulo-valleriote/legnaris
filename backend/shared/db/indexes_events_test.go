package db_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/shared/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestEnsureIndexes_EventOutbox covers the event_outbox indexes: the claim index
// and the published-row TTL that follows the configured retention window.
func TestEnsureIndexes_EventOutbox(t *testing.T) {
	mdb := newIndexesTestDatabase(t)
	const retention = 24 * time.Hour
	require.NoError(t, db.EnsureIndexesWithRetention(context.Background(), mdb, retention))

	outbox := listIndexes(t, mdb, "event_outbox")

	t.Run("Happy path: the claim index covers status and next attempt", func(t *testing.T) {
		claim, ok := outbox["event_outbox_status_next_attempt"]
		require.True(t, ok, "the claim index must exist, got %v", outbox)
		assert.Equal(t, bson.D{{Key: "status", Value: int32(1)}, {Key: "next_attempt_at", Value: int32(1)}}, claim["key"])
	})

	t.Run("Happy path: the published TTL follows the configured retention window", func(t *testing.T) {
		ttl, ok := outbox["event_outbox_published_at_ttl"]
		require.True(t, ok, "the published TTL index must exist, got %v", outbox)
		assert.Equal(t, int32(retention.Seconds()), indexExpireAfterSeconds(t, ttl))
	})
}
