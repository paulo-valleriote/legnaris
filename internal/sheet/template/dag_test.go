package template

import (
	"errors"
	"legnaris/internal/engine"
	"legnaris/shared/maputils"
	"math"
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
				assert.NotEmpty(t, dag.TopologicalOrder())
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
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
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
				"stats":   map[string]any{"str": 16.0},
				"mod":     map[string]any{"str": 3.0},
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

	// Additional scenarios: fan-out, fan-in, multi-update, deep nesting
	additionalTests := []struct {
		name             string
		fields           FieldGroup
		originalData     map[string]any
		updates          map[string]any
		expectedCascades map[string]any
	}{
		{
			name: "Fan-Out: One source triggers multiple derived",
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
				"derived": {Children: FieldGroup{
					"atk": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "mod_str + 5",
						DependsOn: [][]string{{"mod", "str"}},
					}},
					"carry": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "stats_str * 15",
						DependsOn: [][]string{{"stats", "str"}},
					}},
				}},
			},
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"atk": 5.0, "carry": 150.0},
			},
			updates: map[string]any{
				"stats.str": 16.0,
			},
			expectedCascades: map[string]any{
				"mod.str":       3.0,
				"derived.atk":   8.0,
				"derived.carry": 240.0,
			},
		},
		{
			name: "Fan-In: Multiple sources converge to one derived",
			fields: FieldGroup{
				"base": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				}},
				"bonus": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				}},
				"derived": {Children: FieldGroup{
					"total": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "base_str + bonus_str",
						DependsOn: [][]string{{"base", "str"}, {"bonus", "str"}},
					}},
				}},
			},
			originalData: map[string]any{
				"base":    map[string]any{"str": 10.0},
				"bonus":   map[string]any{"str": 2.0},
				"derived": map[string]any{"total": 12.0},
			},
			updates: map[string]any{
				"base.str":  14.0,
				"bonus.str": 5.0,
			},
			expectedCascades: map[string]any{
				"derived.total": 19.0,
			},
		},
		{
			name: "Fan-Out-Fan-In: Diamond dependency (A→B,C; B,C→D)",
			fields: FieldGroup{
				"attr": {Children: FieldGroup{
					"base": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				}},
				"mod1": {Children: FieldGroup{
					"val": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "attr_base * 2",
						DependsOn: [][]string{{"attr", "base"}},
					}},
				}},
				"mod2": {Children: FieldGroup{
					"val": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "attr_base + 5",
						DependsOn: [][]string{{"attr", "base"}},
					}},
				}},
				"derived": {Children: FieldGroup{
					"result": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "mod1_val + mod2_val",
						DependsOn: [][]string{{"mod1", "val"}, {"mod2", "val"}},
					}},
				}},
			},
			originalData: map[string]any{
				"attr":    map[string]any{"base": 10.0},
				"mod1":    map[string]any{"val": 20.0},
				"mod2":    map[string]any{"val": 15.0},
				"derived": map[string]any{"result": 35.0},
			},
			updates: map[string]any{
				"attr.base": 15.0,
			},
			expectedCascades: map[string]any{
				"mod1.val":       30.0,
				"mod2.val":       20.0,
				"derived.result": 50.0,
			},
		},
		{
			name: "Multi-Update: Two root fields updated simultaneously",
			fields: FieldGroup{
				"stats": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
					"dex": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				}},
				"mod": {Children: FieldGroup{
					"str": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "floor((stats_str - 10) / 2)",
						DependsOn: [][]string{{"stats", "str"}},
					}},
					"dex": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "floor((stats_dex - 10) / 2)",
						DependsOn: [][]string{{"stats", "dex"}},
					}},
				}},
			},
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0, "dex": 10.0},
				"mod":   map[string]any{"str": 0.0, "dex": 0.0},
			},
			updates: map[string]any{
				"stats.str": 16.0,
				"stats.dex": 14.0,
			},
			expectedCascades: map[string]any{
				"mod.str": 3.0,
				"mod.dex": 2.0,
			},
		},
		{
			name: "Deep Nesting: 3+ levels (character.attributes.physical.strength)",
			fields: FieldGroup{
				"character": {Children: FieldGroup{
					"attributes": {Children: FieldGroup{
						"physical": {Children: FieldGroup{
							"strength": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
						}},
					}},
				}},
				"derived": {Children: FieldGroup{
					"dmg": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "character_attributes_physical_strength * 2",
						DependsOn: [][]string{{"character", "attributes", "physical", "strength"}},
					}},
				}},
			},
			originalData: map[string]any{
				"character": map[string]any{
					"attributes": map[string]any{
						"physical": map[string]any{"strength": 10.0},
					},
				},
				"derived": map[string]any{"dmg": 20.0},
			},
			updates: map[string]any{
				"character.attributes.physical.strength": 15.0,
			},
			expectedCascades: map[string]any{
				"derived.dmg": 30.0,
			},
		},
	}

	for _, tc := range additionalTests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("test", tc.fields, formulaEngine)
			require.NoError(t, err)

			cascadingEffects, err := dag.Recalculate(tc.originalData, tc.updates)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCascades, cascadingEffects)
		})
	}
}

