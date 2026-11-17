package plan_test

import (
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
)

func TestParseSheetKind(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   plan.SheetKind
		wantOK bool
	}{
		{name: "Happy path: character", input: "character", want: plan.KindCharacter, wantOK: true},
		{name: "Happy path: npc", input: "npc", want: plan.KindNPC, wantOK: true},
		{name: "Happy path: monster", input: "monster", want: plan.KindMonster, wantOK: true},
		{name: "Edge: empty kind is invalid", input: "", wantOK: false},
		{name: "Edge: wrong case is invalid", input: "Character", wantOK: false},
		{name: "Edge: surrounding spaces are invalid", input: " npc ", wantOK: false},
		{name: "Error: unknown kind is invalid", input: "vehicle", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := plan.ParseSheetKind(tc.input)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantID string
	}{
		{name: "Happy path: free resolves to Free", id: "free", wantID: "free"},
		{name: "Happy path: paid resolves to Paid", id: "paid", wantID: "paid"},
		{name: "Edge: missing plan field reads as Free", id: "", wantID: "free"},
		{name: "Edge: unknown plan falls back to Free", id: "enterprise", wantID: "free"},
		{name: "Edge: case mismatch falls back to Free", id: "PAID", wantID: "free"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantID, plan.Lookup(tc.id).ID)
		})
	}
}

func TestSheetLimit(t *testing.T) {
	tests := []struct {
		name  string
		plan  plan.Plan
		kind  plan.SheetKind
		limit int
	}{
		{name: "Happy path: Free allows one character", plan: plan.Free, kind: plan.KindCharacter, limit: 1},
		{name: "Happy path: Free allows two NPCs", plan: plan.Free, kind: plan.KindNPC, limit: 2},
		{name: "Happy path: Free allows two monsters", plan: plan.Free, kind: plan.KindMonster, limit: 2},
		{name: "Happy path: Paid allows ten characters", plan: plan.Paid, kind: plan.KindCharacter, limit: 10},
		{name: "Happy path: Paid allows fifty NPCs", plan: plan.Paid, kind: plan.KindNPC, limit: 50},
		{name: "Happy path: Paid allows fifty monsters", plan: plan.Paid, kind: plan.KindMonster, limit: 50},
		{name: "Edge: unknown kind has no allowance", plan: plan.Paid, kind: plan.SheetKind("vehicle"), limit: 0},
		{name: "Edge: empty kind has no allowance", plan: plan.Free, kind: plan.SheetKind(""), limit: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.limit, tc.plan.SheetLimit(tc.kind))
		})
	}
}

func TestSheetKindsCoverEveryLimit(t *testing.T) {
	for _, kind := range plan.SheetKinds() {
		_, ok := plan.ParseSheetKind(string(kind))
		assert.True(t, ok, "every listed kind parses back: %s", kind)
		assert.Positive(t, plan.Free.SheetLimit(kind), "Free allows %s", kind)
		assert.Positive(t, plan.Paid.SheetLimit(kind), "Paid allows %s", kind)
	}
}

func TestOwnedCampaignLimit(t *testing.T) {
	tests := []struct {
		name  string
		plan  plan.Plan
		limit int
	}{
		{name: "Happy path: Free allows one owned Campaign", plan: plan.Free, limit: 1},
		{name: "Happy path: Paid allows five owned Campaigns", plan: plan.Paid, limit: 5},
		{name: "Edge: zero-value Plan allows no owned Campaign", plan: plan.Plan{}, limit: 0},
		{name: "Edge: Lookup of unknown plan yields the Free limit", plan: plan.Lookup("enterprise"), limit: 1},
		{name: "Edge: Lookup of missing plan yields the Free limit", plan: plan.Lookup(""), limit: 1},
		{name: "Edge: Lookup of paid yields the Paid limit", plan: plan.Lookup(plan.IDPaid), limit: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.limit, tc.plan.OwnedCampaignLimit())
		})
	}
}
