package template

import (
	"fmt"
	"strings"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fifthTemplateByID(t *testing.T, id string) Template {
	t.Helper()
	for _, candidate := range DefaultTemplates() {
		if candidate.ID == id {
			return candidate
		}
	}
	require.FailNow(t, "missing default template", id)
	return Template{}
}

func fifthDAG(t *testing.T, tmpl Template) *DAG {
	t.Helper()
	dag, err := BuildDAG(tmpl.ID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func fifthSeedFields(group FieldGroup, includeLists bool) map[string]any {
	result := make(map[string]any)
	for name, node := range group {
		switch {
		case node.IsLeaf():
			result[name] = node.Schema.SeedValue()
		case node.IsGroup():
			nested := fifthSeedFields(node.Children, includeLists)
			if len(nested) > 0 {
				result[name] = nested
			}
		case node.IsList() && includeLists:
			result[name] = map[string]any{}
		}
	}
	return result
}

func fifthSheet(t *testing.T, tmpl Template, flatUpdates map[string]any) map[string]any {
	t.Helper()
	dag := fifthDAG(t, tmpl)
	seeded := fifthSeedFields(tmpl.Fields, true)
	flatSeed, err := maputils.FlattenPathKeys(fifthSeedFields(tmpl.Fields, false))
	require.NoError(t, err)

	derived, err := dag.Recalculate(map[string]any{}, flatSeed)
	require.NoError(t, err)
	require.NoError(t, maputils.ApplyUpdates(seeded, derived))

	if len(flatUpdates) == 0 {
		return seeded
	}

	calc, err := dag.CalculateSave(seeded, flatUpdates)
	require.NoError(t, err)
	return calc.Merged
}

func fifthValue(t *testing.T, data map[string]any, key string) float64 {
	t.Helper()
	value, ok := maputils.Get(data, strings.Split(key, "."))
	require.True(t, ok, "missing field %s", key)
	number, ok := maputils.ToFloat64(value)
	require.True(t, ok, "field %s is not numeric", key)
	return number
}

func fifthEffectiveDice(tmpl Template, section, row string) string {
	if expression, ok := tmpl.RowDice[row]; ok {
		return expression
	}
	return tmpl.SectionDice[section]
}

func fifthSectionRows(t *testing.T, tmpl Template, section string) []string {
	t.Helper()
	for _, entry := range SectionRowKeys(tmpl) {
		if entry.Section == section {
			return entry.Rows
		}
	}
	return nil
}

func TestFifthTemplates_Registered(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		label    string
		kinds    []plan.SheetKind
		identity []string
	}{
		{
			name:     "Happy path: the character template accepts only character sheets",
			id:       fifthCharacterID,
			label:    "5E Character",
			kinds:    []plan.SheetKind{plan.KindCharacter},
			identity: []string{"name", "level", fifthJackOfAllTradesKey, fieldNameProficiencyBonus},
		},
		{
			name:     "Happy path: the stat block template accepts NPC and enemy sheets",
			id:       fifthStatBlockID,
			label:    "5E Stat Block",
			kinds:    []plan.SheetKind{plan.KindNPC, plan.KindEnemy},
			identity: []string{"name", "challengeRating", fieldNameProficiencyBonus},
		},
	}

	templates := DefaultTemplates()
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}
	assert.Contains(t, ids, fifthCharacterID)
	assert.Contains(t, ids, fifthStatBlockID)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := fifthTemplateByID(t, tc.id)
			assert.Equal(t, tc.label, tmpl.Name)
			assert.Equal(t, StateDraft, tmpl.State)
			assert.Equal(t, GameSystemFifth, tmpl.GameSystemID)
			assert.Equal(t, tc.kinds, tmpl.Kinds)
			assert.False(t, tmpl.Default)
			assert.Equal(t, "identity.name", tmpl.NameLeaf)
			require.NoError(t, ValidateNameLeaf(tmpl))
			assert.Equal(t, tc.identity, orderedKeys(tmpl.Fields[fifthIdentitySection].Children, tmpl.Fields[fifthIdentitySection].Order))
		})
	}
}

