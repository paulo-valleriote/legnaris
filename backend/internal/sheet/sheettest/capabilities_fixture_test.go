package sheettest_test

import (
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/sheettest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const epsilon = 1e-9

func mergedValue(t *testing.T, data map[string]any, path ...string) any {
	t.Helper()

	var current any = data
	for _, key := range path {
		nested, ok := current.(map[string]any)
		require.True(t, ok, "path %v is not an object at %q", path, key)
		current = nested[key]
	}
	return current
}

func mergedNumber(t *testing.T, data map[string]any, path ...string) float64 {
	t.Helper()

	value, ok := mergedValue(t, data, path...).(float64)
	require.True(t, ok, "path %v is not a number", path)
	return value
}

func TestCapabilitiesTemplate_Boots(t *testing.T) {
	sheettest.CapabilitiesBoot(t)
}

func TestCapabilitiesTemplate_DTO(t *testing.T) {
	t.Run("Happy path: the Template accepts npc and enemy but not character", func(t *testing.T) {
		dto := sheettest.CapabilitiesTemplate().ToDTO()

		assert.Equal(t, []plan.SheetKind{plan.KindNPC, plan.KindEnemy}, dto.Kinds)
		assert.NotContains(t, dto.Kinds, plan.KindCharacter)
	})

	t.Run("Happy path: the DTO carries the Field-sized dice and the not-d20 Row", func(t *testing.T) {
		dto := sheettest.CapabilitiesTemplate().ToDTO()

		require.NotNil(t, dto.Dice)
		assert.Equal(t, sheettest.CapabilitiesFieldDice, dto.Dice.Section["stats"])
		assert.Equal(t, sheettest.CapabilitiesRowDice, dto.Dice.Row["stats.insight"])
		require.NotNil(t, dto.Dice.NotD20)
		assert.Equal(t, []string{"stats.insight"}, dto.Dice.NotD20.Row)
	})

	t.Run("Happy path: the DTO carries the filtered reset action", func(t *testing.T) {
		dto := sheettest.CapabilitiesTemplate().ToDTO()

		require.Len(t, dto.Actions, 1)
		assert.Equal(t, sheettest.CapabilitiesActionRest, dto.Actions[0].ID)
		assert.Equal(t, sheettest.CapabilitiesActionRestLabel, dto.Actions[0].Label)
		require.Len(t, dto.Actions[0].Assignments, 2)
		assert.Equal(t, "stats.level", dto.Actions[0].Assignments[0].Target)
		assert.Empty(t, dto.Actions[0].Assignments[0].Filter)
		assert.Equal(t, sheettest.CapabilitiesListPath+".quantity", dto.Actions[0].Assignments[1].Target)
		assert.Equal(t, "active", dto.Actions[0].Assignments[1].Filter)
	})

	t.Run("Happy path: the list declaration carries per-Record formulas and the aggregates", func(t *testing.T) {
		fields := sheettest.CapabilitiesFields()

		inventory := fields["inventory"].Children["items"]
		require.NotNil(t, inventory.List)
		shape := inventory.List.Shape
		require.NotNil(t, shape["derivedTotal"].Schema)
		assert.Equal(t, "record_quantity * record_weight", shape["derivedTotal"].Schema.Formula)
		require.NotNil(t, shape["scaled"].Schema)
		assert.Equal(t, "record_quantity * stats_level", shape["scaled"].Schema.Formula)
		assert.Equal(t, "pick", shape["optionA"].Schema.ExclusiveGroup)
		assert.Equal(t, "pick", shape["optionB"].Schema.ExclusiveGroup)
		assert.True(t, shape["active"].Schema.Flag)

		require.NotNil(t, fields["totals"].Children["inventoryValue"].Schema.Aggregate)
		require.NotNil(t, fields["totals"].Children["inventoryCount"].Schema.Aggregate)
	})
}

func TestCapabilitiesTemplate_ResetApplies(t *testing.T) {
	dag := sheettest.CapabilitiesDAG(t)

	t.Run("Happy path: the rest lowers the level and restocks only the active Record", func(t *testing.T) {
		calculation, err := dag.CalculateReset(
			sheettest.CapabilitiesActionRest,
			sheettest.CapabilitiesSeedData(),
		)
		require.NoError(t, err)

		assert.InEpsilon(t, 2.0, mergedNumber(t, calculation.Merged, "stats", "level"), epsilon)
		assert.InEpsilon(t, 4.0, mergedNumber(t, calculation.Merged, "stats", "insight"), epsilon)
		assert.InEpsilon(t, 3.0, mergedNumber(
			t, calculation.Merged, "inventory", "items", sheettest.CapabilitiesRecordActive, "quantity",
		), epsilon)
		assert.InEpsilon(t, 1.0, mergedNumber(
			t, calculation.Merged, "inventory", "items", sheettest.CapabilitiesRecordIdle, "quantity",
		), epsilon)
		assert.InEpsilon(t, 6.0, mergedNumber(
			t, calculation.Merged, "inventory", "items", sheettest.CapabilitiesRecordActive, "derivedTotal",
		), epsilon)
		assert.InEpsilon(t, 6.0, mergedNumber(
			t, calculation.Merged, "inventory", "items", sheettest.CapabilitiesRecordActive, "scaled",
		), epsilon)
		assert.InEpsilon(t, 2.0, mergedNumber(
			t, calculation.Merged, "inventory", "items", sheettest.CapabilitiesRecordIdle, "scaled",
		), epsilon)
		assert.InEpsilon(t, 7.0, mergedNumber(t, calculation.Merged, "totals", "inventoryValue"), epsilon)
		assert.InEpsilon(t, 2.0, mergedNumber(t, calculation.Merged, "totals", "inventoryCount"), epsilon)
	})

	t.Run("Happy path: the changed keys name the level and only the matching Record field", func(t *testing.T) {
		calculation, err := dag.CalculateReset(
			sheettest.CapabilitiesActionRest,
			sheettest.CapabilitiesSeedData(),
		)
		require.NoError(t, err)

		assert.Equal(t, []string{
			"inventory.items." + sheettest.CapabilitiesRecordActive + ".quantity",
			"stats.level",
		}, calculation.ChangedKeys)
	})

	t.Run("Edge: the level stops at one when the action is replayed over its own result", func(t *testing.T) {
		first, err := dag.CalculateReset(sheettest.CapabilitiesActionRest, sheettest.CapabilitiesSeedData())
		require.NoError(t, err)
		second, err := dag.CalculateReset(sheettest.CapabilitiesActionRest, first.Merged)
		require.NoError(t, err)

		assert.InEpsilon(t, 1.0, mergedNumber(t, second.Merged, "stats", "level"), epsilon)
	})

	t.Run("Error: an unknown action is refused", func(t *testing.T) {
		_, err := dag.CalculateReset("ghost", sheettest.CapabilitiesSeedData())

		require.Error(t, err)
	})
}