func TestBuildDag_EdgeCases(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	tests := []struct {
		name      string
		fields    FieldGroup
		assertErr func(t *testing.T, err error)
		assertDag func(t *testing.T, dag *DAG)
	}{
		{
			name: "Self-dependency (DependsOn points to itself)",
			fields: FieldGroup{
				"health": {Schema: &FieldSchema{
					Type:       "number",
					IsEditable: false,
					Formula:    "health * 2",
					DependsOn:  [][]string{{"health"}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInfiniteCycle)
			},
		},
		{
			name: "Cycle of 3+ nodes (A→B→C→A)",
			fields: FieldGroup{
				"a": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula: "c + 1", DependsOn: [][]string{{"c"}},
				}},
				"b": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula: "a + 1", DependsOn: [][]string{{"a"}},
				}},
				"c": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula: "b + 1", DependsOn: [][]string{{"b"}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrInfiniteCycle)
			},
		},
		{
			name: "DependsOn referencing non-existent path",
			fields: FieldGroup{
				"derived": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "nonexistent + 1",
					DependsOn: [][]string{{"nonexistent"}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				// TODO(LGN-41): Current behavior returns ErrCompileFormula because the engine
				// compiles the formula against mockEnv which doesn't have "nonexistent".
				// The DependsOn is not validated before compilation.
				// Expected: Should validate DependsOn paths exist before compiling, return specific error.
				// Tracking: https://paulovalleriotedev.atlassian.net/browse/LGN-41
				require.Error(t, err)
				assert.ErrorContains(t, err, "unknown name nonexistent")
			},
		},
		{
			name: "Duplicate DependsOn entries",
			fields: FieldGroup{
				"base": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
				"derived": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "base * 2",
					DependsOn: [][]string{{"base"}, {"base"}}, // Duplicate
				}},
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertDag: func(t *testing.T, dag *DAG) {
				// TODO(LGN-41): Current behavior adds duplicate entries to AdjacencyList.
				// inDegree is inflated (2 instead of 1), but Kahn's algorithm still works.
				// Expected: Should deduplicate DependsOn entries before building graph.
				// Tracking: https://paulovalleriotedev.atlassian.net/browse/LGN-41
				node, ok := dag.GetNode("derived")
				require.True(t, ok)
				require.NotNil(t, node)
				assert.Len(t, dag.GetChildren("base"), 2) // Current behavior: 2 entries
			},
		},
		{
			name:   "Template with 0 fields (empty FieldGroup)",
			fields: FieldGroup{},
			assertErr: func(t *testing.T, err error) {
				// TODO(LGN-41): Current behavior returns DAG with empty TopologicalOrder.
				// No error is returned, but downstream code may not handle empty DAG correctly.
				// Expected: Should return error for empty template.
				// Tracking: https://paulovalleriotedev.atlassian.net/browse/LGN-41
				require.NoError(t, err) // Current behavior
			},
			assertDag: func(t *testing.T, dag *DAG) {
				assert.Empty(t, dag.TopologicalOrder())
			},
		},
		{
			name: "Paths with special characters (dots and underscores)",
			fields: FieldGroup{
				"stats": {Children: FieldGroup{
					"str.dex": {Schema: &FieldSchema{Type: "number", IsEditable: true}}, // dot in name
					"str_dex": {Schema: &FieldSchema{Type: "number", IsEditable: true}}, // underscore in name
				}},
				"mod": {Children: FieldGroup{
					"str_dex": {Schema: &FieldSchema{
						Type: "number", IsEditable: false,
						Formula:   "stats_str_dex + 1", // References stats.str_dex (underscore)
						DependsOn: [][]string{{"stats", "str_dex"}},
					}},
				}},
			},
			assertErr: func(t *testing.T, err error) {
				// TODO(LGN-41): MapKey uses ".", toFlatForEngine uses "_" - COLLISION RISK.
				// "stats.str.dex" -> MapKey="stats.str.dex", engine key="stats_str_dex"
				// "stats.str_dex" -> MapKey="stats.str_dex", engine key="stats_str_dex"
				// Both map to same engine key "stats_str_dex" - DATA CORRUPTION!
				// Current behavior: BuildDAG succeeds but Recalculate will have collisions.
				// Tracking: https://paulovalleriotedev.atlassian.net/browse/LGN-41
				require.NoError(t, err) // Current behavior
			},
			assertDag: func(t *testing.T, dag *DAG) {
				// Verify both paths exist and are distinct in MapKey
				assert.True(t, dag.HasNode("stats.str.dex"))
				assert.True(t, dag.HasNode("stats.str_dex"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("test_template", tc.fields, formulaEngine)
			tc.assertErr(t, err)

			if err == nil && tc.assertDag != nil {
				tc.assertDag(t, dag)
			}
		})
	}
}

