package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFifthCharacterCombat_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthCombatSection].Children

	t.Run("Happy path: initiative enters Alert and misc behind a derived total", func(t *testing.T) {
		initiative := group[fifthInitiativeKey]
		require.NotNil(t, initiative.Children)
		assert.Equal(t, fifthInitiativeOrder, initiative.Columns)

		alert := initiative.Children[fifthAlertKey].Schema
		require.NotNil(t, alert)
		assert.True(t, alert.IsEditable)
		assert.True(t, alert.Flag)
		assert.InDelta(t, 0.0, alert.Default, 1e-9)

		misc := initiative.Children[fifthMiscKey].Schema
		require.NotNil(t, misc)
		assert.True(t, misc.IsEditable)

		total := initiative.Children[fieldNameTotal].Schema
		require.NotNil(t, total)
		assert.False(t, total.IsEditable)
		assert.NotEmpty(t, total.Formula)
	})

	t.Run("Happy path: speed enters a base and derives the total", func(t *testing.T) {
		speed := group[fifthSpeedKey]
		require.NotNil(t, speed.Children)

		base := speed.Children[fifthSpeedBaseKey].Schema
		require.NotNil(t, base)
		assert.True(t, base.IsEditable)
		assert.InDelta(t, 30.0, base.Default, 1e-9)

		total := speed.Children[fieldNameTotal].Schema
		require.NotNil(t, total)
		assert.False(t, total.IsEditable)
		assert.NotEmpty(t, total.Formula)
	})

	t.Run("Happy path: the heavy armor requirement is an entered number", func(t *testing.T) {
		requirement := group[fifthArmorStrengthRequirementKey].Schema
		require.NotNil(t, requirement)
		assert.True(t, requirement.IsEditable)
		assert.InDelta(t, 0.0, requirement.Default, 1e-9)
	})

	t.Run("Happy path: hit points enter base, per level, misc, current and temporary", func(t *testing.T) {
		hp := group[fifthHitPointsKey]
		require.NotNil(t, hp.Children)
		assert.Equal(t, fifthHPOrder, hp.Columns)

		for _, key := range []string{fifthHPBaseKey, fifthHPPerLevelKey, fifthHPMiscKey, fifthHPCurrentKey, fifthHPTempKey} {
			schema := hp.Children[key].Schema
			require.NotNil(t, schema, key)
			assert.True(t, schema.IsEditable, key)
		}

		maximum := hp.Children[fifthHPMaxKey].Schema
		require.NotNil(t, maximum)
		assert.False(t, maximum.IsEditable)
		assert.NotEmpty(t, maximum.Formula)
	})

	t.Run("Happy path: carrying capacity is derived from Strength", func(t *testing.T) {
		carrying := group[fifthCarryingKey].Schema
		require.NotNil(t, carrying)
		assert.False(t, carrying.IsEditable)
		assert.NotEmpty(t, carrying.Formula)
	})

	t.Run("Happy path: initiative and death saves carry the Exhaustion penalty dice", func(t *testing.T) {
		require.NoError(t, ValidateDiceDefaults(tmpl))
		assert.Equal(t, fifthCharacterRowDice(), tmpl.RowDice)

		for row, expression := range fifthCharacterRowDice() {
			assert.Contains(t, expression, "exhaustion.penalty", row)
			assert.LessOrEqual(t, len(expression), 64, row)
			assert.NotContains(t, tmpl.NotD20Rows, row)
		}
	})
}

func TestFifthCharacterInitiative(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	tests := []struct {
		name    string
		updates map[string]any
		want    float64
	}{
		{
			name:    "Happy path: initiative is the dexterity modifier plus misc",
			updates: map[string]any{"abilities.dexterity.score": float64(14)},
			want:    2,
		},
		{
			name: "Happy path: Alert adds the proficiency bonus",
			updates: map[string]any{
				"identity.level":            float64(1),
				"combat.initiative.alert":   float64(1),
				"abilities.dexterity.score": float64(14),
				"combat.initiative.misc":    float64(0),
			},
			want: 4,
		},
		{
			name: "Happy path: Alert at level 5 adds three and misc stacks",
			updates: map[string]any{
				"identity.level":            float64(5),
				"combat.initiative.alert":   float64(1),
				"abilities.dexterity.score": float64(16),
				"combat.initiative.misc":    float64(1),
			},
			want: 7,
		},
		{
			name: "Edge: Jack of All Trades never applies to initiative",
			updates: map[string]any{
				"identity.level":            float64(5),
				"identity.jackOfAllTrades":  float64(1),
				"abilities.dexterity.score": float64(16),
			},
			want: 3,
		},
		{
			name: "Edge: Exhaustion never changes the initiative value",
			updates: map[string]any{
				"identity.level":            float64(5),
				"abilities.dexterity.score": float64(16),
				"exhaustion.level":          float64(3),
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, tmpl, tc.updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, "combat.initiative.total"), 1e-9)
		})
	}
}

