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

func TestDefaultTemplateAcceptsCharactersOnly(t *testing.T) {
	templates := DefaultTemplates()
	require.NotEmpty(t, templates)
	fantasy := templates[0]
	assert.True(t, fantasy.AcceptsKind(plan.KindCharacter), "the fantasy template is a character sheet")
	assert.False(t, fantasy.AcceptsKind(plan.KindNPC), "NPC sheets need a template that declares them")
	assert.False(t, fantasy.AcceptsKind(plan.KindEnemy))
}
