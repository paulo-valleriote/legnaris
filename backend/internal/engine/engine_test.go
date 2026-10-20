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
			name:    "Valid Formula",
			formula: "floor((stats_str - 10) / 2)",
			mockEnv: map[string]any{"stats_str": 0.0},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "Empty string returns nil program",
			formula: "",
			mockEnv: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "Syntax Error triggers ErrCompile",
			formula: "10 + * 2",
			mockEnv: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrCompile)
			},
		},
		{
			name:    "Unknown variable triggers ErrCompile",
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
			name:      "Custom 'floor' with float64",
			formula:   "floor(10.5)",
			env:       map[string]any{},
			expected:  10.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      " Custom 'floor' with int",
			formula:   "floor(10)",
			env:       map[string]any{},
			expected:  10.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      "Custom 'max' function",
			formula:   "max(stats_str, 15.0)",
			env:       map[string]any{"stats_str": 12.0},
			expected:  15.0,
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:     "floor with invalid type (string)",
			formula:  "floor(text)",
			env:      map[string]any{"text": "not_a_number"},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorContains(t, err, ErrInvalidFloat.Error())
			},
		},
		{
			name:     "Max without enough arguments",
			formula:  "max(10.0)",
			env:      map[string]any{},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorContains(t, err, ErrInvalidFloat.Error())
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
			name:     "floor accepts int32",
			formula:  "floor(v)",
			env:      map[string]any{"v": int32(10)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "floor accepts int64",
			formula:  "floor(v)",
			env:      map[string]any{"v": int64(10)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "floor accepts float32",
			formula:  "floor(v)",
			env:      map[string]any{"v": float32(10.5)},
			expected: 10.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "max accepts int literals",
			formula:  "max(v, 15)",
			env:      map[string]any{"v": 12},
			expected: 15.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "max accepts int32 and int64",
			formula:  "max(a, b)",
			env:      map[string]any{"a": int32(12), "b": int64(15)},
			expected: 15.0,
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "floor rejects bool with typed error",
			formula:  "floor(v)",
			env:      map[string]any{"v": true},
			expected: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidFloat)
			},
		},
		{
			name:     "max rejects string with typed error",
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