func TestFifthCharacterSpeed(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	tests := []struct {
		name    string
		updates map[string]any
		want    float64
	}{
		{
			name:    "Happy path: speed is the base without penalties",
			updates: map[string]any{},
			want:    30,
		},
		{
			name: "Happy path: heavy armor and Strength below the requirement slows by ten",
			updates: map[string]any{
				"armor.choiceHeavy":               float64(1),
				"combat.armorStrengthRequirement": float64(13),
				"abilities.strength.score":        float64(12),
			},
			want: 20,
		},
		{
			name: "Edge: Strength at the requirement takes no penalty",
			updates: map[string]any{
				"armor.choiceHeavy":               float64(1),
				"combat.armorStrengthRequirement": float64(13),
				"abilities.strength.score":        float64(13),
			},
			want: 30,
		},
		{
			name: "Happy path: Strength above the requirement takes no penalty",
			updates: map[string]any{
				"armor.choiceHeavy":               float64(1),
				"combat.armorStrengthRequirement": float64(15),
				"abilities.strength.score":        float64(16),
			},
			want: 30,
		},
		{
			name: "Edge: a zero requirement never penalizes",
			updates: map[string]any{
				"armor.choiceHeavy":               float64(1),
				"combat.armorStrengthRequirement": float64(0),
				"abilities.strength.score":        float64(8),
			},
			want: 30,
		},
		{
			name: "Edge: a requirement left on non-heavy armor is ignored",
			updates: map[string]any{
				"armor.choiceLight":               float64(1),
				"combat.armorStrengthRequirement": float64(15),
				"abilities.strength.score":        float64(8),
			},
			want: 30,
		},
		{
			name: "Happy path: Exhaustion and the armor penalty combine",
			updates: map[string]any{
				"armor.choiceHeavy":               float64(1),
				"combat.armorStrengthRequirement": float64(13),
				"abilities.strength.score":        float64(12),
				"exhaustion.level":                float64(2),
			},
			want: 10,
		},
		{
			name: "Edge: speed has no floor",
			updates: map[string]any{
				"combat.speed.base": float64(10),
				"exhaustion.level":  float64(3),
			},
			want: -5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, tmpl, tc.updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, "combat.speed.total"), 1e-9)
		})
	}
}

func TestFifthCharacterHitPointMaximum(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	tests := []struct {
		name    string
		updates map[string]any
		want    float64
	}{
		{
			name: "Happy path: maximum adds Constitution per level, the per-level bonus and misc",
			updates: map[string]any{
				"identity.level":               float64(3),
				"abilities.constitution.score": float64(14),
				"combat.hp.base":               float64(8),
				"combat.hp.perLevel":           float64(5),
				"combat.hp.misc":               float64(2),
			},
			want: 31,
		},
		{
			name:    "Edge: the defaults give a zero maximum",
			updates: map[string]any{},
			want:    0,
		},
		{
			name: "Edge: a negative Constitution lowers the maximum",
			updates: map[string]any{
				"identity.level":               float64(2),
				"abilities.constitution.score": float64(6),
				"combat.hp.base":               float64(10),
			},
			want: 6,
		},
		{
			name: "Happy path: current and temporary hit points never change the maximum",
			updates: map[string]any{
				"identity.level":    float64(1),
				"combat.hp.base":    float64(10),
				"combat.hp.current": float64(4),
				"combat.hp.temp":    float64(5),
			},
			want: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := fifthSheet(t, tmpl, tc.updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, "combat.hp.max"), 1e-9)
		})
	}
}

