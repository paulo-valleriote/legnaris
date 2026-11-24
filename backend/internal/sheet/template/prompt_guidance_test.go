package template

import (
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_GuidanceFor(t *testing.T) {
	tests := []struct {
		name string
		tmpl *Template
		kind plan.SheetKind
		want string
	}{
		{
			name: "Happy path: a kind with its own guidance uses it",
			tmpl: &Template{
				PromptGuidance:       "shared guidance",
				PromptGuidanceByKind: map[plan.SheetKind]string{plan.KindNPC: "npc guidance"},
			},
			kind: plan.KindNPC,
			want: "npc guidance",
		},
		{
			name: "Happy path: the shared guidance applies to every kind without its own",
			tmpl: &Template{PromptGuidance: "shared guidance"},
			kind: plan.KindEnemy,
			want: "shared guidance",
		},
		{
			name: "Edge: a kind without its own guidance falls back to the shared layer",
			tmpl: &Template{
				PromptGuidance:       "shared guidance",
				PromptGuidanceByKind: map[plan.SheetKind]string{plan.KindEnemy: "enemy guidance"},
			},
			kind: plan.KindNPC,
			want: "shared guidance",
		},
		{
			name: "Edge: an empty guidance entry falls back to the shared layer",
			tmpl: &Template{
				PromptGuidance:       "shared guidance",
				PromptGuidanceByKind: map[plan.SheetKind]string{plan.KindNPC: ""},
			},
			kind: plan.KindNPC,
			want: "shared guidance",
		},
		{
			name: "Edge: a template without guidance returns an empty string",
			tmpl: &Template{},
			kind: plan.KindCharacter,
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.tmpl.GuidanceFor(tc.kind))
		})
	}
}

func TestValidatePromptGuidance(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    Template
		wantErr error
	}{
		{
			name: "Happy path: guidance for the accepted kinds is valid",
			tmpl: Template{
				Kinds: []plan.SheetKind{plan.KindNPC, plan.KindEnemy},
				PromptGuidanceByKind: map[plan.SheetKind]string{
					plan.KindNPC:   "npc guidance",
					plan.KindEnemy: "enemy guidance",
				},
			},
		},
		{
			name: "Edge: a template without per-kind guidance is valid",
			tmpl: Template{PromptGuidance: "shared guidance"},
		},
		{
			name: "Error: an unknown kind is refused",
			tmpl: Template{
				Kinds:                []plan.SheetKind{plan.KindNPC},
				PromptGuidanceByKind: map[plan.SheetKind]string{"dragon": "dragon guidance"},
			},
			wantErr: ErrPromptGuidanceKindInvalid,
		},
		{
			name: "Error: a kind the template does not accept is refused",
			tmpl: Template{
				Kinds:                []plan.SheetKind{plan.KindCharacter},
				PromptGuidanceByKind: map[plan.SheetKind]string{plan.KindEnemy: "enemy guidance"},
			},
			wantErr: ErrPromptGuidanceKindNotAccepted,
		},
		{
			name: "Error: empty guidance for an accepted kind is refused",
			tmpl: Template{
				Kinds:                []plan.SheetKind{plan.KindNPC},
				PromptGuidanceByKind: map[plan.SheetKind]string{plan.KindNPC: ""},
			},
			wantErr: ErrPromptGuidanceEmpty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePromptGuidance(tc.tmpl)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestDefaultTemplates_CloneKindGuidance(t *testing.T) {
	t.Run("Edge: mutating a returned per-kind guidance map leaves the defaults intact", func(t *testing.T) {
		first := fifthTemplateByID(t, fifthStatBlockID)
		first.PromptGuidanceByKind[plan.KindNPC] = "mutated"

		second := fifthTemplateByID(t, fifthStatBlockID)

		assert.NotEqual(t, "mutated", second.PromptGuidanceByKind[plan.KindNPC])
		assert.Contains(t, second.PromptGuidanceByKind[plan.KindNPC], "non-player character")
	})
}

func TestFifthEditionPromptGuidance(t *testing.T) {
	t.Run("Happy path: each fifth-edition template targets only the kinds it accepts", func(t *testing.T) {
		for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
			tmpl := fifthTemplateByID(t, id)

			require.NotEmpty(t, tmpl.PromptGuidanceByKind, id)
			for _, kind := range plan.SheetKinds() {
				guidance, ok := tmpl.PromptGuidanceByKind[kind]
				if tmpl.AcceptsKind(kind) {
					assert.NotEmpty(t, guidance, "%s accepts %s", id, kind)
					continue
				}
				assert.False(t, ok, "%s does not accept %s", id, kind)
			}
		}
	})
}
