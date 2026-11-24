package generation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func schemaNode(t *testing.T, properties map[string]any, path ...string) map[string]any {
	t.Helper()
	current := properties
	var node map[string]any
	for _, name := range path {
		child, ok := current[name].(map[string]any)
		require.Truef(t, ok, "missing %s", name)
		node = child
		if nested, ok := child["properties"].(map[string]any); ok {
			current = nested
		}
	}
	return node
}

func schemaItemProperties(t *testing.T, call map[string]any) map[string]any {
	t.Helper()
	items, ok := call["items"].(map[string]any)
	require.True(t, ok)
	properties, ok := items["properties"].(map[string]any)
	require.True(t, ok)
	return properties
}

func TestOutputSchema_ContentChecks(t *testing.T) {
	character := schemaProps(t, mustSchema(t, fifthTemplateFixture(t, "fifth_character_v1"), plan.KindCharacter))
	statBlock := schemaProps(t, mustSchema(t, fifthTemplateFixture(t, "fifth_stat_block_v1"), plan.KindNPC))

	t.Run("Happy path: feature text is capped at 300 runes in the schema", func(t *testing.T) {
		features := schemaItemProperties(t, schemaNode(t, character, "features", "list"))
		text := schemaNode(t, features, "text")
		assert.InDelta(t, 300.0, text["maxLength"], 1e-9)
	})

	t.Run("Happy path: trait text is capped at 300 runes in the schema", func(t *testing.T) {
		traits := schemaItemProperties(t, schemaNode(t, statBlock, "traits"))
		text := schemaNode(t, traits, "text")
		assert.InDelta(t, 300.0, text["maxLength"], 1e-9)
	})

	t.Run("Happy path: the hit die is a numeric enum of 6, 8, 10 and 12", func(t *testing.T) {
		size := schemaNode(t, character, "hitDice", "primary", "size")
		assert.Equal(t, []any{6.0, 8.0, 10.0, 12.0}, size["enum"])
	})

	t.Run("Happy path: spell slots carry a non-negative minimum", func(t *testing.T) {
		total := schemaNode(t, character, "spellcasting", "slot1", "total")
		assert.InDelta(t, 0.0, total["minimum"], 1e-9)
		assert.NotContains(t, total, "maximum")
	})

	t.Run("Happy path: save proficiencies are generatable flags", func(t *testing.T) {
		prof := schemaNode(t, character, "saves", "strength", "prof")
		assert.Equal(t, []any{0.0, 1.0}, prof["enum"])
	})

	t.Run("Edge: state fields stay out of the character schema", func(t *testing.T) {
		assert.NotContains(t, schemaNode(t, character, "combat")["properties"], "hp")
		status := schemaNode(t, character, "status")["properties"].(map[string]any)
		assert.NotContains(t, status, "conditions")
		assert.NotContains(t, status, "deathSaves")
		assert.NotContains(t, schemaNode(t, character, "hitDice", "primary")["properties"], "spent")
		assert.NotContains(t, schemaNode(t, character, "spellcasting", "slot1")["properties"], "spent")
		assert.NotContains(t, character, "playerName")

		resources := schemaItemProperties(t, schemaNode(t, character, "resources", "list"))
		assert.NotContains(t, resources, "used")
	})

	t.Run("Edge: the stat block resource usage stays out of the schema", func(t *testing.T) {
		resources := schemaItemProperties(t, schemaNode(t, statBlock, "resources"))
		assert.NotContains(t, resources, "used")
		assert.Contains(t, resources, "max")
	})
}

func TestSystemPrompt_ContentChecks(t *testing.T) {
	character, err := generation.SystemPrompt(fifthTemplateFixture(t, "fifth_character_v1"), plan.KindCharacter)
	require.NoError(t, err)
	statBlock, err := generation.SystemPrompt(fifthTemplateFixture(t, "fifth_stat_block_v1"), plan.KindNPC)
	require.NoError(t, err)

	t.Run("Happy path: the hit die lists its four allowed values", func(t *testing.T) {
		assert.Contains(t, character, "hitDice.primary.size (Hit Die size, number) [min 6, max 12, one of: 6, 8, 10, 12]")
	})

	t.Run("Happy path: the challenge rating has no keep-default hint", func(t *testing.T) {
		assert.Contains(t, statBlock, "identity.challengeRating (Challenge rating, number) [min 0, max 30]\n")
		assert.NotContains(t, statBlock, "challengeRating (Challenge rating, number) [min 0, max 30, keep")
	})

	t.Run("Happy path: experience points keep zero unless the concept states otherwise", func(t *testing.T) {
		assert.Contains(t, character, "status.xp (Experience points, number) [min 0, max 355000, keep 0 unless the concept states otherwise]")
	})

	t.Run("Edge: state fields are not prompted", func(t *testing.T) {
		for _, path := range []string{
			"combat.hp.current", "combat.hp.temp", "hitDice.primary.spent",
			"status.conditions", "status.deathSaves", "spellcasting.slot1.spent",
			"resources.list.used",
		} {
			assert.NotContains(t, character, path)
		}
		assert.NotContains(t, statBlock, "resources.used")
	})

	t.Run("Happy path: feature text is prompted with its 300-rune bound", func(t *testing.T) {
		assert.Contains(t, character, "features.list (Features and feats, at most 15 records)")
	})
}

