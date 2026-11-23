package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFifthSpellcasting_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthSpellcastingSection].Children

	t.Run("Happy path: the six abilities form one exclusive group of numeric flags", func(t *testing.T) {
		for _, ability := range abilities {
			key := fifthSpellcastingAbilityLeafKey(ability.key)
			schema := group[key].Schema
			require.NotNil(t, schema, key)
			assert.Equal(t, fieldTypeNumber, schema.Type, key)
			assert.True(t, schema.IsEditable, key)
			assert.False(t, schema.Flag, key)
			assert.Equal(t, exclusiveGroupSpellcastingAbility, schema.ExclusiveGroup, key)
			assert.Equal(t, ability.label, schema.OptionLabel, key)
			assert.InDelta(t, 0.0, schema.Default, 1e-9, key)
		}
	})

	t.Run("Happy path: the ability modifier, attack and save DC are derived", func(t *testing.T) {
		for _, key := range []string{fifthSpellAbilityModKey, fifthSpellAttackKey, fifthSpellSaveDCKey} {
			schema := group[key].Schema
			require.NotNil(t, schema, key)
			assert.False(t, schema.IsEditable, key)
			assert.NotEmpty(t, schema.Formula, key)
			assert.NotEmpty(t, schema.DependsOn, key)
		}
	})

	t.Run("Happy path: the prepared-spell limit is an entry", func(t *testing.T) {
		schema := group[fifthPreparedLimitKey].Schema
		require.NotNil(t, schema)
		assert.True(t, schema.IsEditable)
		assert.Empty(t, schema.Formula)
	})

	t.Run("Happy path: every spell level enters totals and spent slots behind a derived remaining", func(t *testing.T) {
		for level := 1; level <= 9; level++ {
			key := fifthSpellSlotKey(level)
			node := group[key]
			require.NotNil(t, node.Children, key)
			assert.Equal(t, []string{fieldNameTotal, fifthSpellSlotSpentKey, fieldNameRemaining}, node.Columns, key)

			total := node.Children[fieldNameTotal].Schema
			require.NotNil(t, total, key)
			assert.True(t, total.IsEditable, key)
			assert.InDelta(t, 0.0, total.Default, 1e-9, key)

			spent := node.Children[fifthSpellSlotSpentKey].Schema
			require.NotNil(t, spent, key)
			assert.True(t, spent.IsEditable, key)

			remaining := node.Children[fieldNameRemaining].Schema
			require.NotNil(t, remaining, key)
			assert.False(t, remaining.IsEditable, key)
			assert.NotEmpty(t, remaining.Formula, key)
		}
	})

	t.Run("Happy path: the Pact Magic pool enters count, level and used behind a derived remaining", func(t *testing.T) {
		node := group[fifthPactKey]
		require.NotNil(t, node.Children)
		assert.Equal(t, []string{
			fifthPactCountKey, fifthPactLevelKey, fifthPactUsedKey, fieldNameRemaining,
		}, node.Columns)

		for _, key := range []string{fifthPactCountKey, fifthPactLevelKey, fifthPactUsedKey} {
			schema := node.Children[key].Schema
			require.NotNil(t, schema, key)
			assert.True(t, schema.IsEditable, key)
			assert.InDelta(t, 0.0, schema.Default, 1e-9, key)
		}

		remaining := node.Children[fieldNameRemaining].Schema
		require.NotNil(t, remaining)
		assert.False(t, remaining.IsEditable)
		assert.NotEmpty(t, remaining.Formula)
	})

	t.Run("Happy path: the order lists the abilities, the totals, the slots and the pact", func(t *testing.T) {
		want := []string{
			"abilityStrength", "abilityDexterity", "abilityConstitution",
			"abilityIntelligence", "abilityWisdom", "abilityCharisma",
			fifthSpellAbilityModKey, fifthSpellAttackKey, fifthSpellSaveDCKey, fifthPreparedLimitKey,
			"slot1", "slot2", "slot3", "slot4", "slot5", "slot6", "slot7", "slot8", "slot9",
			fifthPactKey,
		}
		assert.Equal(t, want, fifthSpellcastingOrder())
	})
}

