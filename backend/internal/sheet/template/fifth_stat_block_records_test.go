package template

import (
	"fmt"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fifthTestRecordID = "r0123456789abcdef0123456789abcdef"

func fifthStatBlockRecord(t *testing.T, data map[string]any, listPath string) map[string]any {
	t.Helper()
	stored, ok := data[listPath]
	require.True(t, ok, "missing list %s", listPath)
	records, ok := stored.(map[string]any)
	require.True(t, ok, "list %s is not a map", listPath)
	record, ok := records[fifthTestRecordID].(map[string]any)
	require.True(t, ok, "missing record in %s", listPath)
	return record
}

func TestFifthStatBlock_RecordLists(t *testing.T) {
	tests := []struct {
		name    string
		list    string
		label   string
		columns []string
		shape   []string
	}{
		{
			name:    "Happy path: traits hold a name and text",
			list:    fifthStatBlockTraitsSection,
			label:   "Traits",
			columns: []string{fieldNameName, fifthStatBlockRecordTextKey},
			shape:   []string{fieldNameName, fifthStatBlockRecordTextKey},
		},
		{
			name:    "Happy path: actions hold the attack fields",
			list:    fifthStatBlockActionsSection,
			label:   "Actions",
			columns: []string{fieldNameName, fifthStatBlockRecordToHitKey, fifthStatBlockRecordDamageKey, fifthStatBlockRecordNotesKey},
			shape: []string{
				fieldNameName, fifthStatBlockRecordToHitKey, fifthStatBlockRecordDamageKey, fifthStatBlockRecordNotesKey,
				fifthStatBlockRecordProficientKey, fifthStatBlockRecordMiscKey,
				"abilityStrength", "abilityDexterity", "abilityConstitution",
				"abilityIntelligence", "abilityWisdom", "abilityCharisma",
			},
		},
		{
			name:    "Happy path: bonus actions hold a name and text",
			list:    fifthStatBlockBonusActionsSection,
			label:   "Bonus actions",
			columns: []string{fieldNameName, fifthStatBlockRecordTextKey},
			shape:   []string{fieldNameName, fifthStatBlockRecordTextKey},
		},
		{
			name:    "Happy path: reactions hold a name and text",
			list:    fifthStatBlockReactionsSection,
			label:   "Reactions",
			columns: []string{fieldNameName, fifthStatBlockRecordTextKey},
			shape:   []string{fieldNameName, fifthStatBlockRecordTextKey},
		},
		{
			name:    "Happy path: legendary actions hold a name and text",
			list:    fifthStatBlockLegendaryActionsSection,
			label:   "Legendary actions",
			columns: []string{fieldNameName, fifthStatBlockRecordTextKey},
			shape:   []string{fieldNameName, fifthStatBlockRecordTextKey},
		},
		{
			name:    "Happy path: resources hold uses and recharge",
			list:    fifthStatBlockResourcesSection,
			label:   "Resources",
			columns: []string{fifthStatBlockRecordNameKey, fifthStatBlockRecordMaxKey, fifthStatBlockRecordUsedKey, fifthStatBlockRecordRemainingKey, fifthStatBlockRecordRechargeKey},
			shape:   []string{fifthStatBlockRecordNameKey, fifthStatBlockRecordMaxKey, fifthStatBlockRecordUsedKey, fifthStatBlockRecordRemainingKey, fifthStatBlockRecordRechargeKey},
		},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, ok := statBlock.Fields[tc.list]
			require.True(t, ok, "missing list section %s", tc.list)
			require.NotNil(t, node.List)

			list := node.List
			assert.Equal(t, tc.label, list.Label)
			assert.Equal(t, tc.columns, list.Columns)
			assert.Equal(t, fifthStatBlockRecordCapacity, list.Capacity)
			assert.ElementsMatch(t, tc.shape, groupKeys(list.Shape))
			for name, shape := range list.Shape {
				require.True(t, shape.IsLeaf(), "%s %s is not a leaf", tc.list, name)
				assert.NotEmpty(t, shape.Schema.Label)
			}
			assert.Contains(t, statBlock.SectionOrder, tc.list)
		})
	}
}

