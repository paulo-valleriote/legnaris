package dice_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/dice"
)

func TestParse_Vectors(t *testing.T) {
	set := loadVectors(t)
	require.NotEmpty(t, set.Valid)
	require.NotEmpty(t, set.Invalid)

	for _, vec := range set.Valid {
		t.Run("Happy path: "+vec.Name, func(t *testing.T) {
			expr, err := dice.Parse(vec.Expr)
			require.NoError(t, err)
			assert.Equal(t, vec.Canonical, expr.String())
		})
	}
	for _, vec := range set.Invalid {
		t.Run("Error: "+vec.Name, func(t *testing.T) {
			_, err := dice.Parse(vec.Expr)
			require.Error(t, err)
			assert.Equal(t, vec.Class, classOf(t, err))
		})
	}
}

func TestParse_LimitsMatchVectors(t *testing.T) {
	limits := loadVectors(t).Limits
	assert.Equal(t, dice.MaxLength, limits.MaxLength)
	assert.Equal(t, dice.MaxTerms, limits.MaxTerms)
	assert.Equal(t, dice.MinDice, limits.MinDice)
	assert.Equal(t, dice.MaxDice, limits.MaxDice)
	assert.Equal(t, dice.MinFaces, limits.MinFaces)
	assert.Equal(t, dice.MaxFaces, limits.MaxFaces)
	assert.Equal(t, dice.MaxWholeNumber, limits.MaxWholeNumber)
}

func TestParse_Boundaries(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		class dice.Class
	}{
		{"Edge: 64 characters accepted", strings.Repeat("x", 64), ""},
		{"Error: 65 characters rejected", strings.Repeat("x", 65), dice.ClassTooLong},
		{"Edge: 8 terms accepted", strings.Repeat("1+", 7) + "1", ""},
		{"Error: 9 terms rejected", strings.Repeat("1+", 8) + "1", dice.ClassTooManyTerms},
		{"Edge: N=1 accepted", "1d6", ""},
		{"Edge: N=20 accepted", "20d6", ""},
		{"Error: N=0 rejected", "0d6", dice.ClassDiceCount},
		{"Error: N=21 rejected", "21d6", dice.ClassDiceCount},
		{"Edge: M=1 accepted", "1d1", ""},
		{"Edge: M=1000 accepted", "1d1000", ""},
		{"Error: M=0 rejected", "1d0", dice.ClassDiceFaces},
		{"Error: M=1001 rejected", "1d1001", dice.ClassDiceFaces},
		{"Edge: 0 accepted", "0", ""},
		{"Edge: 9999 accepted", "9999", ""},
		{"Error: 10000 rejected", "10000", dice.ClassNumberRange},
		{"Error: empty rejected", "", dice.ClassEmpty},
		{"Error: whitespace-only rejected", " \t ", dice.ClassEmpty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dice.Parse(tc.expr)
			if tc.class == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, string(tc.class), classOf(t, err))
		})
	}
}

func TestParse_ErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		sentinel error
	}{
		{"Error: empty", "", dice.ErrEmpty},
		{"Error: too long", strings.Repeat("x", 65), dice.ErrTooLong},
		{"Error: too many terms", strings.Repeat("1+", 8) + "1", dice.ErrTooManyTerms},
		{"Error: dice count", "21d6", dice.ErrDiceCount},
		{"Error: dice faces", "1d1001", dice.ErrDiceFaces},
		{"Error: number range", "10000", dice.ErrNumberRange},
		{"Error: missing term", "1d20 +", dice.ErrMissingTerm},
		{"Error: unreadable term", "(1d20)", dice.ErrUnreadableTerm},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dice.Parse(tc.expr)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.sentinel)

			var typed *dice.Error
			require.True(t, errors.As(err, &typed))
			assert.NotEmpty(t, typed.Message)
			assert.NotEmpty(t, err.Error())
		})
	}

	t.Run("Edge: a different class does not match", func(t *testing.T) {
		_, err := dice.Parse("")
		assert.NotErrorIs(t, err, dice.ErrTooLong)
	})

	t.Run("Edge: foreign errors carry no class", func(t *testing.T) {
		_, ok := dice.ClassOf(errors.New("boom"))
		assert.False(t, ok)
		_, ok = dice.ClassOf(nil)
		assert.False(t, ok)
	})
}

func TestParse_ImplicitAndExplicitCountAreEqual(t *testing.T) {
	implicit, err := dice.Parse("d20")
	require.NoError(t, err)
	explicit, err := dice.Parse("1d20")
	require.NoError(t, err)
	assert.Equal(t, explicit.Terms(), implicit.Terms())
}

func TestParse_TermsDescribeTheExpression(t *testing.T) {
	expr, err := dice.Parse("-2D6 + 5 - stats.dexterity + @ - @.total")
	require.NoError(t, err)

	assert.Equal(t, []dice.Term{
		{Kind: dice.KindDice, Sign: -1, Count: 2, Faces: 6},
		{Kind: dice.KindNumber, Sign: 1, Value: 5},
		{Kind: dice.KindField, Sign: -1, Path: "stats.dexterity"},
		{Kind: dice.KindField, Sign: 1, Relative: true},
		{Kind: dice.KindField, Sign: -1, Relative: true, Column: "total"},
	}, expr.Terms())
}

func TestExpression_TermsReturnsACopy(t *testing.T) {
	expr, err := dice.Parse("1d20 + 5")
	require.NoError(t, err)

	terms := expr.Terms()
	terms[0].Faces = 999

	assert.Equal(t, "1d20 + 5", expr.String())
	assert.Equal(t, 20, expr.Terms()[0].Faces)
}

func TestParse_IsDeterministicAndConcurrencySafe(t *testing.T) {
	const workers = 16
	const expr = "1d20 + @.total - stats.dexterity + 3"

	want, err := dice.Parse(expr)
	require.NoError(t, err)

	var wg sync.WaitGroup
	results := make([]string, workers)
	for i := range workers {
		wg.Go(func() {
			parsed, perr := dice.Parse(expr)
			if perr == nil {
				results[i] = parsed.String()
			}
			_ = parsed.Terms()
		})
	}
	wg.Wait()

	for _, got := range results {
		assert.Equal(t, want.String(), got)
	}
}

func TestParse_ZeroExpressionIsEmpty(t *testing.T) {
	var zero dice.Expression
	assert.Empty(t, zero.Terms())
	assert.Empty(t, zero.String())
}