func TestFifthTemplates_DraftVisibility(t *testing.T) {
	tests := []struct {
		name        string
		drafts      bool
		wantListed  bool
		wantCreates bool
	}{
		{name: "Edge: a draft is hidden and not creatable outside development", drafts: false},
		{name: "Happy path: a draft is listed and creatable in development", drafts: true, wantListed: true, wantCreates: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := BuildRegistry(engine.NewFormulaEngine(), WithDraftsAllowed(tc.drafts))
			require.NoError(t, err)

			listed := catalogIDs(registry.List())
			for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
				assert.Equal(t, tc.wantListed, containsString(listed, id), "%s listed", id)

				tmpl, ok := registry.FetchByID(id)
				require.True(t, ok, "%s fetchable", id)
				assert.Equal(t, tc.wantCreates, tmpl.Creatable(tc.drafts))
			}
		})
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestFifthProficiencyBonus_Character(t *testing.T) {
	tests := []struct {
		level float64
		want  float64
	}{
		{level: 1, want: 2},
		{level: 2, want: 2},
		{level: 3, want: 2},
		{level: 4, want: 2},
		{level: 5, want: 3},
		{level: 6, want: 3},
		{level: 7, want: 3},
		{level: 8, want: 3},
		{level: 9, want: 4},
		{level: 10, want: 4},
		{level: 11, want: 4},
		{level: 12, want: 4},
		{level: 13, want: 5},
		{level: 14, want: 5},
		{level: 15, want: 5},
		{level: 16, want: 5},
		{level: 17, want: 6},
		{level: 18, want: 6},
		{level: 19, want: 6},
		{level: 20, want: 6},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(fmt.Sprintf("Happy path: level %v gives +%v", tc.level, tc.want), func(t *testing.T) {
			data := fifthSheet(t, character, map[string]any{"identity.level": tc.level})
			assert.InDelta(t, tc.want, fifthValue(t, data, "identity.proficiencyBonus"), 1e-9)
		})
	}
}

func TestFifthProficiencyBonus_StatBlock(t *testing.T) {
	tests := []struct {
		cr   float64
		want float64
	}{
		{cr: 0, want: 2},
		{cr: 0.125, want: 2},
		{cr: 0.25, want: 2},
		{cr: 0.5, want: 2},
		{cr: 1, want: 2},
		{cr: 2, want: 2},
		{cr: 3, want: 2},
		{cr: 4, want: 2},
		{cr: 5, want: 3},
		{cr: 6, want: 3},
		{cr: 7, want: 3},
		{cr: 8, want: 3},
		{cr: 9, want: 4},
		{cr: 10, want: 4},
		{cr: 11, want: 4},
		{cr: 12, want: 4},
		{cr: 13, want: 5},
		{cr: 14, want: 5},
		{cr: 15, want: 5},
		{cr: 16, want: 5},
		{cr: 17, want: 6},
		{cr: 18, want: 6},
		{cr: 19, want: 6},
		{cr: 20, want: 6},
		{cr: 21, want: 7},
		{cr: 22, want: 7},
		{cr: 23, want: 7},
		{cr: 24, want: 7},
		{cr: 25, want: 8},
		{cr: 26, want: 8},
		{cr: 27, want: 8},
		{cr: 28, want: 8},
		{cr: 29, want: 9},
		{cr: 30, want: 9},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(fmt.Sprintf("Happy path: CR %v gives +%v", tc.cr, tc.want), func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{"identity.challengeRating": tc.cr})
			assert.InDelta(t, tc.want, fifthValue(t, data, "identity.proficiencyBonus"), 1e-9)
		})
	}
}

func TestFifthAbilityModifiers(t *testing.T) {
	tests := []struct {
		score float64
		want  float64
	}{
		{score: 1, want: -5},
		{score: 2, want: -4},
		{score: 3, want: -4},
		{score: 4, want: -3},
		{score: 8, want: -1},
		{score: 9, want: -1},
		{score: 10, want: 0},
		{score: 11, want: 0},
		{score: 12, want: 1},
		{score: 13, want: 1},
		{score: 18, want: 4},
		{score: 20, want: 5},
		{score: 30, want: 10},
	}

	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		for _, tc := range tests {
			t.Run(fmt.Sprintf("Happy path: %s strength %v gives %+v", id, tc.score, tc.want), func(t *testing.T) {
				data := fifthSheet(t, tmpl, map[string]any{"abilities.strength.score": tc.score})
				assert.InDelta(t, tc.want, fifthValue(t, data, "abilities.strength.mod"), 1e-9)
			})
		}
	}
}