func TestExpand_ContentChecks(t *testing.T) {
	tmpl := contentFixtureTemplate()

	withName := func(name string) map[string]any {
		output := contentOutput()
		output["identity"].(map[string]any)["name"] = name
		return output
	}
	withRecordText := func(text string) map[string]any {
		output := contentOutput()
		output["notes"] = map[string]any{"list": []map[string]any{{"text": text}}}
		return output
	}
	withSaves := func(on ...string) map[string]any {
		output := contentOutput()
		saves := output["saves"].(map[string]any)
		for ability := range saves {
			saves[ability] = 0
		}
		for _, ability := range on {
			saves[ability] = 1
		}
		return output
	}
	withHitDie := func(value int) map[string]any {
		output := contentOutput()
		output["hitDice"] = map[string]any{"size": value}
		return output
	}
	withSlots := func(value int) map[string]any {
		output := contentOutput()
		output["spellcasting"] = map[string]any{"slots": value}
		return output
	}

	tests := []struct {
		name   string
		output func() map[string]any
		want   string
	}{
		{name: "Happy path: a valid output passes", output: contentOutput},
		{name: "Edge: an empty text value passes", output: func() map[string]any { return withName("") }},
		{name: "Happy path: exactly 300 runes of feature text pass", output: func() map[string]any {
			return withRecordText(strings.Repeat("x", 300))
		}},
		{name: "Happy path: exactly two save proficiencies pass", output: func() map[string]any {
			return withSaves("strength", "wisdom")
		}},
		{name: "Happy path: a hit die of 12 passes", output: func() map[string]any { return withHitDie(12) }},
		{name: "Happy path: zero spell slots pass", output: func() map[string]any { return withSlots(0) }},
		{name: "Happy path: a denylisted substring passes", output: func() map[string]any {
			return withName("beholders and unofficial races")
		}},
		{name: "Error: 301 runes of feature text are refused", output: func() map[string]any {
			return withRecordText(strings.Repeat("x", 301))
		}, want: "notes.list[0].text: is longer than the maximum length"},
		{name: "Error: an uppercase denylisted term is refused", output: func() map[string]any {
			return withName("A BEHOLDER waits")
		}, want: `identity.name: contains the denied term "beholder"`},
		{name: "Error: a mixed-case denylisted term is refused", output: func() map[string]any {
			return withName("a BeHoLdEr waits")
		}, want: `identity.name: contains the denied term "beholder"`},
		{name: "Error: a multi-word denylisted term is refused", output: func() map[string]any {
			return withName("a mind flayer scout")
		}, want: `identity.name: contains the denied term "mind flayer"`},
		{name: "Error: a trademark denylisted term is refused", output: func() map[string]any {
			return withName("The D&D guild")
		}, want: `identity.name: contains the denied term "D&D"`},
		{name: "Error: a denied term inside a record text is refused", output: func() map[string]any {
			return withRecordText("The beholder waits")
		}, want: `notes.list[0].text: contains the denied term "beholder"`},
		{name: "Error: a single save proficiency is refused", output: func() map[string]any {
			return withSaves("strength")
		}, want: "saves: must have exactly 2 save proficiencies"},
		{name: "Error: three save proficiencies are refused", output: func() map[string]any {
			return withSaves("strength", "dexterity", "constitution")
		}, want: "saves: must have exactly 2 save proficiencies"},
		{name: "Error: a hit die of 4 is refused", output: func() map[string]any { return withHitDie(4) },
			want: "hitDice.size: is below the minimum"},
		{name: "Error: a hit die of 20 is refused", output: func() map[string]any { return withHitDie(20) },
			want: "hitDice.size: is above the maximum"},
		{name: "Error: a hit die of 7 is refused", output: func() map[string]any { return withHitDie(7) },
			want: "hitDice.size: must be one of the listed options"},
		{name: "Error: a negative spell slot count is refused", output: func() map[string]any { return withSlots(-1) },
			want: "spellcasting.slots: is below the minimum"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generation.Expand(tmpl, plan.KindCharacter, fixtureObject(t, tc.output()))
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			requireOutputProblem(t, err, tc.want)
		})
	}
}

