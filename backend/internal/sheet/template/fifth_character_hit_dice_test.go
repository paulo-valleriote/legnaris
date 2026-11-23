package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFifthHitDice_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	group := tmpl.Fields[fifthHitDiceSection].Children

	t.Run("Happy path: the primary pool enters the size and derives the count", func(t *testing.T) {
		node := group[fifthHitDiePrimaryKey]
		require.NotNil(t, node.Children)
		assert.Equal(t, []string{
			fifthHitDieSizeKey, fifthHitDieCountKey, fifthHitDieSpentKey, fieldNameRemaining,
		}, node.Columns)

		size := node.Children[fifthHitDieSizeKey].Schema
		require.NotNil(t, size)
		assert.Equal(t, fieldTypeNumber, size.Type)
		assert.True(t, size.IsEditable)
		assert.InDelta(t, 8.0, size.Default, 1e-9)
		require.NotNil(t, size.Min)
		require.NotNil(t, size.Max)
		assert.InDelta(t, 4.0, *size.Min, 1e-9)
		assert.InDelta(t, 12.0, *size.Max, 1e-9)

		count := node.Children[fifthHitDieCountKey].Schema
		require.NotNil(t, count)
		assert.False(t, count.IsEditable)
		assert.Contains(t, count.Formula, "max(0,")
		assert.NotEmpty(t, count.DependsOn)

		spent := node.Children[fifthHitDieSpentKey].Schema
		require.NotNil(t, spent)
		assert.True(t, spent.IsEditable)
		assert.InDelta(t, 0.0, spent.Default, 1e-9)

		remaining := node.Children[fieldNameRemaining].Schema
		require.NotNil(t, remaining)
		assert.False(t, remaining.IsEditable)
		assert.NotEmpty(t, remaining.Formula)
	})

	t.Run("Happy path: every extra size enters a count and derives its remaining", func(t *testing.T) {
		for _, size := range []string{"d6", "d8", "d10", "d12"} {
			node := group[size]
			require.NotNil(t, node.Children, size)
			assert.Equal(t, []string{fifthHitDieExtraKey, fifthHitDieSpentKey, fieldNameRemaining}, node.Columns)

			extra := node.Children[fifthHitDieExtraKey].Schema
			require.NotNil(t, extra, size)
			assert.True(t, extra.IsEditable)
			assert.False(t, extra.Generatable)
			assert.InDelta(t, 0.0, extra.Default, 1e-9)

			spent := node.Children[fifthHitDieSpentKey].Schema
			require.NotNil(t, spent, size)
			assert.True(t, spent.IsEditable)

			remaining := node.Children[fieldNameRemaining].Schema
			require.NotNil(t, remaining, size)
			assert.False(t, remaining.IsEditable)
			assert.NotEmpty(t, remaining.Formula)
		}
	})

	t.Run("Happy path: the order lists the primary pool before the extra sizes", func(t *testing.T) {
		assert.Equal(t, []string{"primary", "d6", "d8", "d10", "d12"}, fifthHitDiceOrder())
	})
}

func TestFifthHitDice_PrimaryCount(t *testing.T) {
	tests := []struct {
		name   string
		level  float64
		extras map[string]float64
		want   float64
	}{
		{
			name:  "Happy path: no extras leaves the primary count at the level",
			level: 5,
			want:  5,
		},
		{
			name:  "Happy path: several sizes reduce the primary count",
			level: 10,
			extras: map[string]float64{
				"hitDice.d6.extra":  2,
				"hitDice.d8.extra":  1,
				"hitDice.d10.extra": 1,
			},
			want: 6,
		},
		{
			name:   "Happy path: an extra of the primary size counts like any other",
			level:  5,
			extras: map[string]float64{"hitDice.d8.extra": 2},
			want:   3,
		},
		{
			name:  "Edge: extras equal to the level leave a zero primary count",
			level: 3,
			extras: map[string]float64{
				"hitDice.d6.extra":  2,
				"hitDice.d10.extra": 1,
			},
			want: 0,
		},
		{
			name:   "Edge: extras above the level never make the primary count negative",
			level:  2,
			extras: map[string]float64{"hitDice.d12.extra": 5},
			want:   0,
		},
	}

	character := fifthTemplateByID(t, fifthCharacterID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updates := map[string]any{"identity.level": tc.level}
			for key, value := range tc.extras {
				updates[key] = value
			}
			data := fifthSheet(t, character, updates)
			assert.InDelta(t, tc.want, fifthValue(t, data, "hitDice.primary.count"), 1e-9)
		})
	}
}