func TestFifthSaves(t *testing.T) {
	tests := []struct {
		name   string
		score  float64
		prof   float64
		misc   float64
		level  float64
		expect float64
	}{
		{name: "Happy path: an untrained save is the bare modifier", score: 10, prof: 0, misc: 0, level: 1, expect: 0},
		{name: "Happy path: proficiency adds the bonus", score: 16, prof: 1, misc: 0, level: 1, expect: 5},
		{name: "Happy path: misc stacks on the proficient total", score: 16, prof: 1, misc: 1, level: 1, expect: 6},
		{name: "Edge: a negative modifier and a negative misc still sum", score: 8, prof: 1, misc: -2, level: 5, expect: 0},
		{name: "Edge: a level-20 proficiency doubles the gap", score: 20, prof: 1, misc: 0, level: 20, expect: 11},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, character, map[string]any{
				"identity.level":            tc.level,
				"abilities.dexterity.score": tc.score,
				"saves.dexterity.prof":      tc.prof,
				"saves.dexterity.misc":      tc.misc,
			})
			assert.InDelta(t, tc.expect, fifthValue(t, data, "saves.dexterity.total"), 1e-9)
		})
	}
}

func TestFifthSkills(t *testing.T) {
	tests := []struct {
		name   string
		rank   float64
		misc   float64
		level  float64
		expect float64
	}{
		{name: "Happy path: rank 0 is the ability modifier and misc", rank: 0, misc: 0, level: 1, expect: 2},
		{name: "Happy path: rank 1 adds the proficiency bonus", rank: 1, misc: 0, level: 1, expect: 4},
		{name: "Happy path: rank 2 adds twice the proficiency bonus", rank: 2, misc: 0, level: 1, expect: 6},
		{name: "Happy path: misc stacks on expertise", rank: 2, misc: 1, level: 1, expect: 7},
		{name: "Edge: a level-20 rank 2 doubles the bonus", rank: 2, misc: 0, level: 20, expect: 14},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, character, map[string]any{
				"identity.level":         tc.level,
				"abilities.wisdom.score": float64(14),
				"skills.perception.rank": tc.rank,
				"skills.perception.misc": tc.misc,
			})
			assert.InDelta(t, tc.expect, fifthValue(t, data, "skills.perception.total"), 1e-9)
		})
	}
}

func TestFifthJackOfAllTrades(t *testing.T) {
	tests := []struct {
		name   string
		level  float64
		rank   float64
		flag   float64
		expect float64
	}{
		{name: "Happy path: the flag adds half proficiency to a rank-0 skill", level: 5, rank: 0, flag: 1, expect: 3},
		{name: "Edge: the flag is off by default", level: 5, rank: 0, flag: 0, expect: 2},
		{name: "Edge: a trained skill never takes the half bonus", level: 5, rank: 1, flag: 1, expect: 5},
		{name: "Edge: expertise never takes the half bonus", level: 5, rank: 2, flag: 1, expect: 8},
		{name: "Happy path: half of a +4 proficiency rounds down to +2", level: 9, rank: 0, flag: 1, expect: 4},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, character, map[string]any{
				"identity.level":           tc.level,
				"identity.jackOfAllTrades": tc.flag,
				"abilities.wisdom.score":   float64(14),
				"skills.perception.rank":   tc.rank,
			})
			assert.InDelta(t, tc.expect, fifthValue(t, data, "skills.perception.total"), 1e-9)
		})
	}

	t.Run("Edge: the stat block has no Jack of All Trades flag", func(t *testing.T) {
		statBlock := fifthTemplateByID(t, fifthStatBlockID)
		_, ok := statBlock.Fields[fifthIdentitySection].Children[fifthJackOfAllTradesKey]
		assert.False(t, ok)
	})
}