func TestAssemble_FifthStateDefaults(t *testing.T) {
	tmpl := fifthTemplateFixture(t, "fifth_character_v1")
	dag := fixtureDAG(t, tmpl)

	data, err := generation.Assemble(tmpl, dag, plan.KindCharacter, map[string]any{})
	require.NoError(t, err)

	assert.InDelta(t, 1.0, data["identity"].(map[string]any)["level"], 1e-9, "level prefers 1")
	assert.InDelta(t, 0.0, data["status"].(map[string]any)["xp"], 1e-9, "experience points seed at 0")

	combat := data["combat"].(map[string]any)
	hp := combat["hp"].(map[string]any)
	assert.InDelta(t, 0.0, hp["current"], 1e-9, "current hit points seed at 0")
	assert.InDelta(t, 0.0, hp["temp"], 1e-9, "temporary hit points seed at 0")

	status := data["status"].(map[string]any)
	conditions := status["conditions"].(map[string]any)
	for name, value := range conditions {
		assert.InDelta(t, 0.0, value, 1e-9, "condition %s seeds at 0", name)
	}
	deathSaves := status["deathSaves"].(map[string]any)
	assert.InDelta(t, 0.0, deathSaves["successes"], 1e-9)
	assert.InDelta(t, 0.0, deathSaves["failures"], 1e-9)

	hitDice := data["hitDice"].(map[string]any)
	assert.InDelta(t, 0.0, hitDice["primary"].(map[string]any)["spent"], 1e-9)
	assert.InDelta(t, 0.0, hitDice["d12"].(map[string]any)["spent"], 1e-9)

	spellcasting := data["spellcasting"].(map[string]any)
	assert.InDelta(t, 0.0, spellcasting["slot1"].(map[string]any)["spent"], 1e-9)
	assert.InDelta(t, 0.0, spellcasting["pact"].(map[string]any)["used"], 1e-9)
	assert.Equal(t, map[string]any{}, data["resources"].(map[string]any)["list"])
}

func TestWorkerProcess_ContentRetries(t *testing.T) {
	tests := []struct {
		name   string
		broken func(map[string]any)
		fixed  func(map[string]any)
		want   string
	}{
		{
			name: "Happy path: a denylisted term is retried and then accepted",
			broken: func(output map[string]any) {
				output["identity"].(map[string]any)["name"] = "A Beholder"
			},
			want: `identity.name: contains the denied term "beholder"`,
		},
		{
			name: "Happy path: 301 runes of feature text is retried and then accepted",
			broken: func(output map[string]any) {
				output["notes"] = map[string]any{"list": []map[string]any{{"text": strings.Repeat("x", 301)}}}
			},
			fixed: func(output map[string]any) {
				output["notes"] = map[string]any{"list": []map[string]any{{"text": strings.Repeat("x", 300)}}}
			},
			want: "notes.list[0].text: is longer than the maximum length",
		},
		{
			name: "Happy path: three save proficiencies are retried and then accepted",
			broken: func(output map[string]any) {
				output["saves"].(map[string]any)["constitution"] = 1
			},
			want: "saves: must have exactly 2 save proficiencies",
		},
		{
			name: "Happy path: a hit die of 4 is retried and then accepted",
			broken: func(output map[string]any) {
				output["hitDice"] = map[string]any{"size": 4}
			},
			want: "hitDice.size: is below the minimum",
		},
		{
			name: "Happy path: a negative spell slot count is retried and then accepted",
			broken: func(output map[string]any) {
				output["spellcasting"] = map[string]any{"slots": -1}
			},
			want: "spellcasting.slots: is below the minimum",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkerFixture(t, 5)
			tmpl := contentFixtureTemplate()
			dag := fixtureDAG(t, tmpl)
			f.catalog.EXPECT().FetchByID(fixtureID).Return(tmpl, true).Once()
			f.catalog.EXPECT().DAG(fixtureID).Return(dag, true).Once()
			f.expectClaim(dailyReservation)

			broken := contentOutput()
			tc.broken(broken)
			fixed := contentOutput()
			if tc.fixed != nil {
				tc.fixed(fixed)
			}
			brokenRaw, err := json.Marshal(broken)
			require.NoError(t, err)
			fixedRaw, err := json.Marshal(fixed)
			require.NoError(t, err)

			f.generateResponse(llm.Response{Output: brokenRaw}, nil)
			f.client.EXPECT().Generate(mock.Anything, mock.MatchedBy(func(req llm.Request) bool {
				return strings.Contains(req.Prompt, "rejected") && strings.Contains(req.Prompt, tc.want)
			})).Return(llm.Response{Output: fixedRaw}, nil).Once()
			f.completeWith(true)

			require.NoError(t, f.worker.Process(context.Background(), jobID))
		})
	}
}
