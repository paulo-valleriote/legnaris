package outbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	legacyMonsterKind  = "monster"
	rewrittenEnemyKind = "enemy"
)

const (
	eventSheetLinked   = "campaign.sheet_linked"
	eventSheetUnlinked = "campaign.sheet_unlinked"
	eventSheetRevealed = "campaign.sheet_revealed"
	eventSheetHidden   = "campaign.sheet_hidden"
	eventUserDeleted   = "user.deleted"
)

var sheetKindEventTypes = []string{
	eventSheetLinked,
	eventSheetUnlinked,
	eventSheetRevealed,
	eventSheetHidden,
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

func assertRewritten(t *testing.T, mdb *mongo.Database, original outbox.Entry) {
	t.Helper()

	stored := readEntry(t, mdb, original.ID)
	assert.Equal(t, outbox.StatusPending, stored.Status)
	assert.Equal(t, rewrittenEnemyKind, decodedDataKind(t, stored.Body))

	originalEvent, err := events.Decode(original.Body)
	require.NoError(t, err)
	current, err := events.Decode(stored.Body)
	require.NoError(t, err)
	assert.Equal(t, originalEvent.Type, current.Type)
	assert.Equal(t, originalEvent.ID, current.ID)
	assert.Equal(t, originalEvent.Aggregate, current.Aggregate)
	assert.True(t, originalEvent.OccurredAt.Equal(current.OccurredAt))

	var before, after map[string]any
	require.NoError(t, json.Unmarshal(originalEvent.Data, &before))
	require.NoError(t, json.Unmarshal(current.Data, &after))
	assert.Equal(t, before["sheet_id"], after["sheet_id"])
	assert.Equal(t, before["owner_id"], after["owner_id"])
}

func assertBodyUntouched(t *testing.T, mdb *mongo.Database, original outbox.Entry) {
	t.Helper()
	assert.Equal(t, original.Body, readEntry(t, mdb, original.ID).Body)
}

type rewriteCase struct {
	name       string
	seed       func(t *testing.T, mdb *mongo.Database, repo outbox.Repository) []outbox.Entry
	eventTypes []string
	from       string
	to         string
	at         time.Time
	wantRuns   []int64
	wantErrIs  error
	verify     func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry)
}

