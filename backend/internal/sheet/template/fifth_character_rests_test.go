package template

import (
	"slices"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fifthCharacterResetDAG(t *testing.T) *DAG {
	t.Helper()
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	dag, err := BuildDAGWithActions(tmpl.ID, tmpl.Fields, tmpl.Actions, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func fifthSpentCharacterUpdates() map[string]any {
	updates := map[string]any{
		"identity.level":                                     float64(5),
		"abilities.constitution.score":                       float64(14),
		"combat.hp.base":                                     float64(10),
		"combat.hp.perLevel":                                 float64(6),
		"combat.hp.misc":                                     float64(5),
		"combat.hp.current":                                  float64(4),
		"hitDice.primary.spent":                              float64(2),
		"hitDice.d6.extra":                                   float64(2),
		"hitDice.d6.spent":                                   float64(2),
		"hitDice.d8.extra":                                   float64(1),
		"hitDice.d8.spent":                                   float64(1),
		"exhaustion.level":                                   float64(3),
		"spellcasting.pact.count":                            float64(2),
		"spellcasting.pact.level":                            float64(3),
		"spellcasting.pact.used":                             float64(2),
		"status.deathSaves.successes":                        float64(2),
		"status.deathSaves.failures":                         float64(1),
		"resources.list." + validRecordID + ".max":           float64(3),
		"resources.list." + validRecordID + ".used":          float64(2),
		"resources.list." + validRecordID + ".rechargeShort": float64(1),
		"resources.list." + secondRecordID + ".max":          float64(2),
		"resources.list." + secondRecordID + ".used":         float64(1),
		"resources.list." + secondRecordID + ".rechargeLong": float64(1),
		"resources.list." + thirdRecordID + ".max":           float64(5),
		"resources.list." + thirdRecordID + ".used":          float64(3),
		"resources.list." + thirdRecordID + ".rechargeNone":  float64(1),
	}

	totals := []float64{4, 3, 2, 1, 1, 1, 1, 1, 1}
	spent := []float64{3, 2, 1, 1, 1, 1, 1, 1, 1}
	for level := 1; level <= 9; level++ {
		key := Path{fifthSpellcastingSection, fifthSpellSlotKey(level)}.String()
		updates[key+".total"] = totals[level-1]
		updates[key+".spent"] = spent[level-1]
	}
	return updates
}

func fifthMergeUpdates(base, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func TestFifthCharacterRestActions_CalculateReset(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	dag := fifthCharacterResetDAG(t)
	base := fifthSpentCharacterUpdates()

	tests := []struct {
		name     string
		actionID string
		updates  map[string]any
		want     map[string]float64
	}{
		{
			name:     "Happy path: a long rest restores hit points, every die pool, every slot, Pact Magic, Exhaustion, death saves and both recharging resource kinds",
			actionID: fifthLongRestActionID,
			want: map[string]float64{
				"combat.hp.current":                          49,
				"hitDice.primary.spent":                      0,
				"hitDice.d6.spent":                           0,
				"hitDice.d8.spent":                           0,
				"hitDice.d10.spent":                          0,
				"hitDice.d12.spent":                          0,
				"hitDice.primary.remaining":                  2,
				"hitDice.d6.remaining":                       2,
				"hitDice.d8.remaining":                       1,
				"spellcasting.slot1.spent":                   0,
				"spellcasting.slot2.spent":                   0,
				"spellcasting.slot3.spent":                   0,
				"spellcasting.slot4.spent":                   0,
				"spellcasting.slot5.spent":                   0,
				"spellcasting.slot6.spent":                   0,
				"spellcasting.slot7.spent":                   0,
				"spellcasting.slot8.spent":                   0,
				"spellcasting.slot9.spent":                   0,
				"spellcasting.slot1.remaining":               4,
				"spellcasting.slot9.remaining":               1,
				"spellcasting.pact.used":                     0,
				"spellcasting.pact.remaining":                2,
				"exhaustion.level":                           2,
				"exhaustion.penalty":                         -4,
				"status.deathSaves.successes":                0,
				"status.deathSaves.failures":                 0,
				"resources.list." + validRecordID + ".used":  0,
				"resources.list." + secondRecordID + ".used": 0,
				"resources.list." + thirdRecordID + ".used":  3,
			},
		},
		{
			name:     "Happy path: a short rest restores Pact Magic and only the short-recharge resources",
			actionID: fifthShortRestActionID,
			want: map[string]float64{
				"spellcasting.pact.used":                     0,
				"spellcasting.pact.remaining":                2,
				"resources.list." + validRecordID + ".used":  0,
				"resources.list." + secondRecordID + ".used": 1,
				"resources.list." + thirdRecordID + ".used":  3,
				"combat.hp.current":                          4,
				"hitDice.primary.spent":                      2,
				"spellcasting.slot1.spent":                   3,
				"exhaustion.level":                           3,
				"status.deathSaves.successes":                2,
			},
		},
		{
			name:     "Edge: a long rest at Exhaustion 0 keeps it 0",
			actionID: fifthLongRestActionID,
			updates:  map[string]any{"exhaustion.level": float64(0)},
			want: map[string]float64{
				"exhaustion.level":   0,
				"exhaustion.penalty": 0,
				"combat.hp.current":  49,
			},
		},
		{
			name:     "Edge: a long rest leaves a no-recharge resource used",
			actionID: fifthLongRestActionID,
			want:     map[string]float64{"resources.list." + thirdRecordID + ".used": 3},
		},
		{
			name:     "Edge: a short rest leaves a no-recharge resource used",
			actionID: fifthShortRestActionID,
			want:     map[string]float64{"resources.list." + thirdRecordID + ".used": 3},
		},
		{
			name:     "Edge: a short rest leaves a long-recharge resource used",
			actionID: fifthShortRestActionID,
			want:     map[string]float64{"resources.list." + secondRecordID + ".used": 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, tmpl, fifthMergeUpdates(base, tc.updates))

			calculation, err := dag.CalculateReset(tc.actionID, data)
			require.NoError(t, err)
			for field, want := range tc.want {
				assert.InDelta(t, want, fifthValue(t, calculation.Merged, field), 1e-9, field)
			}
		})
	}
}

func TestFifthCharacterRestActions_EdgeCases(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	dag := fifthCharacterResetDAG(t)

	t.Run("Edge: a long rest at Exhaustion 0 reports no change for the level", func(t *testing.T) {
		data := fifthSheet(t, tmpl, fifthMergeUpdates(
			fifthSpentCharacterUpdates(),
			map[string]any{"exhaustion.level": float64(0)},
		))

		calculation, err := dag.CalculateReset(fifthLongRestActionID, data)
		require.NoError(t, err)
		assert.NotContains(t, calculation.ChangedKeys, "exhaustion.level")
	})

	t.Run("Edge: a long rest on a fully rested sheet changes no key", func(t *testing.T) {
		data := fifthSheet(t, tmpl, fifthMergeUpdates(
			fifthSpentCharacterUpdates(),
			map[string]any{"exhaustion.level": float64(0)},
		))

		first, err := dag.CalculateReset(fifthLongRestActionID, data)
		require.NoError(t, err)
		require.NotEmpty(t, first.ChangedKeys)

		second, err := dag.CalculateReset(fifthLongRestActionID, first.Merged)
		require.NoError(t, err)
		assert.Empty(t, second.ChangedKeys)
	})

	t.Run("Happy path: the template declares the long rest then the short rest", func(t *testing.T) {
		require.Len(t, tmpl.Actions, 2)
		assert.Equal(t, fifthLongRestActionID, tmpl.Actions[0].ID)
		assert.Equal(t, "Long rest", tmpl.Actions[0].Label)
		assert.Equal(t, fifthShortRestActionID, tmpl.Actions[1].ID)
		assert.Equal(t, "Short rest", tmpl.Actions[1].Label)
	})
}

func TestFifthCharacter_RollModes(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	require.NoError(t, ValidateNotD20Tests(tmpl))

	dto := tmpl.ToDTO()
	require.NotNil(t, dto.Dice)
	require.NotNil(t, dto.Dice.NotD20)

	d20Sections := []string{fifthAbilitiesSection, fifthSavesSection, fifthSkillsSection}
	for _, section := range d20Sections {
		assert.Contains(t, tmpl.SectionDice, section)
		assert.NotContains(t, dto.Dice.NotD20.Section, section)
		rows := fifthSectionRows(t, tmpl, section)
		require.NotEmpty(t, rows, section)
		for _, row := range rows {
			assert.NotContains(t, dto.Dice.NotD20.Row, row)
		}
	}

	d20Rows := []string{
		Path{fifthCombatSection, fifthInitiativeKey}.String(),
		Path{fifthStatusSection, fifthDeathSavesKey}.String(),
		Path{fifthSpellcastingSection, fifthSpellAttackKey}.String(),
	}
	for _, row := range d20Rows {
		assert.Contains(t, tmpl.RowDice, row)
		assert.NotContains(t, dto.Dice.NotD20.Row, row)
	}

	attacks := dto.Fields[fifthAttacksSection].Children[fifthRecordListKey].List
	require.NotNil(t, attacks)
	assert.NotEmpty(t, attacks.Dice)
	assert.False(t, attacks.NotD20)

	hitDiceRows := fifthSectionRows(t, tmpl, fifthHitDiceSection)
	require.ElementsMatch(t, []string{
		"hitDice.primary", "hitDice.d6", "hitDice.d8", "hitDice.d10", "hitDice.d12",
	}, hitDiceRows)
	for _, row := range hitDiceRows {
		marked := slices.Contains(dto.Dice.NotD20.Section, fifthHitDiceSection) ||
			slices.Contains(dto.Dice.NotD20.Row, row)
		assert.True(t, marked, row)
	}
}