func TestFifthExhaustionPenalty(t *testing.T) {
	tests := []struct {
		level float64
		want  float64
	}{
		{level: 0, want: 0},
		{level: 1, want: -2},
		{level: 3, want: -6},
		{level: 6, want: -12},
	}

	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		for _, tc := range tests {
			t.Run(fmt.Sprintf("Happy path: %s exhaustion %v gives %v", id, tc.level, tc.want), func(t *testing.T) {
				data := fifthSheet(t, tmpl, map[string]any{"exhaustion.level": tc.level})
				assert.InDelta(t, tc.want, fifthValue(t, data, "exhaustion.penalty"), 1e-9)
			})
		}
	}
}

func TestFifthExhaustionNeverChangesTotals(t *testing.T) {
	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		t.Run("Edge: "+id+" keeps every displayed total when exhaustion rises", func(t *testing.T) {
			base := map[string]any{
				"abilities.dexterity.score": float64(16),
				"saves.dexterity.prof":      float64(1),
				"skills.perception.rank":    float64(2),
			}
			rested := fifthSheet(t, tmpl, base)
			exhaustedUpdates := map[string]any{"exhaustion.level": float64(6)}
			for key, value := range base {
				exhaustedUpdates[key] = value
			}
			exhausted := fifthSheet(t, tmpl, exhaustedUpdates)

			for _, key := range []string{
				"abilities.dexterity.mod",
				"saves.dexterity.total",
				"skills.perception.total",
				"passive.perception",
			} {
				assert.InDelta(t, fifthValue(t, rested, key), fifthValue(t, exhausted, key), 1e-9, key)
			}
		})
	}
}

func TestFifthCheckDiceCarryExhaustion(t *testing.T) {
	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		require.NoError(t, ValidateDiceDefaults(tmpl))

		for _, section := range []string{fifthAbilitiesSection, fifthSavesSection, fifthSkillsSection} {
			rows := fifthSectionRows(t, tmpl, section)
			require.NotEmpty(t, rows, "%s %s rows", id, section)

			for _, row := range rows {
				expression := fifthEffectiveDice(tmpl, section, row)
				assert.Contains(t, expression, "exhaustion.penalty",
					"%s %s must add the exhaustion penalty", id, row)
				assert.LessOrEqual(t, len(expression), 64,
					"%s %s dice must stay inside the 64-character cap", id, row)
			}
		}
	}
}

func TestFifthPassivePerception(t *testing.T) {
	tests := []struct {
		name        string
		choice      string
		rank        float64
		want        float64
		description string
	}{
		{name: "Happy path: the default choice is none", choice: "", rank: 0, want: 12, description: "10 + 2"},
		{name: "Happy path: advantage adds five", choice: "passive.choiceAdvantage", rank: 0, want: 17, description: "10 + 2 + 5"},
		{name: "Happy path: disadvantage subtracts five", choice: "passive.choiceDisadvantage", rank: 0, want: 7, description: "10 + 2 - 5"},
		{name: "Happy path: expertise raises the passive score", choice: "", rank: 2, want: 16, description: "10 + 6"},
		{name: "Happy path: expertise with advantage", choice: "passive.choiceAdvantage", rank: 2, want: 21, description: "10 + 6 + 5"},
	}

	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s: %s", tc.name, id), func(t *testing.T) {
				updates := map[string]any{
					"abilities.wisdom.score": float64(14),
					"skills.perception.rank": tc.rank,
				}
				if tc.choice != "" {
					updates[tc.choice] = float64(1)
				}
				data := fifthSheet(t, tmpl, updates)
				assert.InDelta(t, tc.want, fifthValue(t, data, "passive.perception"), 1e-9, tc.description)
			})
		}
	}
}

func TestFifthPassivePerceptionHasNoDice(t *testing.T) {
	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		for _, row := range fifthSectionRows(t, tmpl, fifthPassiveSection) {
			assert.Empty(t, fifthEffectiveDice(tmpl, fifthPassiveSection, row), "%s %s", id, row)
		}
	}
}