func TestFifthHitDice_Remaining(t *testing.T) {
	tests := []struct {
		name    string
		updates map[string]any
		field   string
		want    float64
	}{
		{
			name:    "Happy path: the primary remaining subtracts the spent dice",
			updates: map[string]any{"identity.level": float64(5), "hitDice.primary.spent": float64(2)},
			field:   "hitDice.primary.remaining",
			want:    3,
		},
		{
			name:    "Happy path: an extra pool subtracts its own spent dice",
			updates: map[string]any{"hitDice.d6.extra": float64(4), "hitDice.d6.spent": float64(1)},
			field:   "hitDice.d6.remaining",
			want:    3,
		},
		{
			name:    "Edge: spending past the pool leaves a zero remaining",
			updates: map[string]any{"hitDice.d10.extra": float64(1), "hitDice.d10.spent": float64(3)},
			field:   "hitDice.d10.remaining",
			want:    0,
		},
		{
			name:    "Edge: a zero pool with no spend stays zero",
			updates: map[string]any{"hitDice.d12.spent": float64(0)},
			field:   "hitDice.d12.remaining",
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

func TestFifthHitDice_RowDice(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	require.NoError(t, ValidateDiceDefaults(tmpl))
	require.NoError(t, ValidateNotD20Tests(tmpl))

	t.Run("Happy path: every pool rolls one die plus the Constitution modifier", func(t *testing.T) {
		rows := fifthSectionRows(t, tmpl, fifthHitDiceSection)
		assert.ElementsMatch(t, []string{
			"hitDice.primary", "hitDice.d6", "hitDice.d8", "hitDice.d10", "hitDice.d12",
		}, rows)

		for _, row := range rows {
			expression := fifthEffectiveDice(tmpl, fifthHitDiceSection, row)
			assert.Contains(t, expression, "abilities.constitution.mod", row)
			assert.Contains(t, expression, "min 1", row)
			assert.LessOrEqual(t, len(expression), 64, row)
		}
	})

	t.Run("Happy path: the primary pool sizes its die from the entered size", func(t *testing.T) {
		expression := fifthEffectiveDice(tmpl, fifthHitDiceSection, "hitDice.primary")
		assert.Contains(t, expression, "1d{hitDice.primary.size}")
	})

	t.Run("Happy path: each extra pool rolls its own die size", func(t *testing.T) {
		for _, size := range []string{"d6", "d8", "d10", "d12"} {
			expression := fifthEffectiveDice(tmpl, fifthHitDiceSection, "hitDice."+size)
			assert.Contains(t, expression, "1"+size+" ", size)
		}
	})

	t.Run("Happy path: no Hit Die Row offers a Roll mode", func(t *testing.T) {
		assert.Contains(t, tmpl.NotD20Sections, fifthHitDiceSection)
		for _, row := range fifthSectionRows(t, tmpl, fifthHitDiceSection) {
			assert.NotContains(t, tmpl.NotD20Rows, row)
		}
	})
}

func TestFifthHitDice_SpentDiceFeedTheRemaining(t *testing.T) {
	character := fifthTemplateByID(t, fifthCharacterID)
	data := fifthSheet(t, character, map[string]any{
		"identity.level":        float64(6),
		"hitDice.primary.spent": float64(1),
		"hitDice.d8.extra":      float64(2),
		"hitDice.d8.spent":      float64(2),
	})

	assert.InDelta(t, 4.0, fifthValue(t, data, "hitDice.primary.count"), 1e-9)
	assert.InDelta(t, 3.0, fifthValue(t, data, "hitDice.primary.remaining"), 1e-9)
	assert.InDelta(t, 0.0, fifthValue(t, data, "hitDice.d8.remaining"), 1e-9)
}

func TestFifthHitDice_Labels(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	for _, size := range fifthExtraHitDice {
		node := tmpl.Fields[fifthHitDiceSection].Children[size]
		require.NotNil(t, node)
		assert.Equal(t, fmt.Sprintf("%s Hit Dice", size), node.Label)
	}
}
