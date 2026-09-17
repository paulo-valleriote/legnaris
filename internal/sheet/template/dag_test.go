package template

import (
	"errors"
	"legnaris/internal/engine"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDag(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	tests := []struct {
		name      string
		rawNodes  map[string]FieldSchema
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "Valid DAG without cycles",
			rawNodes: map[string]FieldSchema{
				"stats.str": {Type: "number", IsEditable: true},
				"mod.str": {
					Type: "number", IsEditable: false,
					Formula:   "floor((stats.str - 10) / 2)",
					DependsOn: []string{"stats.str"},
				},
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Infinite Loop (cycle detection between two fields)",
			rawNodes: map[string]FieldSchema{
				"health": {
					Formula:   "level * 10",
					DependsOn: []string{"level"},
				},
				"level": {
					Formula:   "health / 10",
					DependsOn: []string{"health"},
				},
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInfiniteCycle)
			},
		},
		{
			name: "Invalid Formula Syntax",
			rawNodes: map[string]FieldSchema{
				"stats.str": {Type: "number"},
				"mod.str": {
					Formula:   "floor((stats.str - 10) / ",
					DependsOn: []string{"stats.str"},
				},
			},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)

				dummyErr := errors.New("")
				expectedMsg := ErrCompileFormula("floor((stats.str - 10) / ", "mod.str", dummyErr).Error()
				expectedMsg = strings.TrimSuffix(expectedMsg, dummyErr.Error()) // Remove a parte vazia do final
				assert.ErrorContains(t, err, expectedMsg)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("test_template", tc.rawNodes, formulaEngine)
			tc.assertErr(t, err)

			if err == nil {
				require.NotNil(t, dag)
				assert.NotEmpty(t, dag.TopologicalOrder)
			}
		})
	}
}

func TestRecalculate(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	rawNodes := map[string]FieldSchema{
		"stats.str": {Type: "number", IsEditable: true},
		"mod.str": {
			Type: "number", IsEditable: false,
			Formula:   "floor((stats_str - 10) / 2)",
			DependsOn: []string{"stats.str"},
		},
		"derived.dmg": {
			Type: "number", IsEditable: false,
			Formula:   "mod_str + 5",
			DependsOn: []string{"mod.str"},
		},
	}

	dag, err := BuildDAG("fantasy", rawNodes, formulaEngine)
	require.NoError(t, err, "Failed to bootstrap test DAG")

	tests := []struct {
		name             string
		originalData     map[string]any
		updates          map[string]any
		expectedCascades map[string]any
	}{
		{
			name: "Happy Path: Full Cascade Calculation",
			originalData: map[string]any{
				"stats.str":   10.0,
				"mod.str":     0.0,
				"derived.dmg": 5.0,
			},
			updates: map[string]any{
				"stats.str": 16.0, // Str changes from 10 to 16
			},
			expectedCascades: map[string]any{
				"mod.str":     3.0, // floor((16-10)/2)
				"derived.dmg": 8.0, // 3 + 5
			},
		},
		{
			name: "Edge Case: Short-Circuit (Propagation stops if value doesn't change)",
			originalData: map[string]any{
				"stats.str":   16.0,
				"mod.str":     3.0,
				"derived.dmg": 8.0,
			},
			updates: map[string]any{
				// Str changes from 16 to 17, but mod formula floor((17-10)/2) is STILL 3.
				// Because mod.str remains 3, derived.dmg should NOT be recalculated or returned.
				"stats.str": 17.0,
			},
			expectedCascades: map[string]any{
				// Empty because nothing downstream actually mutated!
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cascadingEffects, err := dag.Recalculate(tc.originalData, tc.updates)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedCascades, cascadingEffects)
		})
	}
}