func TestFifthSpellcasting_AttackAndSaveDC(t *testing.T) {
	tests := []struct {
		name       string
		ability    string
		score      float64
		level      float64
		wantMod    float64
		wantAttack float64
		wantDC     float64
	}{
		{name: "Happy path: strength spellcasting", ability: abilityStrength, score: 16, level: 1, wantMod: 3, wantAttack: 5, wantDC: 13},
		{name: "Happy path: dexterity spellcasting", ability: abilityDexterity, score: 14, level: 1, wantMod: 2, wantAttack: 4, wantDC: 12},
		{name: "Happy path: constitution spellcasting", ability: abilityConstitution, score: 18, level: 5, wantMod: 4, wantAttack: 7, wantDC: 15},
		{name: "Happy path: intelligence spellcasting", ability: abilityIntelligence, score: 20, level: 20, wantMod: 5, wantAttack: 11, wantDC: 19},
		{name: "Happy path: wisdom spellcasting", ability: abilityWisdom, score: 8, level: 9, wantMod: -1, wantAttack: 3, wantDC: 11},
		{name: "Happy path: charisma spellcasting", ability: abilityCharisma, score: 12, level: 13, wantMod: 1, wantAttack: 6, wantDC: 14},
		{name: "Edge: no ability chosen leaves the bare proficiency bonus", ability: "", score: 20, level: 1, wantMod: 0, wantAttack: 2, wantDC: 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updates := map[string]any{"identity.level": tc.level}
			if tc.ability != "" {
				updates[Path{fifthAbilitiesSection, tc.ability, fifthScoreKey}.String()] = tc.score
				updates[Path{fifthSpellcastingSection, fifthSpellcastingAbilityLeafKey(tc.ability)}.String()] = float64(1)
			}

			data := fifthSheet(t, fifthTemplateByID(t, fifthCharacterID), updates)
			assert.InDelta(t, tc.wantMod, fifthValue(t, data, "spellcasting.abilityMod"), 1e-9)
			assert.InDelta(t, tc.wantAttack, fifthValue(t, data, "spellcasting.attack"), 1e-9)
			assert.InDelta(t, tc.wantDC, fifthValue(t, data, "spellcasting.saveDC"), 1e-9)
		})
	}
}

func TestFifthSpellcasting_ExhaustionNeverChangesTheSaveDC(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	base := map[string]any{
		"identity.level":               float64(5),
		"abilities.charisma.score":     float64(18),
		"spellcasting.abilityCharisma": float64(1),
	}

	rested := fifthSheet(t, tmpl, base)
	exhaustedUpdates := map[string]any{"exhaustion.level": float64(6)}
	for key, value := range base {
		exhaustedUpdates[key] = value
	}
	exhausted := fifthSheet(t, tmpl, exhaustedUpdates)

	t.Run("Edge: the save DC, the attack and the ability modifier ignore Exhaustion", func(t *testing.T) {
		for _, key := range []string{
			"spellcasting.abilityMod", "spellcasting.attack", "spellcasting.saveDC",
		} {
			assert.InDelta(t, fifthValue(t, rested, key), fifthValue(t, exhausted, key), 1e-9, key)
		}
	})
}

func TestFifthSpellcasting_RowDice(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	require.NoError(t, ValidateDiceDefaults(tmpl))
	require.NoError(t, ValidateNotD20Tests(tmpl))

	t.Run("Happy path: only the spell attack rolls and it carries the Exhaustion penalty", func(t *testing.T) {
		expression := fifthEffectiveDice(tmpl, fifthSpellcastingSection, "spellcasting.attack")
		assert.Contains(t, expression, "1d20")
		assert.Contains(t, expression, "exhaustion.penalty")
		assert.LessOrEqual(t, len(expression), 64)
	})

	t.Run("Happy path: the spell attack stays a d20 test", func(t *testing.T) {
		assert.NotContains(t, tmpl.NotD20Sections, fifthSpellcastingSection)
		assert.NotContains(t, tmpl.NotD20Rows, "spellcasting.attack")
	})

	t.Run("Happy path: no other spellcasting Row rolls", func(t *testing.T) {
		for _, row := range fifthSectionRows(t, tmpl, fifthSpellcastingSection) {
			if row == "spellcasting.attack" {
				continue
			}
			assert.Empty(t, fifthEffectiveDice(tmpl, fifthSpellcastingSection, row), row)
		}
	})
}