func TestRewritePendingDataKind(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	seedSingle := func(eventType string) func(*testing.T, *mongo.Database, outbox.Repository) []outbox.Entry {
		return func(t *testing.T, _ *mongo.Database, repo outbox.Repository) []outbox.Entry {
			entry := newKindEntry(t, eventType, legacyMonsterKind, now)
			require.NoError(t, repo.Insert(ctx, entry))
			return []outbox.Entry{entry}
		}
	}

	tests := []rewriteCase{
		{
			name:       "Happy path: a pending campaign.sheet_linked entry is rewritten to the new kind",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{1},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name:       "Happy path: a pending campaign.sheet_unlinked entry is rewritten to the new kind",
			seed:       seedSingle(eventSheetUnlinked),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{1},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name:       "Happy path: a pending campaign.sheet_revealed entry is rewritten to the new kind",
			seed:       seedSingle(eventSheetRevealed),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{1},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name:       "Happy path: a pending campaign.sheet_hidden entry is rewritten to the new kind",
			seed:       seedSingle(eventSheetHidden),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{1},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name:       "Error: an empty event type list is an invalid rewrite",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: nil,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantErrIs:  outbox.ErrInvalidRewrite,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name:       "Error: an empty source kind is an invalid rewrite",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: sheetKindEventTypes,
			from:       "",
			to:         rewrittenEnemyKind,
			at:         now,
			wantErrIs:  outbox.ErrInvalidRewrite,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name:       "Error: an empty target kind is an invalid rewrite",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         "",
			at:         now,
			wantErrIs:  outbox.ErrInvalidRewrite,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name:       "Error: equal source and target kinds are an invalid rewrite",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         legacyMonsterKind,
			at:         now,
			wantErrIs:  outbox.ErrInvalidRewrite,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name: "Error: a malformed body fails the rewrite with ErrInvalidEntry",
			seed: func(t *testing.T, _ *mongo.Database, repo outbox.Repository) []outbox.Entry {
				entry := outbox.Entry{
					ID:            newUUIDString(t),
					EventType:     eventSheetLinked,
					Body:          []byte("not json"),
					Status:        outbox.StatusPending,
					NextAttemptAt: now,
					CreatedAt:     now,
				}
				require.NoError(t, repo.Insert(ctx, entry))
				return []outbox.Entry{entry}
			},
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantErrIs:  outbox.ErrInvalidEntry,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name: "Error: a live relay lease refuses the rewrite",
			seed: func(t *testing.T, _ *mongo.Database, repo outbox.Repository) []outbox.Entry {
				entry := newKindEntry(t, eventSheetLinked, legacyMonsterKind, now)
				require.NoError(t, repo.Insert(ctx, entry))
				claimed, err := repo.Claim(ctx, "relay-a", now, time.Minute, 1)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				return []outbox.Entry{entry}
			},
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantErrIs:  outbox.ErrOutboxLeased,
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertBodyUntouched(t, mdb, seeded[0])
			},
		},
		{
			name: "Edge: an expired relay lease is rewritten",
			seed: func(t *testing.T, _ *mongo.Database, repo outbox.Repository) []outbox.Entry {
				entry := newKindEntry(t, eventSheetLinked, legacyMonsterKind, now)
				require.NoError(t, repo.Insert(ctx, entry))
				claimed, err := repo.Claim(ctx, "relay-a", now, time.Minute, 1)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				return []outbox.Entry{entry}
			},
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now.Add(2 * time.Minute),
			wantRuns:   []int64{1},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name: "Edge: published, dead, other type and other kind entries are untouched",
			seed: func(t *testing.T, _ *mongo.Database, repo outbox.Repository) []outbox.Entry {
				published := newKindEntry(t, eventSheetLinked, legacyMonsterKind, now)
				dead := newKindEntry(t, eventSheetLinked, legacyMonsterKind, now)
				otherType := newKindEntry(t, eventUserDeleted, legacyMonsterKind, now)
				otherKind := newKindEntry(t, eventSheetLinked, "npc", now)
				require.NoError(t, repo.Insert(ctx, published))
				require.NoError(t, repo.Insert(ctx, dead))
				require.NoError(t, repo.Insert(ctx, otherType))
				require.NoError(t, repo.Insert(ctx, otherKind))
				require.NoError(t, repo.MarkPublished(ctx, published.ID, now))
				require.NoError(t, repo.MarkTerminal(ctx, dead.ID, now, "attempt cap"))
				return []outbox.Entry{published, dead, otherType, otherKind}
			},
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{0},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				for _, entry := range seeded {
					assertBodyUntouched(t, mdb, entry)
				}
				assert.Equal(t, legacyMonsterKind, decodedDataKind(t, readEntry(t, mdb, seeded[2].ID).Body))
				assert.Equal(t, "npc", decodedDataKind(t, readEntry(t, mdb, seeded[3].ID).Body))
			},
		},
		{
			name:       "Edge: a second run rewrites nothing",
			seed:       seedSingle(eventSheetLinked),
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{1, 0},
			verify: func(t *testing.T, mdb *mongo.Database, seeded []outbox.Entry) {
				assertRewritten(t, mdb, seeded[0])
			},
		},
		{
			name:       "Edge: an empty outbox reports zero rewrites",
			seed:       func(*testing.T, *mongo.Database, outbox.Repository) []outbox.Entry { return nil },
			eventTypes: sheetKindEventTypes,
			from:       legacyMonsterKind,
			to:         rewrittenEnemyKind,
			at:         now,
			wantRuns:   []int64{0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mdb, repo := newOutboxFixture(t)
			seeded := tc.seed(t, mdb, repo)

			if tc.wantErrIs != nil {
				rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, tc.eventTypes, tc.from, tc.to, tc.at)
				require.ErrorIs(t, err, tc.wantErrIs)
				assert.Zero(t, rewritten)
			} else {
				for run, want := range tc.wantRuns {
					rewritten, err := outbox.RewritePendingDataKind(ctx, mdb, tc.eventTypes, tc.from, tc.to, tc.at)
					require.NoErrorf(t, err, "run %d", run+1)
					assert.Equalf(t, want, rewritten, "run %d", run+1)
				}
			}

			if tc.verify != nil {
				tc.verify(t, mdb, seeded)
			}
		})
	}
}
