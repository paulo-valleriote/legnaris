package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile(t *testing.T) {
	eng := NewFormulaEngine()

	tests := []struct {
		name      string
		formula   string
		mockEnv   map[string]any
		assertErr func(t *testing.T, err error)
	}{
		{
			name:    "Happy path: Valid Formula",
			formula: "floor((stats_str - 10) / 2)",
			mockEnv: map[string]any{"stats_str": 0.0},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "Edge: Empty string returns nil program",
			formula: "",
			mockEnv: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "Error: Syntax Error triggers ErrCompile",
			formula: "10 + * 2",
			mockEnv: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrCompile)
			},
		},
		{
			name:    "Error: Unknown variable triggers ErrCompile",
			formula: "unknown_var + 10",
			mockEnv: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrCompile)
				assert.ErrorContains(t, err, "unknown name unknown_var")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := eng.Compile(tc.formula, tc.mockEnv)
			tc.assertErr(t, err)

			if err == nil && tc.formula != "" {
				assert.NotNil(t, prog)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	eng := NewFormulaEngine()

	tests := []struct {
		name      string
		formula   string
		env       map[string]any
		expected  any
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "Happy path: Custom 'floor' with float64",
			formula:   "floor(10.5)",
			env:       map[string]any{},
			expected:  10.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      "Happy path: Custom 'floor' with int",
			formula:   "floor(10)",
			env:       map[string]any{},
			expected:  10.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      "Happy path: Custom 'max' function",
			formula:   "max(stats_str, 15.0)",
			env:       map[string]any{"stats_str": 12.0},
			expected:  15.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:     "Error: floor with invalid type (string)",
			formula:  "floor(text)",
			env:      map[string]any{"text": "not_a_number"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorContains(t, err, ErrInvalidFloat.Error())
			},
		},
		{
			name:     "Error: Max without enough arguments",
			formula:  "max(10.0)",
			env:      map[string]any{},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrInvalidFloat.Error())
				assert.ErrorContains(t, err, "expects exactly 2 arguments")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := eng.Compile(tc.formula, tc.env)
			require.NoError(t, err, "Compile failed in Evaluate test setup")

			result, err := eng.Evaluate(prog, tc.env)
			tc.assertErr(t, err)
			if err == nil {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestMin(t *testing.T) {
	eng := NewFormulaEngine()

	tests := []struct {
		name      string
		formula   string
		env       map[string]any
		expected  any
		assertErr func(t *testing.T, err error)
	}{
		{
			name:     "Edge: equal arguments return the shared value",
			formula:  "min(a, b)",
			env:      map[string]any{"a": 7.0, "b": 7.0},
			expected: 7.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: both arguments are negative",
			formula:  "min(a, b)",
			env:      map[string]any{"a": -3.5, "b": -8.0},
			expected: -8.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: int and float are mixed",
			formula:  "min(a, b)",
			env:      map[string]any{"a": 4, "b": 2.5},
			expected: 2.5,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: the smaller argument is returned",
			formula:  "min(a, 15.0)",
			env:      map[string]any{"a": 12.0},
			expected: 12.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Error: min without enough arguments",
			formula:  "min(10.0)",
			env:      map[string]any{},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidFloat)
				assert.ErrorContains(t, err, "min expects exactly 2 arguments")
			},
		},
		{
			name:     "Error: min rejects a non-numeric argument",
			formula:  "min(v, 15)",
			env:      map[string]any{"v": "12"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := eng.Compile(tc.formula, tc.env)
			require.NoError(t, err, "Compile failed in min test setup")

			result, err := eng.Evaluate(prog, tc.env)
			tc.assertErr(t, err)
			if err == nil {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	eng := NewFormulaEngine()

	tests := []struct {
		name      string
		formula   string
		env       map[string]any
		expected  any
		assertErr func(t *testing.T, err error)
	}{
		{
			name:     "Happy path: a value exactly on a key returns that key's value",
			formula:  "lookup(x, 0, 10, 5, 20, 10, 30)",
			env:      map[string]any{"x": 5.0},
			expected: 20.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: a value between keys returns the lower key's value",
			formula:  "lookup(x, 0, 10, 5, 20, 10, 30)",
			env:      map[string]any{"x": 7.0},
			expected: 20.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: below the first key returns the first value",
			formula:  "lookup(x, 0, 10, 5, 20, 10, 30)",
			env:      map[string]any{"x": -1.0},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: above the last key returns the last value",
			formula:  "lookup(x, 0, 10, 5, 20, 10, 30)",
			env:      map[string]any{"x": 100.0},
			expected: 30.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: fractional keys are ordered and selected",
			formula:  "lookup(x, 0, 1, 0.125, 2, 0.25, 3)",
			env:      map[string]any{"x": 0.2},
			expected: 2.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: a single pair always returns its value",
			formula:  "lookup(x, 4, 99)",
			env:      map[string]any{"x": 1.0},
			expected: 99.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Edge: int and float arguments are mixed",
			formula:  "lookup(x, 0, 1, 10, 2.5)",
			env:      map[string]any{"x": 10},
			expected: 2.5,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Error: an even argument count has no complete pair",
			formula:  "lookup(x, 0, 10, 5)",
			env:      map[string]any{"x": 3.0},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidLookupArity)
			},
		},
		{
			name:     "Error: fewer than three arguments",
			formula:  "lookup(x)",
			env:      map[string]any{"x": 3.0},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidLookupArity)
			},
		},
		{
			name:     "Error: unordered keys are refused",
			formula:  "lookup(x, 0, 10, 10, 20, 5, 30)",
			env:      map[string]any{"x": 7.0},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidLookupKeyOrder)
			},
		},
		{
			name:     "Error: duplicate keys are not strictly ascending",
			formula:  "lookup(x, 0, 10, 5, 20, 5, 30)",
			env:      map[string]any{"x": 7.0},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidLookupKeyOrder)
			},
		},
		{
			name:     "Error: a non-numeric lookup value is refused",
			formula:  "lookup(x, 0, 10, 5, v)",
			env:      map[string]any{"x": 3.0, "v": "twenty"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
		{
			name:     "Error: a non-numeric key is refused",
			formula:  "lookup(x, 0, 10, k, 20)",
			env:      map[string]any{"x": 3.0, "k": "five"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
		{
			name:     "Error: a non-numeric input is refused",
			formula:  "lookup(x, 0, 10, 5, 20)",
			env:      map[string]any{"x": "three"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := eng.Compile(tc.formula, tc.env)
			require.NoError(t, err, "Compile failed in lookup test setup")

			result, err := eng.Evaluate(prog, tc.env)
			tc.assertErr(t, err)
			if err == nil {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestEvaluateNilProgram(t *testing.T) {
	eng := NewFormulaEngine()

	_, err := eng.Evaluate(nil, map[string]any{})
	require.ErrorIs(t, err, ErrNilProgram)
}

func TestEvaluateNumericCoercion(t *testing.T) {
	eng := NewFormulaEngine()

	tests := []struct {
		name      string
		formula   string
		env       map[string]any
		expected  any
		assertErr func(t *testing.T, err error)
	}{
		{
			name:     "Happy path: floor accepts int32",
			formula:  "floor(v)",
			env:      map[string]any{"v": int32(10)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: floor accepts int64",
			formula:  "floor(v)",
			env:      map[string]any{"v": int64(10)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: floor accepts float32",
			formula:  "floor(v)",
			env:      map[string]any{"v": float32(10.5)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: max accepts int literals",
			formula:  "max(v, 15)",
			env:      map[string]any{"v": 12},
			expected: 15.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Happy path: max accepts int32 and int64",
			formula:  "max(a, b)",
			env:      map[string]any{"a": int32(12), "b": int64(15)},
			expected: 15.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Error: floor rejects bool with typed error",
			formula:  "floor(v)",
			env:      map[string]any{"v": true},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
		{
			name:     "Error: max rejects string with typed error",
			formula:  "max(v, 15)",
			env:      map[string]any{"v": "12"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := eng.Compile(tc.formula, tc.env)
			require.NoError(t, err, "Compile failed in coercion test setup")

			result, err := eng.Evaluate(prog, tc.env)
			tc.assertErr(t, err)
			if err == nil {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