func TestRecalculate_EdgeCases(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	// Base DAG for most tests
	baseFields := FieldGroup{
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
	}

	baseDag, err := BuildDAG("test", baseFields, formulaEngine)
	require.NoError(t, err)

	// Helper to create DAG with string formula
	stringFormulaDag := func() *DAG {
		fields := FieldGroup{
			"input": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
			"output": {Schema: &FieldSchema{
				Type: "string", IsEditable: false,
				Formula:   "'hello_' + string(input)",
				DependsOn: [][]string{{"input"}},
			}},
		}
		dag, _ := BuildDAG("test", fields, formulaEngine)
		return dag
	}

	// Helper to create DAG with bool formula
	boolFormulaDag := func() *DAG {
		fields := FieldGroup{
			"input": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
			"output": {Schema: &FieldSchema{
				Type: "boolean", IsEditable: false,
				Formula:   "input > 15",
				DependsOn: [][]string{{"input"}},
			}},
		}
		dag, _ := BuildDAG("test", fields, formulaEngine)
		return dag
	}

	tests := []struct {
		name         string
		dag          *DAG
		originalData map[string]any
		updates      map[string]any
		assertErr    func(t *testing.T, err error)
		assertResult func(t *testing.T, result map[string]any)
	}{
		{
			name:         "nil originalData",
			dag:          baseDag,
			originalData: nil,
			updates:      map[string]any{"stats.str": 16.0},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "original data is undefined")
			},
		},
		{
			name:         "nil updatedData",
			dag:          baseDag,
			originalData: map[string]any{"stats": map[string]any{"str": 10.0}},
			updates:      nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "updated data is undefined")
			},
		},
		{
			name: "Formula returns string type",
			dag:  stringFormulaDag(),
			originalData: map[string]any{
				"input":  10.0,
				"output": "hello_10",
			},
			updates: map[string]any{
				"input": 20.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertResult: func(t *testing.T, result map[string]any) {
				assert.Equal(t, "hello_20", result["output"])
			},
		},
		{
			name: "Formula returns boolean type",
			dag:  boolFormulaDag(),
			originalData: map[string]any{
				"input":  10.0,
				"output": false,
			},
			updates: map[string]any{
				"input": 20.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertResult: func(t *testing.T, result map[string]any) {
				assert.Equal(t, true, result["output"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cascadingEffects, err := tc.dag.Recalculate(tc.originalData, tc.updates)
			tc.assertErr(t, err)

			if err == nil && tc.assertResult != nil {
				tc.assertResult(t, cascadingEffects)
			}
		})
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		{
			name:     "float64 equal",
			a:        float64(10.0),
			b:        float64(10.0),
			expected: true,
		},
		{
			name:     "float64 not equal",
			a:        float64(10.0),
			b:        float64(20.0),
			expected: false,
		},
		{
			name:     "string equal",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "string not equal",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "bool equal true",
			a:        true,
			b:        true,
			expected: true,
		},
		{
			name:     "bool equal false",
			a:        false,
			b:        false,
			expected: true,
		},
		{
			name:     "bool not equal",
			a:        true,
			b:        false,
			expected: false,
		},
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        nil,
			b:        float64(10),
			expected: false,
		},
		{
			name:     "slice equal (DeepEqual fallback)",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "slice not equal (DeepEqual fallback)",
			a:        []int{1, 2},
			b:        []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "map equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1, "b": 2},
			b:        map[string]any{"a": 1, "b": 2},
			expected: true,
		},
		{
			name:     "map not equal (DeepEqual fallback)",
			a:        map[string]any{"a": 1},
			b:        map[string]any{"a": 2},
			expected: false,
		},
		{
			name:     "int vs float64 (different concrete types)",
			a:        int(10),
			b:        float64(10.0),
			expected: false, // int not handled in fast path, DeepEqual returns false
		},
		{
			name:     "int64 vs float64 (different concrete types)",
			a:        int64(10),
			b:        float64(10.0),
			expected: false, // int64 not handled in fast path, DeepEqual returns false
		},
		{
			name:     "float64 with fractional part",
			a:        float64(10.5),
			b:        float64(10.5),
			expected: true,
		},
		{
			name:     "empty string",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "empty string vs non-empty",
			a:        "",
			b:        "x",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := maputils.Equal(tc.a, tc.b)
			assert.Equal(t, tc.expected, result, "valuesEqual(%v, %v)", tc.a, tc.b)
		})
	}
}

