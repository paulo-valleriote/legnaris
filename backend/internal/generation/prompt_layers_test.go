package generation_test

import (
	"strings"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fifthEditionBrief = "The game system is fifth edition, as written in SRD 5.2.1."

func defaultTemplateFixture(t *testing.T, id string) *template.Template {
	t.Helper()
	templates := template.DefaultTemplates()
	for i := range templates {
		if templates[i].ID == id {
			return &templates[i]
		}
	}
	require.FailNow(t, "missing default template", id)
	return nil
}

func TestSystemPrompt_LayersPerTemplate(t *testing.T) {
	tests := []struct {
		name              string
		templateID        string
		kind              plan.SheetKind
		subject           string
		wantBrief         bool
		wantFantasyRules  bool
		wantFifthGuidance bool
	}{
		{
			name:              "Happy path: fantasy_v1 has guidance but no game system brief",
			templateID:        fantasyV1ID,
			kind:              plan.KindCharacter,
			subject:           "a tabletop RPG player character sheet",
			wantBrief:         false,
			wantFantasyRules:  true,
			wantFifthGuidance: false,
		},
		{
			name:              "Happy path: the fifth-edition character carries its game system brief and per-kind guidance",
			templateID:        "fifth_character_v1",
			kind:              plan.KindCharacter,
			subject:           "a tabletop RPG player character sheet",
			wantBrief:         true,
			wantFantasyRules:  false,
			wantFifthGuidance: true,
		},
		{
			name:              "Happy path: the fifth-edition stat block carries its game system brief and per-kind guidance",
			templateID:        "fifth_stat_block_v1",
			kind:              plan.KindEnemy,
			subject:           "an enemy sheet that the game master controls",
			wantBrief:         true,
			wantFantasyRules:  false,
			wantFifthGuidance: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := defaultTemplateFixture(t, tc.templateID)

			prompt, err := generation.SystemPrompt(tmpl, tc.kind)
			require.NoError(t, err)

			assert.True(t, strings.HasPrefix(prompt, "You fill in the values of "+tc.subject+"."))
			assert.Contains(t, prompt, "\n\nFields:\n")

			if tc.wantBrief {
				assert.Contains(t, prompt, fifthEditionBrief)
			} else {
				assert.NotContains(t, prompt, fifthEditionBrief)
				assert.NotContains(t, prompt, "game system is")
			}

			if tc.wantFantasyRules {
				assert.Contains(t, prompt, "experience points and proficiency bonus consistent with it.")
			} else {
				assert.NotContains(t, prompt, "experience points and proficiency bonus")
			}

			if tc.wantFifthGuidance {
				assert.Contains(t, prompt, "closest SRD option")
			} else {
				assert.NotContains(t, prompt, "closest SRD option")
			}
		})
	}
}

func TestSystemPrompt_GuidanceLayer(t *testing.T) {
	t.Run("Happy path: per-template guidance is appended before the fields", func(t *testing.T) {
		tmpl := fixtureTemplate()
		tmpl.PromptGuidance = "Custom guidance for this template."

		prompt, err := generation.SystemPrompt(tmpl, plan.KindCharacter)
		require.NoError(t, err)

		guidanceIndex := strings.Index(prompt, "Custom guidance for this template.")
		fieldsIndex := strings.Index(prompt, "\n\nFields:\n")
		require.NotEqual(t, -1, guidanceIndex)
		require.NotEqual(t, -1, fieldsIndex)
		assert.Less(t, guidanceIndex, fieldsIndex)
	})

	t.Run("Edge: a template without guidance adds no level or XP rules", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(fixtureTemplate(), plan.KindCharacter)
		require.NoError(t, err)

		assert.NotContains(t, prompt, "veteran, experienced or powerful never change it")
		assert.NotContains(t, prompt, "experience points and proficiency bonus")
	})

	t.Run("Edge: a template without a game system adds no brief", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(fixtureTemplate(), plan.KindCharacter)
		require.NoError(t, err)

		assert.NotContains(t, prompt, fifthEditionBrief)
		assert.NotContains(t, prompt, "game system is")
	})
}
