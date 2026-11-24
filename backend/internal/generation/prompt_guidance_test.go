package generation_test

import (
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemPrompt_FifthEditionKindGuidance(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
		kind       plan.SheetKind
		subject    string
		rules      []string
	}{
		{
			name:       "Happy path: the 5E Character guidance addresses the character",
			templateID: "fifth_character_v1",
			kind:       plan.KindCharacter,
			subject:    "The subject is a player character with one class and its SRD subclass.",
			rules: []string{
				"closest SRD option",
				"Name the character with an original name.",
				"level 1 with 0 experience points",
				"Set the hit point base to the Hit Die size and the hit points per level to half the die size plus one.",
				"Write feature text as a short paraphrase of at most about 300 runes.",
			},
		},
		{
			name:       "Happy path: the 5E Stat Block NPC guidance addresses the NPC",
			templateID: "fifth_stat_block_v1",
			kind:       plan.KindNPC,
			subject:    "The subject is a non-player character that the game master controls.",
			rules: []string{
				"closest SRD option",
				"SRD non-player character",
				"SRD name or an original name",
				"challenge rating from the concept",
				"Write trait and action text as a short paraphrase of at most about 300 runes.",
			},
		},
		{
			name:       "Happy path: the 5E Stat Block enemy guidance addresses the enemy",
			templateID: "fifth_stat_block_v1",
			kind:       plan.KindEnemy,
			subject:    "The subject is an enemy that the game master controls.",
			rules: []string{
				"closest SRD option",
				"SRD monster",
				"SRD name or an original name",
				"challenge rating from the concept",
				"Write trait and action text as a short paraphrase of at most about 300 runes.",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := defaultTemplateFixture(t, tc.templateID)

			prompt, err := generation.SystemPrompt(tmpl, tc.kind)
			require.NoError(t, err)

			assert.Contains(t, prompt, fifthEditionBrief)
			assert.Contains(t, prompt, tc.subject)
			for _, rule := range tc.rules {
				assert.Contains(t, prompt, rule)
			}
		})
	}
}

func TestSystemPrompt_OutOfSRDRules(t *testing.T) {
	t.Run("Edge: the 5E Character prompt turns a multiclass concept into its dominant class", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(defaultTemplateFixture(t, "fifth_character_v1"), plan.KindCharacter)
		require.NoError(t, err)

		assert.Contains(t, prompt, "A multiclass concept becomes its dominant class.")
	})

	t.Run("Edge: the 5E Stat Block prompt turns a non-SRD monster into an original creature", func(t *testing.T) {
		for _, kind := range []plan.SheetKind{plan.KindNPC, plan.KindEnemy} {
			prompt, err := generation.SystemPrompt(defaultTemplateFixture(t, "fifth_stat_block_v1"), kind)
			require.NoError(t, err)

			assert.Contains(t, prompt, "A monster outside the SRD becomes an original creature.", "kind %s", kind)
		}
	})

	t.Run("Edge: the character guidance does not carry the stat block rules", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(defaultTemplateFixture(t, "fifth_character_v1"), plan.KindCharacter)
		require.NoError(t, err)

		assert.NotContains(t, prompt, "A monster outside the SRD becomes an original creature.")
		assert.NotContains(t, prompt, "challenge rating from the concept")
	})

	t.Run("Edge: the stat block guidance does not carry the character rules", func(t *testing.T) {
		for _, kind := range []plan.SheetKind{plan.KindNPC, plan.KindEnemy} {
			prompt, err := generation.SystemPrompt(defaultTemplateFixture(t, "fifth_stat_block_v1"), kind)
			require.NoError(t, err)

			assert.NotContains(t, prompt, "A multiclass concept becomes its dominant class.", "kind %s", kind)
			assert.NotContains(t, prompt, "level 1 with 0 experience points", "kind %s", kind)
		}
	})
}

func TestSystemPrompt_KindGuidancePerKind(t *testing.T) {
	t.Run("Happy path: the stat block NPC and enemy prompts differ by their subject", func(t *testing.T) {
		tmpl := defaultTemplateFixture(t, "fifth_stat_block_v1")

		npcPrompt, err := generation.SystemPrompt(tmpl, plan.KindNPC)
		require.NoError(t, err)
		enemyPrompt, err := generation.SystemPrompt(tmpl, plan.KindEnemy)
		require.NoError(t, err)

		assert.NotEqual(t, npcPrompt, enemyPrompt)
		assert.Contains(t, npcPrompt, "SRD non-player character")
		assert.NotContains(t, npcPrompt, "SRD monster")
		assert.Contains(t, enemyPrompt, "SRD monster")
		assert.NotContains(t, enemyPrompt, "SRD non-player character")
	})

	t.Run("Edge: the frozen fantasy template carries no fifth-edition guidance", func(t *testing.T) {
		prompt, err := generation.SystemPrompt(fantasyV1Fixture(t), plan.KindCharacter)
		require.NoError(t, err)

		assert.NotContains(t, prompt, "The subject is a player character")
		assert.NotContains(t, prompt, "closest SRD option")
		assert.NotContains(t, prompt, fifthEditionBrief)
	})
}
