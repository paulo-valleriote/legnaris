package template

import (
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptsKind(t *testing.T) {
	tests := []struct {
		name  string
		kinds []plan.SheetKind
		kind  plan.SheetKind
		want  bool
	}{
		{name: "Happy path: declared kind is accepted", kinds: []plan.SheetKind{plan.KindCharacter}, kind: plan.KindCharacter, want: true},
		{name: "Happy path: any of several declared kinds is accepted", kinds: []plan.SheetKind{plan.KindCharacter, plan.KindNPC}, kind: plan.KindNPC, want: true},
		{name: "Edge: undeclared kind is refused", kinds: []plan.SheetKind{plan.KindCharacter}, kind: plan.KindEnemy, want: false},
		{name: "Edge: template without kinds accepts nothing", kinds: nil, kind: plan.KindCharacter, want: false},
		{name: "Edge: empty kind is refused", kinds: []plan.SheetKind{plan.KindCharacter}, kind: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := Template{Kinds: tc.kinds}
			assert.Equal(t, tc.want, tmpl.AcceptsKind(tc.kind))
		})
	}
}

func TestFantasyTemplateAcceptsEverySheetKind(t *testing.T) {
	fantasy := fifthTemplateByID(t, "fantasy_v1")
	for _, kind := range plan.SheetKinds() {
		assert.True(t, fantasy.AcceptsKind(kind), "the fantasy template accepts %s sheets", kind)
	}
}

func TestDefaultTemplateAcceptsCharacterSheets(t *testing.T) {
	templates := DefaultTemplates()
	defaults := make([]Template, 0, 1)
	for _, tmpl := range templates {
		if tmpl.Default {
			defaults = append(defaults, tmpl)
		}
	}
	require.Len(t, defaults, 1)
	assert.True(t, defaults[0].AcceptsKind(plan.KindCharacter))
	assert.False(t, defaults[0].AcceptsKind(plan.KindNPC))
	assert.False(t, defaults[0].AcceptsKind(plan.KindEnemy))
}

func TestGeneratableFor(t *testing.T) {
	tests := []struct {
		name             string
		generatable      bool
		generatableKinds []plan.SheetKind
		kind             plan.SheetKind
		want             bool
	}{
		{
			name:        "Happy path: a leaf with no restriction is generatable for every kind",
			generatable: true,
			kind:        plan.KindEnemy,
			want:        true,
		},
		{
			name:             "Happy path: a restricted leaf is generatable for its declared kind",
			generatable:      true,
			generatableKinds: []plan.SheetKind{plan.KindCharacter},
			kind:             plan.KindCharacter,
			want:             true,
		},
		{
			name:             "Edge: a restricted leaf is not generatable for another declared kind",
			generatable:      true,
			generatableKinds: []plan.SheetKind{plan.KindCharacter},
			kind:             plan.KindNPC,
			want:             false,
		},
		{
			name:             "Edge: an unknown kind does not match a restriction",
			generatable:      true,
			generatableKinds: []plan.SheetKind{plan.KindCharacter},
			kind:             plan.SheetKind("dragon"),
			want:             false,
		},
		{
			name:             "Edge: an empty kind is generatable when no kind is restricted",
			generatable:      true,
			generatableKinds: nil,
			kind:             "",
			want:             true,
		},
		{
			name:             "Edge: a non-generatable leaf is never generatable",
			generatable:      false,
			generatableKinds: []plan.SheetKind{plan.KindCharacter},
			kind:             plan.KindCharacter,
			want:             false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := FieldSchema{Generatable: tc.generatable, GeneratableKinds: tc.generatableKinds}
			assert.Equal(t, tc.want, schema.GeneratableFor(tc.kind))
		})
	}
}
