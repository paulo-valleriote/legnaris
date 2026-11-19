package events_test

import (
	"os"
	"testing"
	"time"

	"legnaris/backend/internal/events"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEventID    = "0190a5f0-0000-7000-8000-000000000001"
	testActorID    = "0190a5f0-0000-7000-8000-000000000002"
	testSheetID    = "0190a5f0-0000-7000-8000-000000000003"
	goldenEnvelope = "testdata/sheet_updated.golden.json"
)

var testInstant = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func validEvent() events.Event {
	actor := testActorID
	return events.Event{
		Type:             "sheet.updated",
		ActorID:          &actor,
		AggregateType:    "sheet",
		AggregateID:      testSheetID,
		AggregateVersion: 42,
		Data: map[string]any{
			"changed_keys":  []string{"hp", "dice"},
			"sheet_version": 42,
		},
	}
}

func TestEncode(t *testing.T) {
	nullActor := (*string)(nil)

	tests := []struct {
		name    string
		mutate  func(ev *events.Event)
		wantErr error
	}{
		{name: "Happy path: valid event encodes", mutate: func(ev *events.Event) {}},
		{name: "Happy path: system action with null actor encodes", mutate: func(ev *events.Event) { ev.ActorID = nullActor }},
		{name: "Edge: empty type is rejected", mutate: func(ev *events.Event) { ev.Type = "" }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: type without dot is rejected", mutate: func(ev *events.Event) { ev.Type = "sheetupdated" }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: uppercase type is rejected", mutate: func(ev *events.Event) { ev.Type = "Sheet.Updated" }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: empty aggregate type is rejected", mutate: func(ev *events.Event) { ev.AggregateType = "" }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: non-UUID aggregate id is rejected", mutate: func(ev *events.Event) { ev.AggregateID = "sheet-1" }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: non-UUID actor id is rejected", mutate: func(ev *events.Event) { bad := "me"; ev.ActorID = &bad }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: negative aggregate version is rejected", mutate: func(ev *events.Event) { ev.AggregateVersion = -1 }, wantErr: events.ErrInvalidEvent},
		{name: "Edge: unmarshalable data is rejected", mutate: func(ev *events.Event) { ev.Data = make(chan int) }, wantErr: events.ErrInvalidEvent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := validEvent()
			tc.mutate(&ev)

			body, err := events.Encode(ev, testEventID, testInstant)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, body)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, body)
		})
	}
}

func TestEncodeGoldenEnvelope(t *testing.T) {
	body, err := events.Encode(validEvent(), testEventID, testInstant)
	require.NoError(t, err)

	want, err := os.ReadFile(goldenEnvelope)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(body))
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantErr  bool
		wantType string
		wantVer  int64
	}{
		{name: "Happy path: envelope decodes", body: `{"id":"` + testEventID + `","type":"sheet.updated","version":1,"occurred_at":"2026-11-13T12:00:00Z","actor_id":null,"aggregate":{"type":"sheet","id":"` + testSheetID + `","version":42},"data":{}}`, wantType: "sheet.updated", wantVer: 42},
		{name: "Edge: unknown fields are tolerated", body: `{"id":"` + testEventID + `","type":"sheet.updated","version":1,"future":true,"aggregate":{"type":"sheet","id":"` + testSheetID + `","version":1,"extra":"x"},"data":{}}`, wantType: "sheet.updated", wantVer: 1},
		{name: "Edge: missing aggregate version decodes as zero", body: `{"id":"` + testEventID + `","type":"sheet.deleted","version":1,"aggregate":{"type":"sheet","id":"` + testSheetID + `"},"data":{}}`, wantType: "sheet.deleted", wantVer: 0},
		{name: "Error: malformed JSON is rejected", body: `{"id":`, wantErr: true},
		{name: "Edge: empty body is rejected", body: ``, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, err := events.Decode([]byte(tc.body))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantType, env.Type)
			assert.Equal(t, tc.wantVer, env.Aggregate.Version)
		})
	}
}

func TestNoOpPublisher(t *testing.T) {
	tests := []struct {
		name string
		ev   events.Event
	}{
		{name: "Happy path: valid event is accepted silently", ev: validEvent()},
		{name: "Edge: invalid event is accepted silently", ev: events.Event{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() { events.NoOp{}.Publish(tc.ev) })
		})
	}
}
