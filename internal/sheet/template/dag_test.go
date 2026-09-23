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
		fields    FieldGroup
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "Valid DAG without cycles",
			fields: FieldGroup{
				"stats": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				}},
				"mod": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "floor((stats_str - 10) / 2)",
						DependsOn: [][]string{{"stats", "str"}},
					}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Infinite Loop (cycle detection between two fields)",
			fields: FieldGroup{
				"health": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "level * 10",
					DependsOn: [][]string{{"level"}},
				}},
				"level": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "health / 10",
					DependsOn: [][]string{{"health"}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInfiniteCycle)
			},
		},
		{
			name: "Invalid Formula Syntax",
			fields: FieldGroup{
				"stats": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{Type: "number"}},
				}},
				"mod": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "floor((stats_str - 10) / ",
						DependsOn: [][]string{{"stats", "str"}},
					}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)

				dummyErr := errors.New("")
				expectedMsg := ErrCompileFormula("floor((stats_str - 10) / ", "mod.str", dummyErr).Error()
				expectedMsg = strings.TrimSuffix(expectedMsg, dummyErr.Error())
				assert.ErrorContains(t, err, expectedMsg)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("test_template", tc.fields, formulaEngine)
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

	fields := FieldGroup{
		"stats": {Children: FieldGroup{
			"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		}},
		"mod": {Children: FieldGroup{
			"str": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_str - 10) / 2)",
				DependsOn:  [][]string{{"stats", "str"}},
			}},
		}},
		"derived": {Children: FieldGroup{
			"dmg": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "mod_str + 5",
				DependsOn:  [][]string{{"mod", "str"}},
			}},
		}},
	}

	dag, err := BuildDAG("fantasy", fields, formulaEngine)
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
				"stats": map[string]any{"str": 10.0},
				"mod":   map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats.str": 16.0,
			},
			expectedCascades: map[string]any{
				"mod.str":     3.0,
				"derived.dmg": 8.0,
			},
		},
		{
			name: "Edge Case: Short-Circuit (Propagation stops if value doesn't change)",
			originalData: map[string]any{
				"stats": map[string]any{"str": 16.0},
				"mod":   map[string]any{"str": 3.0},
				"derived": map[string]any{"dmg": 8.0},
			},
			updates: map[string]any{
				"stats.str": 17.0,
			},
			expectedCascades: map[string]any{},
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
