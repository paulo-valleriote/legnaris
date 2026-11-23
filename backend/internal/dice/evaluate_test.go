package dice_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/dice"
)

type fixedDice struct {
	faces []int
	rolls []int
	next  int
}

func (f *fixedDice) roll(faces int) int {
	f.faces = append(f.faces, faces)
	if f.next >= len(f.rolls) {
		return 1
	}
	value := f.rolls[f.next]
	f.next++
	return value
}

func TestEvaluate_Vectors(t *testing.T) {
	set := loadVectors(t)
	require.NotEmpty(t, set.Evaluate)

	for _, vec := range set.Evaluate {
		name := "Happy path: "
		if vec.Error != "" {
			name = "Error: "
		}
		t.Run(name+vec.Name, func(t *testing.T) {
			expr, err := dice.Parse(vec.Expr)
			require.NoError(t, err)
			fixed := &fixedDice{rolls: vec.Dice}

			result, err := expr.Evaluate(vec.Row, vec.Values, fixed.roll)

			if vec.Error != "" {
				require.Error(t, err)
				assert.Equal(t, vec.Error, classOf(t, err))
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, vec.Total, result.Total, 1e-9)
			assert.Equal(t, vec.Minimum, result.Minimum)
			assert.Equal(t, len(vec.Dice), fixed.next, "every fixed die must be consumed")
			require.Len(t, result.Parts, len(vec.Parts))
			for i, want := range vec.Parts {
				got := result.Parts[i]
				assert.Equal(t, want.Sign, got.Sign, "part %d sign", i)
				assert.Equal(t, want.Label, got.Label, "part %d label", i)
				assert.InDelta(t, want.Value, got.Value, 1e-9, "part %d value", i)
				assert.Equal(t, want.Rolls, got.Rolls, "part %d rolls", i)
			}
		})
	}
}

func TestEvaluate_AsksTheRollerForTheRightFaces(t *testing.T) {
	expr, err := dice.Parse("2d6 + 1d20 - 3d4")
	require.NoError(t, err)
	fixed := &fixedDice{}

	_, err = expr.Evaluate("r", nil, fixed.roll)

	require.NoError(t, err)
	assert.Equal(t, []int{6, 6, 20, 4, 4, 4}, fixed.faces)
}

func TestEvaluate_Edges(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		values map[string]float64
		roll   dice.Roller
		class  dice.Class
	}{
		{"Error: NaN value", "@", map[string]float64{"r": math.NaN()}, nil, dice.ClassNoValue},
		{"Error: +Inf value", "@", map[string]float64{"r": math.Inf(1)}, nil, dice.ClassNoValue},
		{"Error: -Inf value", "@", map[string]float64{"r": math.Inf(-1)}, nil, dice.ClassNoValue},
		{"Error: nil values map", "@", nil, nil, dice.ClassNoValue},
		{"Error: total overflows to infinity", "@ + @.other", map[string]float64{"r": math.MaxFloat64, "r.other": math.MaxFloat64}, nil, dice.ClassOverflow},
		{"Error: nil roller with dice", "1d20", nil, nil, dice.ClassNoRoller},
		{"Error: count from a Field of zero", "{r.count}d6", map[string]float64{"r.count": 0}, nil, dice.ClassDiceCount},
		{"Error: count from a Field above the limit", "{r.count}d6", map[string]float64{"r.count": 21}, nil, dice.ClassDiceCount},
		{"Error: faces from a Field of zero", "1d{r.faces}", map[string]float64{"r.faces": 0}, nil, dice.ClassDiceFaces},
		{"Error: faces from a Field above the limit", "1d{r.faces}", map[string]float64{"r.faces": 1001}, nil, dice.ClassDiceFaces},
		{"Error: missing Field-sized count", "{r.count}d6", map[string]float64{}, nil, dice.ClassNoValue},
		{"Error: missing Field-sized faces", "1d{r.faces}", map[string]float64{}, nil, dice.ClassNoValue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := dice.Parse(tc.expr)
			require.NoError(t, err)

			_, err = expr.Evaluate("r", tc.values, tc.roll)

			require.Error(t, err)
			assert.Equal(t, string(tc.class), classOf(t, err))
		})
	}
}

func TestEvaluate_MinimumEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		expr       string
		roll       []int
		wantTotal  float64
		wantMin    *int
		wantThrows int
	}{
		{"Edge: below the minimum raises the total", "1d4 min 3", []int{1}, 3, intPointer(3), 1},
		{"Edge: at the minimum leaves it unapplied", "1d4 min 3", []int{3}, 3, nil, 1},
		{"Edge: above the minimum leaves it unapplied", "1d4 min 3", []int{4}, 4, nil, 1},
		{"Edge: minimum zero raises a negative total", "1d4 - 6 min 0", []int{1}, 0, intPointer(0), 1},
		{"Edge: a minimum without dice never throws", "5 min 9", nil, 9, intPointer(9), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := dice.Parse(tc.expr)
			require.NoError(t, err)
			fixed := &fixedDice{rolls: tc.roll}

			result, err := expr.Evaluate("r", nil, fixed.roll)

			require.NoError(t, err)
			assert.InDelta(t, tc.wantTotal, result.Total, 1e-9)
			assert.Equal(t, tc.wantMin, result.Minimum)
			assert.Equal(t, tc.wantThrows, fixed.next)
		})
	}
}

func intPointer(value int) *int {
	return &value
}

func TestEvaluate_NilRollerIsFineWithoutDice(t *testing.T) {
	expr, err := dice.Parse("@ + 2")
	require.NoError(t, err)

	result, err := expr.Evaluate("r", map[string]float64{"r": 3}, nil)

	require.NoError(t, err)
	assert.InDelta(t, float64(5), result.Total, 1e-9)
}

func TestEvaluate_ZeroExpressionTotalsZero(t *testing.T) {
	var zero dice.Expression

	result, err := zero.Evaluate("r", nil, nil)

	require.NoError(t, err)
	assert.Zero(t, result.Total)
	assert.Empty(t, result.Parts)
}

func TestEvaluate_DoesNotMutateItsInputs(t *testing.T) {
	expr, err := dice.Parse("1d6 + @")
	require.NoError(t, err)
	values := map[string]float64{"r": 2.5}
	fixed := &fixedDice{rolls: []int{4}}

	first, err := expr.Evaluate("r", values, fixed.roll)
	require.NoError(t, err)
	first.Parts[0].Rolls[0] = 99

	assert.Equal(t, map[string]float64{"r": 2.5}, values)
	assert.Equal(t, "1d6 + @", expr.String())
}