// TestRecalculate_DerivedFieldUpdate tests behavior when client sends update for a computed field.
// Current behavior: the update is applied, then recalculation overwrites it with the formula result.
func TestRecalculate_DerivedFieldUpdate(t *testing.T) {
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

	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	// User tries to set derived.dmg directly (should be recomputed)
	originalData := map[string]any{
		"stats":   map[string]any{"str": 16.0},
		"mod":     map[string]any{"str": 3.0},
		"derived": map[string]any{"dmg": 8.0}, // correct value
	}
	updates := map[string]any{
		"derived.dmg": 999.0, // malicious/incorrect client value
	}

	cascadingEffects, err := dag.Recalculate(originalData, updates)
	require.NoError(t, err)

	// Formula should win: derived.dmg = mod_str(3) + 5 = 8
	assert.Equal(t, float64(8.0), cascadingEffects["derived.dmg"])
	assert.Equal(t, float64(8.0), cascadingEffects["derived.dmg"])
}

// TestRecalculate_RuntimeFormulaError tests handling of formula evaluation edge cases.
// The expr-lang engine returns +Inf/-Inf for division by zero rather than erroring.
// Type mismatches (e.g., string + number) will return an error from Evaluate.
func TestRecalculate_RuntimeFormulaError(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	// Test 1: Division by zero returns +Inf (not an error in expr-lang)
	fieldsDivZero := FieldGroup{
		"input": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		"output": {Schema: &FieldSchema{
			Type:       "number",
			IsEditable: false,
			Formula:    "10 / (input - 10)", // +Inf when input = 10
			DependsOn:  [][]string{{"input"}},
		}},
	}

	dag, err := BuildDAG("test", fieldsDivZero, formulaEngine)
	require.NoError(t, err)

	originalData := map[string]any{
		"input":  10.0,
		"output": 0.0,
	}
	updates := map[string]any{"input": 10.0}

	cascadingEffects, err := dag.Recalculate(originalData, updates)
	require.NoError(t, err)
	// +Inf != 0, so it propagates as a change
	require.Contains(t, cascadingEffects, "output")
	assert.True(t, math.IsInf(cascadingEffects["output"].(float64), 1), "expected +Inf")

	// Test 2: Valid computation after
	originalData2 := map[string]any{"input": 10.0, "output": math.Inf(1)}
	updates2 := map[string]any{"input": 20.0} // 10 / (20 - 10) = 1.0

	cascadingEffects2, err := dag.Recalculate(originalData2, updates2)
	require.NoError(t, err)
	assert.Equal(t, float64(1.0), cascadingEffects2["output"])

	// Test 3: Type mismatch error (string + number) - expr-lang returns error
	fieldsTypeErr := FieldGroup{
		"input": {Schema: &FieldSchema{Type: "string", IsEditable: true}},
		"output": {Schema: &FieldSchema{
			Type:       "number",
			IsEditable: false,
			Formula:    "input + 10", // string + number = type error
			DependsOn:  [][]string{{"input"}},
		}},
	}

	dag2, err := BuildDAG("test2", fieldsTypeErr, formulaEngine)
	require.NoError(t, err)

	originalData3 := map[string]any{"input": "hello", "output": 0.0}
	updates3 := map[string]any{"input": "world"}

	cascadingEffects3, err := dag2.Recalculate(originalData3, updates3)

	require.Error(t, err, "Type mismatch during formula evaluation should return error to caller")
	assert.ErrorContains(t, err, "invalid operation") // expr-lang error message
	assert.Empty(t, cascadingEffects3)
}