func TestFifthCharacterCarryingCapacity(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	tests := []struct {
		strength float64
		want     float64
	}{
		{strength: 8, want: 120},
		{strength: 10, want: 150},
		{strength: 20, want: 300},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("Happy path: Strength %v carries %v pounds", tc.strength, tc.want), func(t *testing.T) {
			data := fifthSheet(t, tmpl, map[string]any{"abilities.strength.score": tc.strength})
			assert.InDelta(t, tc.want, fifthValue(t, data, "combat.carryingCapacity"), 1e-9)
		})
	}
}

func TestFifthCharacterStatus_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthStatusSection].Children

	t.Run("Happy path: experience points are entered independently of level", func(t *testing.T) {
		xp := group[fifthXPKey].Schema
		require.NotNil(t, xp)
		assert.True(t, xp.IsEditable)
		assert.Empty(t, xp.Formula)
		assert.InDelta(t, 0.0, xp.Default, 1e-9)

		data := fifthSheet(t, tmpl, map[string]any{
			"identity.level": float64(4),
			"status.xp":      float64(6500),
		})
		assert.InDelta(t, 6500, fifthValue(t, data, "status.xp"), 1e-9)
		assert.InDelta(t, 4, fifthValue(t, data, "identity.level"), 1e-9)
	})

	t.Run("Happy path: Heroic Inspiration is a numeric flag", func(t *testing.T) {
		inspiration := group[fifthInspirationKey].Schema
		require.NotNil(t, inspiration)
		assert.True(t, inspiration.IsEditable)
		assert.True(t, inspiration.Flag)
		assert.InDelta(t, 0.0, inspiration.Default, 1e-9)
	})

	t.Run("Happy path: every SRD condition but Exhaustion is a labelled numeric flag", func(t *testing.T) {
		conditions := group[fifthConditionsKey].Children
		require.Len(t, conditions, len(fifthConditions))
		for _, condition := range fifthConditions {
			schema := conditions[condition.key].Schema
			require.NotNil(t, schema, condition.key)
			assert.True(t, schema.IsEditable, condition.key)
			assert.True(t, schema.Flag, condition.key)
			assert.InDelta(t, 0.0, schema.Default, 1e-9, condition.key)
			assert.Contains(t, schema.Label, ":", condition.key)
		}
		_, hasExhaustion := conditions["exhaustion"]
		assert.False(t, hasExhaustion)
	})

	t.Run("Happy path: death save counters are bounded entries with rule text", func(t *testing.T) {
		deathSaves := group[fifthDeathSavesKey]
		require.NotNil(t, deathSaves.Children)
		assert.Contains(t, deathSaves.Label, "10 or higher succeeds")

		for _, key := range []string{fifthSuccessesKey, fifthFailuresKey} {
			schema := deathSaves.Children[key].Schema
			require.NotNil(t, schema, key)
			assert.True(t, schema.IsEditable, key)
			require.NotNil(t, schema.Min, key)
			require.NotNil(t, schema.Max, key)
			assert.InDelta(t, 0.0, *schema.Min, 1e-9, key)
			assert.InDelta(t, 3.0, *schema.Max, 1e-9, key)
		}
	})
}

func TestFifthCharacterCoins(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthCoinsSection].Children

	t.Run("Happy path: every coin is an entered number defaulting to zero", func(t *testing.T) {
		require.Len(t, group, len(fifthCoinsOrder))
		for _, coin := range fifthCoinsOrder {
			schema := group[coin].Schema
			require.NotNil(t, schema, coin)
			assert.True(t, schema.IsEditable, coin)
			assert.Equal(t, fieldTypeNumber, schema.Type, coin)
			assert.InDelta(t, 0.0, schema.Default, 1e-9, coin)
		}
	})

	t.Run("Edge: coins are independent entries", func(t *testing.T) {
		data := fifthSheet(t, tmpl, map[string]any{"coins.gp": float64(25)})
		assert.InDelta(t, 25, fifthValue(t, data, "coins.gp"), 1e-9)
		for _, coin := range []string{"cp", "sp", "ep", "pp"} {
			assert.InDelta(t, 0, fifthValue(t, data, "coins."+coin), 1e-9, coin)
		}
	})
}