func TestFifthSpellcasting_AbilitySwitchesExclusively(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	dag := fifthDAG(t, tmpl)
	data := fifthSheet(t, tmpl, map[string]any{
		"abilities.intelligence.score":     float64(16),
		"spellcasting.abilityIntelligence": float64(1),
	})
	require.InDelta(t, 5.0, fifthValue(t, data, "spellcasting.attack"), 1e-9)

	t.Run("Happy path: choosing another ability clears the previous one", func(t *testing.T) {
		calc, err := dag.CalculateSave(data, map[string]any{
			"abilities.wisdom.score":     float64(20),
			"spellcasting.abilityWisdom": float64(1),
		})
		require.NoError(t, err)

		assert.InDelta(t, 0.0, fifthValue(t, calc.Merged, "spellcasting.abilityIntelligence"), 1e-9)
		assert.InDelta(t, 1.0, fifthValue(t, calc.Merged, "spellcasting.abilityWisdom"), 1e-9)
		assert.InDelta(t, 7.0, fifthValue(t, calc.Merged, "spellcasting.attack"), 1e-9)
	})

	t.Run("Error: two abilities in one save are refused", func(t *testing.T) {
		_, err := dag.CalculateSave(data, map[string]any{
			"spellcasting.abilityIntelligence": float64(1),
			"spellcasting.abilityWisdom":       float64(1),
		})
		require.Error(t, err)
		var conflict *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Equal(t, exclusiveGroupSpellcastingAbility, conflict.Group)
	})
}

func TestFifthSpellcasting_Remaining(t *testing.T) {
	tests := []struct {
		name    string
		updates map[string]any
		field   string
		want    float64
	}{
		{
			name:    "Happy path: a slot remaining subtracts the spent slots",
			updates: map[string]any{"spellcasting.slot1.total": float64(3), "spellcasting.slot1.spent": float64(1)},
			field:   "spellcasting.slot1.remaining",
			want:    2,
		},
		{
			name:    "Happy path: a higher level slot tracks its own pool",
			updates: map[string]any{"spellcasting.slot9.total": float64(1), "spellcasting.slot9.spent": float64(1)},
			field:   "spellcasting.slot9.remaining",
			want:    0,
		},
		{
			name:    "Edge: spending past the slots leaves a zero remaining",
			updates: map[string]any{"spellcasting.slot2.total": float64(1), "spellcasting.slot2.spent": float64(4)},
			field:   "spellcasting.slot2.remaining",
			want:    0,
		},
		{
			name:    "Happy path: a Pact Magic remaining subtracts the used slots",
			updates: map[string]any{"spellcasting.pact.count": float64(2), "spellcasting.pact.used": float64(1)},
			field:   "spellcasting.pact.remaining",
			want:    1,
		},
		{
			name:    "Edge: using past the Pact Magic pool leaves a zero remaining",
			updates: map[string]any{"spellcasting.pact.count": float64(2), "spellcasting.pact.used": float64(3)},
			field:   "spellcasting.pact.remaining",
			want:    0,
		},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, character, tc.updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, tc.field), 1e-9)
		})
	}
}

func TestFifthSpellcasting_Labels(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthSpellcastingSection].Children
	for level := 1; level <= 9; level++ {
		key := fifthSpellSlotKey(level)
		require.NotNil(t, group[key].Children, key)
		assert.Equal(t, fmt.Sprintf("Level %d", level), group[key].Label)
	}
	assert.Equal(t, "Pact Magic", group[fifthPactKey].Label)
}