func TestFifthArmorClass(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
		updates    map[string]any
		unarmored  bool
		want       float64
	}{
		{
			name:       "Happy path: unarmored is ten plus dexterity",
			templateID: fifthCharacterID,
			updates:    map[string]any{"abilities.dexterity.score": float64(14)},
			want:       12,
		},
		{
			name:       "Happy path: light armor adds the full dexterity",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceLight":         float64(1),
				"armor.base":                float64(11),
				"abilities.dexterity.score": float64(14),
			},
			want: 13,
		},
		{
			name:       "Happy path: medium armor caps dexterity at two",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceMedium":        float64(1),
				"armor.base":                float64(12),
				"abilities.dexterity.score": float64(16),
			},
			want: 14,
		},
		{
			name:       "Edge: a negative dexterity still applies in medium armor",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceMedium":        float64(1),
				"armor.base":                float64(12),
				"abilities.dexterity.score": float64(8),
			},
			want: 11,
		},
		{
			name:       "Happy path: heavy armor ignores dexterity",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceHeavy":         float64(1),
				"armor.base":                float64(16),
				"abilities.dexterity.score": float64(20),
			},
			want: 16,
		},
		{
			name:       "Happy path: Barbarian unarmored defense adds constitution",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceBarbarian":        float64(1),
				"abilities.dexterity.score":    float64(14),
				"abilities.constitution.score": float64(16),
			},
			want: 15,
		},
		{
			name:       "Happy path: Barbarian unarmored defense keeps the shield",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceBarbarian":        float64(1),
				"armor.shield":                 float64(1),
				"abilities.dexterity.score":    float64(14),
				"abilities.constitution.score": float64(16),
			},
			want: 17,
		},
		{
			name:       "Happy path: Monk unarmored defense adds wisdom",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceMonk":          float64(1),
				"abilities.dexterity.score": float64(16),
				"abilities.wisdom.score":    float64(14),
			},
			want: 15,
		},
		{
			name:       "Edge: Monk ignores the shield",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.choiceMonk":          float64(1),
				"armor.shield":              float64(1),
				"abilities.dexterity.score": float64(16),
				"abilities.wisdom.score":    float64(14),
			},
			want: 15,
		},
		{
			name:       "Happy path: the shield adds two to unarmored",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.shield":              float64(1),
				"abilities.dexterity.score": float64(14),
			},
			want: 14,
		},
		{
			name:       "Happy path: the misc bonus stacks on any base",
			templateID: fifthCharacterID,
			updates: map[string]any{
				"armor.misc":                float64(1),
				"abilities.dexterity.score": float64(14),
			},
			want: 13,
		},
		{
			name:       "Happy path: natural armor is its base plus dexterity",
			templateID: fifthStatBlockID,
			updates: map[string]any{
				"armor.choiceNatural":       float64(1),
				"armor.base":                float64(13),
				"abilities.dexterity.score": float64(14),
			},
			want: 15,
		},
		{
			name:       "Happy path: natural armor keeps the shield",
			templateID: fifthStatBlockID,
			updates: map[string]any{
				"armor.choiceNatural":       float64(1),
				"armor.base":                float64(13),
				"armor.shield":              float64(1),
				"abilities.dexterity.score": float64(14),
			},
			want: 17,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := fifthTemplateByID(t, tc.templateID)
			data := fifthSheet(t, tmpl, tc.updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, "armor.armorClass"), 1e-9)
		})
	}
}

func TestFifthStatBlockHasNaturalArmorOnly(t *testing.T) {
	character := fifthTemplateByID(t, fifthCharacterID)
	statBlock := fifthTemplateByID(t, fifthStatBlockID)

	_, ok := character.Fields[fifthArmorSection].Children[fifthArmorChoiceNaturalKey]
	assert.False(t, ok)
	_, ok = statBlock.Fields[fifthArmorSection].Children[fifthArmorChoiceNaturalKey]
	assert.True(t, ok)
}