func TestFifthStatBlock_ActionToHit(t *testing.T) {
	tests := []struct {
		name       string
		ability    string
		proficient float64
		misc       float64
		want       float64
	}{
		{name: "Happy path: Strength with proficiency and misc", ability: abilityStrength, proficient: 1, misc: 1, want: 7},
		{name: "Happy path: Dexterity without proficiency", ability: abilityDexterity, proficient: 0, misc: 0, want: 3},
		{name: "Happy path: Constitution with proficiency", ability: abilityConstitution, proficient: 1, misc: 0, want: 6},
		{name: "Happy path: Intelligence without proficiency and a misc bonus", ability: abilityIntelligence, proficient: 0, misc: 2, want: 5},
		{name: "Happy path: Wisdom with proficiency and a negative misc", ability: abilityWisdom, proficient: 1, misc: -1, want: 5},
		{name: "Happy path: Charisma without proficiency and a negative misc", ability: abilityCharisma, proficient: 0, misc: -2, want: 1},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abilityField := "actions." + fifthTestRecordID + ".ability" + upperFirst(tc.ability)
			data := fifthSheet(t, statBlock, map[string]any{
				"identity.challengeRating":                     float64(5),
				"abilities." + tc.ability + ".score":           float64(16),
				"actions." + fifthTestRecordID + ".name":       "Slam",
				abilityField:                                   1.0,
				"actions." + fifthTestRecordID + ".proficient": tc.proficient,
				"actions." + fifthTestRecordID + ".misc":       tc.misc,
			})
			assert.InDelta(t, tc.want, fifthValue(t, data, "actions."+fifthTestRecordID+".toHit"), 1e-9)
		})
	}

	t.Run("Edge: no ability chosen leaves the misc bonus", func(t *testing.T) {
		data := fifthSheet(t, statBlock, map[string]any{
			"identity.challengeRating":               float64(5),
			"actions." + fifthTestRecordID + ".name": "Slam",
			"actions." + fifthTestRecordID + ".misc": float64(2),
		})
		assert.InDelta(t, 2.0, fifthValue(t, data, "actions."+fifthTestRecordID+".toHit"), 1e-9)
	})

	t.Run("Happy path: the proficient flag reads the challenge rating bonus", func(t *testing.T) {
		low := fifthSheet(t, statBlock, map[string]any{
			"identity.challengeRating":                          float64(1),
			"abilities.strength.score":                          float64(16),
			"actions." + fifthTestRecordID + ".abilityStrength": 1.0,
			"actions." + fifthTestRecordID + ".proficient":      1.0,
		})
		high := fifthSheet(t, statBlock, map[string]any{
			"identity.challengeRating":                          float64(17),
			"abilities.strength.score":                          float64(16),
			"actions." + fifthTestRecordID + ".abilityStrength": 1.0,
			"actions." + fifthTestRecordID + ".proficient":      1.0,
		})
		assert.InDelta(t, 5.0, fifthValue(t, low, "actions."+fifthTestRecordID+".toHit"), 1e-9)
		assert.InDelta(t, 9.0, fifthValue(t, high, "actions."+fifthTestRecordID+".toHit"), 1e-9)
	})
}

func TestFifthStatBlock_ActionDice(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	list := statBlock.Fields[fifthStatBlockActionsSection].List
	require.NotNil(t, list)

	t.Run("Happy path: the action roll is a d20 test with the exhaustion penalty", func(t *testing.T) {
		assert.Equal(t, "1d20 + @.toHit + exhaustion.penalty", list.Dice)
		assert.LessOrEqual(t, len(list.Dice), 64)
		assert.False(t, list.NotD20)
	})

	t.Run("Happy path: the action dice defaults validate", func(t *testing.T) {
		require.NoError(t, ValidateDiceDefaults(statBlock))
	})
}

func TestFifthStatBlock_Resources(t *testing.T) {
	tests := []struct {
		name string
		max  float64
		used float64
		want float64
	}{
		{name: "Happy path: an unused resource keeps its maximum", max: 3, used: 0, want: 3},
		{name: "Happy path: a spent use lowers the remaining count", max: 3, used: 1, want: 2},
		{name: "Edge: a fully spent resource remains zero", max: 1, used: 1, want: 0},
		{name: "Edge: more uses than the maximum go negative", max: 2, used: 5, want: -3},
		{name: "Edge: a fresh resource seeds zero", max: 0, used: 0, want: 0},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, statBlock, map[string]any{
				"resources." + fifthTestRecordID + ".name":     "Legendary Resistance",
				"resources." + fifthTestRecordID + ".max":      tc.max,
				"resources." + fifthTestRecordID + ".used":     tc.used,
				"resources." + fifthTestRecordID + ".recharge": "Recharge 5-6",
			})
			assert.InDelta(t, tc.want, fifthValue(t, data, "resources."+fifthTestRecordID+".remaining"), 1e-9)
			assert.Equal(t, "Recharge 5-6", fifthStatBlockRecord(t, data, fifthStatBlockResourcesSection)[fifthStatBlockRecordRechargeKey])
		})
	}
}

