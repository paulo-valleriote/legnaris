package outbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	legacyMonsterKind  = "monster"
	rewrittenEnemyKind = "enemy"
)

var sheetKindEventTypes = []string{
	campaign.EventSheetLinked,
	campaign.EventSheetUnlinked,
	campaign.EventSheetRevealed,
	campaign.EventSheetHidden,
}

func newKindEntry(t *testing.T, eventType, kind string, now time.Time) outbox.Entry {
	t.Helper()
	entry, err := outbox.NewEntry(events.Event{
		Type:          eventType,
		AggregateType: "campaign",
		AggregateID:   newUUIDString(t),
		Data: map[string]any{
			"sheet_id": newUUIDString(t),
			"kind":     kind,
			"owner_id": newUUIDString(t),
		},
	}, now)
	require.NoError(t, err)
	return entry
}

func decodedDataKind(t *testing.T, body []byte) string {
	t.Helper()
	envelope, err := events.Decode(body)
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	kind, ok := data["kind"].(string)
	require.True(t, ok, "data.kind must be a string")
	return kind
}

func TestRewritePendingDataKind(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: a pending campaign.sheet_linked entry is rewritten to the new kind", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		entry := newKindEntry(t, campaign.EventSheetLinked, legacyMonsterKind, now)
		require.NoError(t, repo.Insert(ctx, entry))

		rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rewritten)

		stored := readEntry(t, mdb, entry.ID)
		assert.Equal(t, outbox.StatusPending, stored.Status)
		assert.Equal(t, rewrittenEnemyKind, decodedDataKind(t, stored.Body))

		original, err := events.Decode(entry.Body)
		require.NoError(t, err)
		current, err := events.Decode(stored.Body)
		require.NoError(t, err)
		assert.Equal(t, original.Type, current.Type)
		assert.Equal(t, original.ID, current.ID)
		assert.Equal(t, original.Aggregate, current.Aggregate)
		assert.True(t, original.OccurredAt.Equal(current.OccurredAt))

		var before, after map[string]any
		require.NoError(t, json.Unmarshal(original.Data, &before))
		require.NoError(t, json.Unmarshal(current.Data, &after))
		assert.Equal(t, before["sheet_id"], after["sheet_id"])
		assert.Equal(t, before["owner_id"], after["owner_id"])
	})

	t.Run("Edge: published and dead entries are never touched", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		published := newKindEntry(t, campaign.EventSheetLinked, legacyMonsterKind, now)
		dead := newKindEntry(t, campaign.EventSheetLinked, legacyMonsterKind, now)
		require.NoError(t, repo.Insert(ctx, published))
		require.NoError(t, repo.Insert(ctx, dead))
		require.NoError(t, repo.MarkPublished(ctx, published.ID, now))
		require.NoError(t, repo.MarkTerminal(ctx, dead.ID, now, "attempt cap"))

		rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		assert.Zero(t, rewritten)

		publishedStored := readEntry(t, mdb, published.ID)
		assert.Equal(t, published.Body, publishedStored.Body)
		assert.Equal(t, legacyMonsterKind, decodedDataKind(t, publishedStored.Body))
		deadStored := readEntry(t, mdb, dead.ID)
		assert.Equal(t, dead.Body, deadStored.Body)
		assert.Equal(t, legacyMonsterKind, decodedDataKind(t, deadStored.Body))
	})

	t.Run("Edge: an entry of another event type is not touched", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		other := newKindEntry(t, "user.deleted", legacyMonsterKind, now)
		require.NoError(t, repo.Insert(ctx, other))

		rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		assert.Zero(t, rewritten)
		assert.Equal(t, other.Body, readEntry(t, mdb, other.ID).Body)
	})

	t.Run("Edge: a pending entry with another kind is not touched", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		npc := newKindEntry(t, campaign.EventSheetLinked, "npc", now)
		require.NoError(t, repo.Insert(ctx, npc))

		rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		assert.Zero(t, rewritten)
		assert.Equal(t, npc.Body, readEntry(t, mdb, npc.ID).Body)
	})

	t.Run("Edge: an empty event type list rewrites nothing", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		entry := newKindEntry(t, campaign.EventSheetLinked, legacyMonsterKind, now)
		require.NoError(t, repo.Insert(ctx, entry))

		rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, nil, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		assert.Zero(t, rewritten)
		assert.Equal(t, entry.Body, readEntry(t, mdb, entry.ID).Body)
	})

	t.Run("Edge: a second run rewrites nothing", func(t *testing.T) {
		mdb, repo := newOutboxFixture(t)
		entry := newKindEntry(t, campaign.EventSheetLinked, legacyMonsterKind, now)
		require.NoError(t, repo.Insert(ctx, entry))

		first, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)
		second, err := outbox.RewritePendingDataKind(ctx, mdb, sheetKindEventTypes, legacyMonsterKind, rewrittenEnemyKind)
		require.NoError(t, err)

		assert.Equal(t, int64(1), first)
		assert.Zero(t, second)
		assert.Equal(t, rewrittenEnemyKind, decodedDataKind(t, readEntry(t, mdb, entry.ID).Body))
	})
}
