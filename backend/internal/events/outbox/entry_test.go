package outbox_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/shared/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleEvent(t *testing.T, eventType, aggregateID string) events.Event {
	t.Helper()
	return events.Event{
		Type:          eventType,
		AggregateType: "user",
		AggregateID:   aggregateID,
		Data:          map[string]string{"changed": "email"},
	}
}

func newUUID(t *testing.T) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	return id
}

func TestNewEntry(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	aggregateID := newUUID(t)

	t.Run("Happy path: a valid event becomes a pending entry with a canonical identity", func(t *testing.T) {
		entry, err := outbox.NewEntry(sampleEvent(t, "user.deleted", aggregateID), now)
		require.NoError(t, err)

		assert.NoError(t, idgen.Validate(entry.ID))
		assert.Equal(t, outbox.StatusPending, entry.Status)
		assert.Equal(t, "user.deleted", entry.EventType)
		assert.Equal(t, 0, entry.Attempts)
		assert.Nil(t, entry.LockedUntil)
		assert.Nil(t, entry.PublishedAt)
		assert.True(t, entry.NextAttemptAt.Equal(now))
		assert.True(t, entry.CreatedAt.Equal(now))
	})

	t.Run("Happy path: the stored body is the envelope carrying the entry identity", func(t *testing.T) {
		entry, err := outbox.NewEntry(sampleEvent(t, "user.deleted", aggregateID), now)
		require.NoError(t, err)

		env, err := events.Decode(entry.Body)
		require.NoError(t, err)
		assert.Equal(t, entry.ID, env.ID, "the envelope id is the dedupe key and must match the row")
		assert.Equal(t, "user.deleted", env.Type)
		assert.Equal(t, aggregateID, env.Aggregate.ID)
		assert.True(t, env.OccurredAt.Equal(now))
	})

	t.Run("Edge: two entries for the same event get distinct identities and bodies", func(t *testing.T) {
		ev := sampleEvent(t, "user.deleted", aggregateID)
		first, err := outbox.NewEntry(ev, now)
		require.NoError(t, err)
		second, err := outbox.NewEntry(ev, now)
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)
		assert.NotEqual(t, first.Body, second.Body)
	})

	tests := []struct {
		name      string
		event     events.Event
		wantError error
	}{
		{
			name:      "Error: a type outside the naming pattern is rejected",
			event:     events.Event{Type: "UserDeleted", AggregateType: "user", AggregateID: aggregateID},
			wantError: events.ErrInvalidEvent,
		},
		{
			name:      "Error: an empty type is rejected",
			event:     events.Event{Type: "", AggregateType: "user", AggregateID: aggregateID},
			wantError: events.ErrInvalidEvent,
		},
		{
			name:      "Error: a non-UUID aggregate id is rejected",
			event:     events.Event{Type: "user.deleted", AggregateType: "user", AggregateID: "not-a-uuid"},
			wantError: events.ErrInvalidEvent,
		},
		{
			name:      "Edge: an empty aggregate id is rejected",
			event:     events.Event{Type: "user.deleted", AggregateType: "user", AggregateID: ""},
			wantError: events.ErrInvalidEvent,
		},
		{
			name:      "Edge: a negative aggregate version is rejected",
			event:     events.Event{Type: "user.deleted", AggregateType: "user", AggregateID: aggregateID, AggregateVersion: -1},
			wantError: events.ErrInvalidEvent,
		},
		{
			name:      "Edge: an empty aggregate type is rejected",
			event:     events.Event{Type: "user.deleted", AggregateType: "", AggregateID: aggregateID},
			wantError: events.ErrInvalidEvent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := outbox.NewEntry(tc.event, now)
			assert.True(t, errors.Is(err, tc.wantError), "want %v, got %v", tc.wantError, err)
		})
	}
}

func TestNewEntryBodyIsJSON(t *testing.T) {
	entry, err := outbox.NewEntry(sampleEvent(t, "user.deleted", newUUID(t)), time.Now())
	require.NoError(t, err)
	assert.True(t, json.Valid(entry.Body))
}