func TestFifthStatBlock_Details(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		label string
	}{
		{name: "Happy path: damage resistances are text", key: fifthStatBlockDetailsResistancesKey, label: "Damage resistances"},
		{name: "Happy path: damage immunities are text", key: fifthStatBlockDetailsImmunitiesKey, label: "Damage immunities"},
		{name: "Happy path: senses are text", key: fifthStatBlockDetailsSensesKey, label: "Senses"},
		{name: "Happy path: languages are text", key: fifthStatBlockDetailsLanguagesKey, label: "Languages"},
	}

	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	group := statBlock.Fields[fifthStatBlockDetailsSection].Children

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, ok := group[tc.key]
			require.True(t, ok)
			require.NotNil(t, node.Schema)
			assert.Equal(t, fieldTypeText, node.Schema.Type)
			assert.True(t, node.Schema.IsEditable)
			assert.Equal(t, tc.label, node.Schema.Label)
			require.NotNil(t, node.Schema.MaxLength)
			assert.Positive(t, *node.Schema.MaxLength)
			assert.Contains(t, statBlock.Fields[fifthStatBlockDetailsSection].Order, tc.key)
		})
	}
}

func TestFifthStatBlock_RecordBootValidation(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)

	t.Run("Happy path: every record list builds and validates at boot", func(t *testing.T) {
		_, err := BuildDAG(statBlock.ID, statBlock.Fields, engine.NewFormulaEngine())
		require.NoError(t, err)
		require.NoError(t, ValidateFlags(statBlock))
		require.NoError(t, ValidateDiceDefaults(statBlock))
		require.NoError(t, ValidateNotD20Tests(statBlock))
	})

	t.Run("Happy path: the registry accepts the shipped template", func(t *testing.T) {
		_, err := BuildRegistry(engine.NewFormulaEngine(), WithDraftsAllowed(true))
		require.NoError(t, err)
	})

	t.Run("Edge: the action abilities form one exclusive choice group", func(t *testing.T) {
		shape := statBlock.Fields[fifthStatBlockActionsSection].List.Shape
		for _, ability := range abilities {
			node, ok := shape[fifthAbilityFlagKey(ability.key)]
			require.True(t, ok, ability.key)
			require.NotNil(t, node.Schema)
			assert.Equal(t, fieldTypeNumber, node.Schema.Type, ability.key)
			assert.True(t, node.Schema.IsEditable, ability.key)
			assert.Equal(t, fifthStatBlockActionAbilityGroup, node.Schema.ExclusiveGroup, ability.key)
			assert.Equal(t, ability.label, node.Schema.OptionLabel, ability.key)
			assert.InDelta(t, 0.0, node.Schema.Default, 1e-9, ability.key)
		}
	})

	t.Run("Edge: two abilities on in one record are refused", func(t *testing.T) {
		_, err := fifthDAG(t, statBlock).CalculateSave(fifthSeedFields(statBlock.Fields, true), map[string]any{
			"actions." + fifthTestRecordID + ".abilityStrength":  1.0,
			"actions." + fifthTestRecordID + ".abilityDexterity": 1.0,
		})
		var conflict *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &conflict)
		assert.Equal(t, fifthStatBlockActionAbilityGroup, conflict.Group)
	})

	t.Run("Happy path: the action to-hit is a derived record field", func(t *testing.T) {
		schema := statBlock.Fields[fifthStatBlockActionsSection].List.Shape[fifthStatBlockRecordToHitKey].Schema
		require.NotNil(t, schema)
		assert.False(t, schema.IsEditable)
		assert.Contains(t, schema.Formula, "identity_proficiencyBonus")
		assert.Contains(t, schema.Formula, "abilities_strength_mod")
	})

	t.Run("Happy path: the remaining uses are a derived record field", func(t *testing.T) {
		schema := statBlock.Fields[fifthStatBlockResourcesSection].List.Shape[fifthStatBlockRecordRemainingKey].Schema
		require.NotNil(t, schema)
		assert.False(t, schema.IsEditable)
		assert.Contains(t, schema.Formula, "record_max")
		assert.Contains(t, schema.Formula, "record_used")
	})
}

func TestFifthStatBlock_RecordCapacity(t *testing.T) {
	statBlock := fifthTemplateByID(t, fifthStatBlockID)
	list := statBlock.Fields[fifthStatBlockTraitsSection].List
	require.NotNil(t, list)

	t.Run("Happy path: capacity is bounded by the shared limit", func(t *testing.T) {
		assert.Equal(t, fifthStatBlockRecordCapacity, list.Capacity)
		assert.GreaterOrEqual(t, list.Capacity, MinListCapacity)
		assert.LessOrEqual(t, list.Capacity, MaxListCapacity)
	})

	t.Run("Edge: a write above capacity is refused", func(t *testing.T) {
		updates := map[string]any{}
		for i := range list.Capacity + 1 {
			id := fmt.Sprintf("r%032x", i)
			updates["traits."+id+".name"] = "Trait"
		}
		_, err := fifthDAG(t, statBlock).CalculateSave(fifthSeedFields(statBlock.Fields, true), updates)
		var capacity *ListCapacityExceededError
		require.ErrorAs(t, err, &capacity)
	})
}