func TestFifth_FormulaReferencesMatchDependsOn(t *testing.T) {
	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		t.Run("Happy path: "+id+" declares every formula read", func(t *testing.T) {
			schemas, paths := collectLeaves(tmpl.Fields)
			require.NotEmpty(t, schemas)

			validEngineKeys := make(map[string]string, len(paths))
			for _, path := range paths {
				validEngineKeys[path.EngineKey()] = path.String()
			}

			for i, schema := range schemas {
				if schema.Formula == "" {
					continue
				}
				key := paths[i].String()
				require.NotEmpty(t, schema.DependsOn, "derived leaf %s declares no dependencies", key)

				declared := make(map[string]bool, len(schema.DependsOn))
				for _, dep := range schema.DependsOn {
					engineKey := Path(dep).EngineKey()
					_, exists := validEngineKeys[engineKey]
					require.True(t, exists, "DependsOn of %s references unknown leaf %s", key, Path(dep).String())
					declared[engineKey] = true
				}

				referenced := make(map[string]bool)
				for _, ident := range formulaIdentRe.FindAllString(schema.Formula, -1) {
					if engineFunctions[ident] {
						continue
					}
					referenced[ident] = true
					_, exists := validEngineKeys[ident]
					require.True(t, exists, "formula of %s references unknown identifier %s", key, ident)
				}

				for ident := range referenced {
					assert.True(t, declared[ident], "formula of %s reads %s but DependsOn does not declare it", key, ident)
				}
				for engineKey := range declared {
					assert.True(t, referenced[engineKey], "DependsOn of %s declares %s which the formula never reads", key, engineKey)
				}
			}
		})
	}
}

func TestFifth_LeafContracts(t *testing.T) {
	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		t.Run("Happy path: "+id+" derives or enters every leaf", func(t *testing.T) {
			schemas, paths := collectLeaves(tmpl.Fields)
			require.NotEmpty(t, schemas)

			for i, schema := range schemas {
				key := paths[i].String()
				assert.NotEmpty(t, strings.TrimSpace(schema.Label), "leaf %s has no label", key)
				assert.Contains(t, []string{fieldTypeNumber, fieldTypeText}, schema.Type, "leaf %s has an unsupported type", key)

				switch {
				case schema.Formula != "":
					assert.False(t, schema.IsEditable, "derived leaf %s must not be editable", key)
					assert.NotEmpty(t, schema.DependsOn, "derived leaf %s declares no dependencies", key)
				default:
					assert.True(t, schema.IsEditable, "leaf %s is neither derived nor editable", key)
				}

				if schema.Type == fieldTypeText {
					assert.Empty(t, schema.Formula, "text leaf %s must not carry a formula", key)
				}
			}
		})

		t.Run("Edge: "+id+" keeps every rank outside config", func(t *testing.T) {
			_, paths := collectLeaves(tmpl.Fields)
			for _, path := range paths {
				if path[len(path)-1] != fifthRankKey {
					continue
				}
				assert.Equal(t, fifthSkillsSection, path[0], "rank %s must live under skills", path.String())
			}
		})

		t.Run("Edge: "+id+" uses no publisher trademark", func(t *testing.T) {
			identity := strings.ToLower(tmpl.ID + " " + tmpl.Name)
			assert.NotContains(t, identity, "d&d")
			assert.NotContains(t, identity, "dnd")
		})
	}
}

func TestFifth_SkillsCoverTheEighteenSRDSkills(t *testing.T) {
	want := []string{
		"acrobatics", "animalHandling", "arcana", "athletics", "deception",
		"history", "insight", "intimidation", "investigation", "medicine",
		"nature", "perception", "performance", "persuasion", "religion",
		"sleightOfHand", "stealth", "survival",
	}

	for _, id := range []string{fifthCharacterID, fifthStatBlockID} {
		tmpl := fifthTemplateByID(t, id)
		group := tmpl.Fields[fifthSkillsSection].Children
		keys := groupKeys(group)
		assert.ElementsMatch(t, want, keys, id)
		for _, key := range want {
			require.NotNil(t, group[key].Children, "%s skill %s", id, key)
			assert.Equal(t, []string{"rank", "misc", "total"}, group[key].Columns, "%s skill %s columns", id, key)
		}
	}
}
