package template

import (
	"errors"
	"legnaris/internal/engine"
	"legnaris/shared/maputils"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// topologicalOrderOf reads the DAG evaluation order under the read lock,
// mirroring the exported accessor removed in LGN-55 without bypassing the
// locking contract (never read guarded state without holding mu).
func topologicalOrderOf(d *DAG) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	order := make([]string, len(d.topologicalOrder))
	copy(order, d.topologicalOrder)
	return order
}

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
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInfiniteCycle)
				var cycleErr *CycleError
				require.ErrorAs(t, err, &cycleErr)
				assert.NotEmpty(t, cycleErr.Nodes)
			},
		},
		{
			name: "Infinite Loop (cycle of 3+ nodes)",
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
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInfiniteCycle)
				var cycleErr *CycleError
				require.ErrorAs(t, err, &cycleErr)
				assert.Len(t, cycleErr.Nodes, 3)
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
				expectedMsg := NewFormulaCompileError("floor((stats_str - 10) / ", "mod.str", dummyErr).Error()
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
				assert.NotEmpty(t, topologicalOrderOf(dag))
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
		{
			// LGN-59: Updating an editable field with the same value
			// (even via type coercion, e.g., int 10 vs float64 10.0)
			// must not trigger cascading recalculation.
			name: "LGN-59: Editable node unchanged (int vs float64) prevents cascade",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats.str": int(10), // int(10) == float64(10.0) → no real change
			},
			expectedCascades: map[string]any{},
		},
		{
			// LGN-59: Float rounding — updating with a value that differs
			// only by floating-point rounding should not cascade.
			name: "LGN-59: Editable node unchanged (float epsilon) prevents cascade",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats.str": 10.0 + 1e-10, // within epsilon → no real change
			},
			expectedCascades: map[string]any{},
		},
		{
			// LGN-59: Updating an editable field with a value that differs
			// by MORE than epsilon MUST trigger cascading recalculation.
			name: "LGN-59: Editable node changed (exceeds epsilon) triggers cascade",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats.str": 16.0, // large change → real change, mod.str becomes 3, dmg becomes 8
			},
			expectedCascades: map[string]any{
				"mod.str":     3.0, // floor((16 - 10) / 2) = 3
				"derived.dmg": 8.0, // 3 + 5 = 8
			},
		},
		{
			// LGN-59: A group-level update key ("stats") is expanded to its
			// leaf path keys, so it cascades exactly like the leaf key does.
			name: "LGN-59: Group-level update expands to leaf keys and cascades",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats": map[string]any{"str": 16.0},
			},
			expectedCascades: map[string]any{
				"mod.str":     3.0,
				"derived.dmg": 8.0,
			},
		},
		{
			// LGN-59: A group-level update whose leaves are unchanged must
			// not start a cascade either.
			name: "LGN-59: Group-level update without real change is a no-op",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			expectedCascades: map[string]any{},
		},
		{
			// LGN-59: The epsilon check runs on the expanded leaves, so a
			// group-level key still suppresses a rounding-only difference.
			name: "LGN-59: Group-level update within epsilon is a no-op",
			originalData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			updates: map[string]any{
				"stats": map[string]any{"str": 10.0 + 1e-10},
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
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInfiniteCycle)
				var cycleErr *CycleError
				require.ErrorAs(t, err, &cycleErr)
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
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInfiniteCycle)
				var cycleErr *CycleError
				require.ErrorAs(t, err, &cycleErr)
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
				require.Error(t, err)
				// Returns a DependencyError instead of a compilation error
				var depErr *DependencyError
				require.ErrorAs(t, err, &depErr)
				assert.Equal(t, "derived", depErr.TargetNode)
				assert.Equal(t, []string{"nonexistent"}, depErr.DepPath)
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
				node, ok := dag.GetNode("derived")
				require.True(t, ok)
				assert.Equal(t, "base * 2", node.FormulaString)
				assert.True(t, node.IsDerived())
				assert.Len(t, dag.GetChildren("base"), 2) // Current behavior: 2 entries
			},
		},
		{
			name:   "Template with 0 fields (empty FieldGroup)",
			fields: FieldGroup{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err) // Current behavior
			},
			assertDag: func(t *testing.T, dag *DAG) {
				assert.Empty(t, topologicalOrderOf(dag))
			},
		},
		{
			// LGN-41 / LGN-52: before field-name validation these names built
			// fine, but "stats.str.dex" and "stats.str_dex" are distinct DAG
			// nodes that collapse into the same engine key "stats_str_dex" —
			// one value silently overwriting the other in the expr-lang env.
			name: "LGN-41/LGN-52: names with dots or underscores are rejected",
			fields: FieldGroup{
				"stats": {Children: FieldGroup{
					"str.dex": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
					"str_dex": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
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
				require.Error(t, err)
				var nameErr *FieldNameError
				require.ErrorAs(t, err, &nameErr)
			},
			assertDag: nil,
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

// TestBuildDAG_FieldNameValidation covers the key-collision fix of LGN-52
// (tracked as LGN-41 in dag_test.go): field names may not contain '.' or '_',
// otherwise Path.String() (DAG map keys) and Path.EngineKey() (expr-lang
// variables) stop being injective and two distinct fields collapse into a
// single key.
func TestBuildDAG_FieldNameValidation(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	leaf := &FieldSchema{Type: "number", IsEditable: true}

	// Document the raw collisions the validation prevents:
	// two distinct paths, the same key.
	assert.Equal(t, Path{"a_b", "c"}.EngineKey(), Path{"a", "b_c"}.EngineKey()) // "a_b_c"
	assert.Equal(t, Path{"a.b", "c"}.String(), Path{"a", "b", "c"}.String())    // "a.b.c"

	tests := []struct {
		name string
		// fields is the template field tree under test.
		fields FieldGroup
		// wantField is the full path expected in the FieldNameError.
		// Empty means "don't assert the field" (e.g. the empty-name case).
		wantField string
		// wantNode, when set, flips the case to the success path: the DAG
		// must build and contain this node.
		wantNode string
	}{
		{
			name:      "dot in group name is rejected",
			fields:    FieldGroup{"a.b": {Children: FieldGroup{"c": {Schema: leaf}}}},
			wantField: "a.b",
		},
		{
			name:      "underscore in leaf name is rejected",
			fields:    FieldGroup{"stats": {Children: FieldGroup{"dexterity_mod": {Schema: leaf}}}},
			wantField: "stats.dexterity_mod",
		},
		{
			// Before LGN-52 this built successfully, but both paths engine-key
			// to "a_b_c": ToEngineEnv collapsed them into one entry whose value
			// depended on Go's map iteration order.
			name: "collision a_b.c vs a.b_c is rejected",
			fields: FieldGroup{
				"a_b": {Children: FieldGroup{"c": {Schema: leaf}}},
				"a":   {Children: FieldGroup{"b_c": {Schema: leaf}}},
			},
		},
		{
			// Both paths string-key to "a.b.c": the second node would shadow
			// the first in d.nodes, and ApplyUpdates would split the key and
			// write to the wrong nesting level.
			name: "collision a.b.c vs a.b.c is rejected",
			fields: FieldGroup{
				"a.b": {Children: FieldGroup{"c": {Schema: leaf}}},
				"a":   {Children: FieldGroup{"b": {Children: FieldGroup{"c": {Schema: leaf}}}}},
			},
		},
		{
			name:   "empty name is rejected",
			fields: FieldGroup{"": {Schema: leaf}},
		},
		{
			name:      "leading digit is rejected",
			fields:    FieldGroup{"stats": {Children: FieldGroup{"2nd": {Schema: leaf}}}},
			wantField: "stats.2nd",
		},
		{
			name: "valid names build",
			fields: FieldGroup{
				"stats":  {Children: FieldGroup{"dexterity": {Schema: leaf}}},
				"others": {Children: FieldGroup{"proficiencyBonus": {Schema: leaf}}},
			},
			wantNode: "others.proficiencyBonus",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("test_template", tc.fields, formulaEngine)

			if tc.wantNode != "" {
				require.NoError(t, err)
				assert.True(t, dag.HasNode(tc.wantNode))
				return
			}

			require.Error(t, err)
			var nameErr *FieldNameError
			require.ErrorAs(t, err, &nameErr)
			if tc.wantField != "" {
				assert.Equal(t, tc.wantField, nameErr.Field)
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
	}

	baseDag, err := BuildDAG("test", baseFields, formulaEngine)
	require.NoError(t, err)

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
				assert.ErrorIs(t, err, ErrNilOriginalData)
			},
		},
		{
			name:         "nil updatedData",
			dag:          baseDag,
			originalData: map[string]any{"stats": map[string]any{"str": 10.0}},
			updates:      nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrNilUpdatedData)
			},
		},
		{
			name: "Unknown field in updates",
			dag:  baseDag,
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
				"mod":   map[string]any{"str": 0.0},
			},
			updates: map[string]any{
				"stats.str":     16.0,
				"unknown.field": 99.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var fieldErr *UnknownFieldError
				require.ErrorAs(t, err, &fieldErr)
				assert.Equal(t, "unknown.field", fieldErr.Key)
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
		{
			// LGN-59: Updating a new editable field not present in originalData
			// (found == false) should propagate as a change.
			name: "LGN-59: New editable field (found==false) propagates",
			dag:  baseDag,
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
				"mod":   map[string]any{"str": 0.0},
			},
			updates: map[string]any{
				"stats.dex": 14.0, // new field not in originalData
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertResult: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(2.0), result["mod.dex"]) // floor((14-10)/2) = 2
			},
		},
		{
			// LGN-59: Equal update (within epsilon) must suppress formula
			// evaluation entirely — formula should not run, no errors from it.
			// Uses a formula that would error (division by zero) if evaluated.
			name: "LGN-59: Equal update suppresses formula evaluation (div by zero)",
			dag: func() *DAG {
				fields := FieldGroup{
					"input": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
					"output": {Schema: &FieldSchema{
						Type:       "number",
						IsEditable: false,
						Formula:    "10 / (input - 10)", // +Inf/error when input = 10
						DependsOn:  [][]string{{"input"}},
					}},
				}
				dag, _ := BuildDAG("test", fields, formulaEngine)
				return dag
			}(),
			originalData: map[string]any{
				"input":  10.0,
				"output": math.Inf(1), // previous result was +Inf
			},
			updates: map[string]any{
				"input": 10.0 + 1e-10, // within epsilon → filtered out, formula not evaluated
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err) // no error because formula not evaluated
			},
			assertResult: func(t *testing.T, result map[string]any) {
				assert.Empty(t, result) // no cascade because update was filtered
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
		"input":  5.0,
		"output": 0.0,
	}
	// Update input to 10.0 to trigger division by zero (+Inf)
	// Note: this update differs from the original, so it passes the LGN-59 filter
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

// TestTopologicalSort_CycleDetection verifies that cycle detection returns the correct nodes.
func TestTopologicalSort_CycleDetection(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	tests := []struct {
		name             string
		fields           FieldGroup
		expectedCycleLen int
	}{
		{
			name: "2-node cycle identifies both nodes",
			fields: FieldGroup{
				"a": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula: "b + 1", DependsOn: [][]string{{"b"}},
				}},
				"b": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula: "a + 1", DependsOn: [][]string{{"a"}},
				}},
			},
			expectedCycleLen: 2,
		},
		{
			name: "3-node cycle identifies all nodes",
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
			expectedCycleLen: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDAG("test", tc.fields, formulaEngine)
			require.Error(t, err)

			var cycleErr *CycleError
			require.ErrorAs(t, err, &cycleErr)
			assert.Len(t, cycleErr.Nodes, tc.expectedCycleLen)
		})
	}
}

// TestDetectCycle verifies the standalone cycle-detection helper directly.
func TestDetectCycle(t *testing.T) {
	tests := []struct {
		name   string
		nodes  map[string]*Node
		adj    map[string][]string
		expect []string // nil means no cycle
	}{
		{
			name:   "acyclic graph returns nil",
			nodes:  map[string]*Node{"a": {}, "b": {}},
			adj:    map[string][]string{"a": {"b"}},
			expect: nil,
		},
		{
			name:   "self-loop returns the single node",
			nodes:  map[string]*Node{"a": {}},
			adj:    map[string][]string{"a": {"a"}},
			expect: []string{"a"},
		},
		{
			name:   "cycle behind a prefix returns only the cycle nodes",
			nodes:  map[string]*Node{"x": {}, "a": {}, "b": {}},
			adj:    map[string][]string{"x": {"a"}, "a": {"b"}, "b": {"a"}},
			expect: []string{"a", "b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cycle := detectCycle(tc.nodes, tc.adj)
			if tc.expect == nil {
				require.Nil(t, cycle)
				return
			}
			require.NotNil(t, cycle)
			assert.Len(t, cycle, len(tc.expect))
			for _, want := range tc.expect {
				assert.Contains(t, cycle, want)
			}
		})
	}
}

// TestTopologicalSort_PrefixIntoCycle verifies that a cycle reached through a
// non-cycle prefix reports only the cycle nodes (no prefix leakage).
func TestTopologicalSort_PrefixIntoCycle(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := FieldGroup{
		"x": {Schema: &FieldSchema{
			Type: "number", IsEditable: false,
			Formula:   "a + 1",
			DependsOn: [][]string{{"a"}},
		}},
		"a": {Schema: &FieldSchema{
			Type: "number", IsEditable: false,
			Formula:   "b + 1",
			DependsOn: [][]string{{"b"}},
		}},
		"b": {Schema: &FieldSchema{
			Type: "number", IsEditable: false,
			Formula:   "a + 1",
			DependsOn: [][]string{{"a"}},
		}},
	}
	_, err := BuildDAG("test", fields, formulaEngine)
	require.Error(t, err)

	var cycleErr *CycleError
	require.ErrorAs(t, err, &cycleErr)
	assert.Len(t, cycleErr.Nodes, 2)
	assert.Contains(t, cycleErr.Nodes, "a")
	assert.Contains(t, cycleErr.Nodes, "b")
}

// TestKahnTopoSort verifies that a valid DAG produces a topological order
// where every dependency precedes its dependent.
func TestKahnTopoSort(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := FieldGroup{
		"a": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		"b": {Schema: &FieldSchema{
			Type: "number", IsEditable: false,
			Formula:   "a + 1",
			DependsOn: [][]string{{"a"}},
		}},
		"c": {Schema: &FieldSchema{
			Type: "number", IsEditable: false,
			Formula:   "b + 1",
			DependsOn: [][]string{{"b"}},
		}},
	}
	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)
	order := topologicalOrderOf(dag)

	pos := make(map[string]int, len(order))
	for i, n := range order {
		pos[n] = i
	}
	assert.Less(t, pos["a"], pos["b"], "a must precede b")
	assert.Less(t, pos["b"], pos["c"], "b must precede c")
}

// TestBuildDAG_InvalidDependency verifies that DependsOn referencing non-existent paths returns error.
func TestBuildDAG_InvalidDependency(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	tests := []struct {
		name       string
		fields     FieldGroup
		depPath    []string
		targetNode string
	}{
		{
			name: "DependsOn referencing non-existent path",
			fields: FieldGroup{
				"derived": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "nonexistent + 1",
					DependsOn: [][]string{{"nonexistent"}},
				}},
			},
			depPath:    []string{"nonexistent"},
			targetNode: "derived",
		},
		{
			name: "DependsOn referencing deep non-existent path",
			fields: FieldGroup{
				"derived": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "base_str * 2",
					DependsOn: [][]string{{"base", "str"}},
				}},
			},
			depPath:    []string{"base", "str"},
			targetNode: "derived",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildDAG("test", tc.fields, formulaEngine)
			require.Error(t, err)

			var depErr *DependencyError
			require.ErrorAs(t, err, &depErr)
			assert.Equal(t, tc.targetNode, depErr.TargetNode)
			assert.ErrorContains(t, err, tc.targetNode)
		})
	}
}

// TestRecalculate_UnknownField verifies that updates with unknown fields return error.
func TestRecalculate_UnknownField(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	fields := FieldGroup{
		"stats": {Children: FieldGroup{
			"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		}},
	}

	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	originalData := map[string]any{
		"stats": map[string]any{"str": 10.0},
	}
	updates := map[string]any{
		"stats.str": 16.0,
		"unknown":   99.0,
	}

	_, err = dag.Recalculate(originalData, updates)
	require.Error(t, err)

	var fieldErr *UnknownFieldError
	require.ErrorAs(t, err, &fieldErr)
	assert.Equal(t, "unknown", fieldErr.Key)
}

// TestMapUtilsSet_ErrorCases verifies Set error handling.
func TestMapUtilsSet_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		nested  map[string]any
		path    []string
		value   any
		wantErr error
	}{
		{
			name:    "nil map returns ErrNilMap",
			nested:  nil,
			path:    []string{"a"},
			value:   1.0,
			wantErr: maputils.ErrNilMap,
		},
		{
			name:    "empty path returns ErrEmptyPath",
			nested:  map[string]any{"a": 1.0},
			path:    []string{},
			value:   1.0,
			wantErr: maputils.ErrEmptyPath,
		},
		{
			name:    "intermediate not a map returns ErrNotMap",
			nested:  map[string]any{"a": "not a map"},
			path:    []string{"a", "b"},
			value:   1.0,
			wantErr: maputils.ErrNotMap,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := maputils.Set(tc.nested, tc.path, tc.value)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestPathSet_ErrorCases verifies Path.Set returns errors.
func TestPathSet_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		path    Path
		nested  map[string]any
		value   any
		wantErr error
	}{
		{
			name:    "nil map returns ErrNilMap",
			path:    Path{"a"},
			nested:  nil,
			value:   1.0,
			wantErr: maputils.ErrNilMap,
		},
		{
			name:    "empty path returns ErrEmptyPath",
			path:    Path{},
			nested:  map[string]any{"a": 1.0},
			value:   1.0,
			wantErr: maputils.ErrEmptyPath,
		},
		{
			name:    "intermediate not a map returns ErrNotMap",
			path:    Path{"a", "b"},
			nested:  map[string]any{"a": "not a map"},
			value:   1.0,
			wantErr: maputils.ErrNotMap,
		},
		{
			name:    "valid path succeeds",
			path:    Path{"a"},
			nested:  map[string]any{},
			value:   1.0,
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.path.Set(tc.nested, tc.value)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestApplyUpdates_ErrorCases verifies ApplyUpdates returns errors.
func TestApplyUpdates_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		nested  map[string]any
		updates map[string]any
		wantErr error
	}{
		{
			name:    "intermediate not a map returns ErrNotMap",
			nested:  map[string]any{"a": "not a map"},
			updates: map[string]any{"a.b": 1.0},
			wantErr: maputils.ErrNotMap,
		},
		{
			name:    "nil nested with updates - no error (no-op)",
			nested:  nil,
			updates: map[string]any{"a": 1.0},
			wantErr: nil,
		},
		{
			name:    "nil updates - no error (no-op)",
			nested:  map[string]any{"a": 1.0},
			updates: nil,
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := maputils.ApplyUpdates(tc.nested, tc.updates)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestBuildDAG_ExclusiveGroups pins the index build rules (EXC-08): only
// editable leaves join a group, a group with fewer than two editable members is
// dropped, and members are sorted for deterministic errors.
func TestBuildDAG_ExclusiveGroups(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	fields := FieldGroup{
		"config": {Children: FieldGroup{
			"acUsesDex": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
			"solo":      {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "soloGroup"}},
			"untagged":  {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		}},
		"derived":  {Schema: &FieldSchema{Type: "number", IsEditable: false, ExclusiveGroup: "armorClassAbility"}},
		"derivedA": {Schema: &FieldSchema{Type: "number", IsEditable: false, ExclusiveGroup: "derivedOnly"}},
		"derivedB": {Schema: &FieldSchema{Type: "number", IsEditable: false, ExclusiveGroup: "derivedOnly"}},
	}

	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	dag.mu.RLock()
	groups := dag.exclusiveGroups
	dag.mu.RUnlock()

	assert.Equal(t, map[string][]string{
		"armorClassAbility": {"config.acUsesCon", "config.acUsesDex"},
	}, groups, "derived members must be excluded from the group")
	assert.NotContains(t, groups, "soloGroup", "a single editable member is not an exclusive group")
	assert.NotContains(t, groups, "derivedOnly", "a group with no editable member is dropped")
}

// TestNormalizeExclusiveUpdates covers the update-time collapse (EXC-04,
// EXC-05, EXC-06, EXC-08): one non-zero member zeroes the absent siblings, two
// non-zero members conflict, and an untouched payload is returned as the same
// map without allocation.
func TestNormalizeExclusiveUpdates(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()

	fields := FieldGroup{
		"config": {Children: FieldGroup{
			"acUsesDex": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
			"solo":      {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "soloGroup"}},
			"untagged":  {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		}},
	}

	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	tests := []struct {
		name         string
		updates      map[string]any
		want         map[string]any
		wantSameMap  bool
		addedSibling string
		assertErr    func(t *testing.T, err error)
	}{
		{
			name:         "Happy path: turning one member on zeroes the absent sibling",
			updates:      map[string]any{"config.acUsesDex": 1.0},
			want:         map[string]any{"config.acUsesDex": 1.0, "config.acUsesCon": 0.0},
			addedSibling: "config.acUsesCon",
		},
		{
			name:         "Happy path: an int member on zeroes the absent sibling",
			updates:      map[string]any{"config.acUsesCon": int(1)},
			want:         map[string]any{"config.acUsesCon": int(1), "config.acUsesDex": 0.0},
			addedSibling: "config.acUsesDex",
		},
		{
			name:    "Edge: an explicitly present zero sibling is kept, not duplicated",
			updates: map[string]any{"config.acUsesDex": 1.0, "config.acUsesCon": 0.0},
			want:    map[string]any{"config.acUsesDex": 1.0, "config.acUsesCon": 0.0},
		},
		{
			name:        "Edge: turning a member off leaves siblings unchanged",
			updates:     map[string]any{"config.acUsesDex": 0.0},
			want:        map[string]any{"config.acUsesDex": 0.0},
			wantSameMap: true,
		},
		{
			name:    "Error: a string member is rejected",
			updates: map[string]any{"config.acUsesDex": "1"},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var invalid *ExclusiveValueTypeError
				require.ErrorAs(t, err, &invalid)
				assert.Equal(t, "armorClassAbility", invalid.Group)
				assert.Equal(t, "config.acUsesDex", invalid.Key)
			},
		},
		{
			name:    "Error: a bool member is rejected",
			updates: map[string]any{"config.acUsesCon": true},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var invalid *ExclusiveValueTypeError
				require.ErrorAs(t, err, &invalid)
				assert.Equal(t, "config.acUsesCon", invalid.Key)
			},
		},
		{
			name:    "Error: a nil member is rejected",
			updates: map[string]any{"config.acUsesDex": nil},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var invalid *ExclusiveValueTypeError
				require.ErrorAs(t, err, &invalid)
			},
		},
		{
			name:        "Edge: a single-member group is not enforced",
			updates:     map[string]any{"config.solo": 1.0},
			want:        map[string]any{"config.solo": 1.0},
			wantSameMap: true,
		},
		{
			name:        "Edge: an update touching no exclusive group is returned unchanged",
			updates:     map[string]any{"config.untagged": 5.0},
			want:        map[string]any{"config.untagged": 5.0},
			wantSameMap: true,
		},
		{
			name:    "Error: two non-zero members of one group conflict",
			updates: map[string]any{"config.acUsesDex": 1.0, "config.acUsesCon": 1.0},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var conflict *ExclusiveGroupConflictError
				require.ErrorAs(t, err, &conflict)
				assert.Equal(t, "armorClassAbility", conflict.Group)
				assert.Equal(t, []string{"config.acUsesCon", "config.acUsesDex"}, conflict.Keys)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dag.NormalizeExclusiveUpdates(tc.updates)

			if tc.assertErr != nil {
				tc.assertErr(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			if tc.wantSameMap {
				assert.Equal(t, reflect.ValueOf(tc.updates).Pointer(), reflect.ValueOf(got).Pointer(),
					"an untouched payload must be returned as the same map")
				return
			}

			assert.NotEqual(t, reflect.ValueOf(tc.updates).Pointer(), reflect.ValueOf(got).Pointer(),
				"normalization must return a copy")
			if tc.addedSibling != "" {
				assert.NotContains(t, tc.updates, tc.addedSibling,
					"normalization must not mutate the caller's payload")
			}
		})
	}
}

// TestNormalizeExclusiveUpdates_ConflictGroupIsDeterministic pins the group
// visit order: a payload that turns on two members of two different groups
// always reports the alphabetically first group, instead of whichever entry
// Go's randomized map iteration reached first.
func TestNormalizeExclusiveUpdates_ConflictGroupIsDeterministic(t *testing.T) {
	fields := FieldGroup{
		"config": {Children: FieldGroup{
			"zetaA":  {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "zetaGroup"}},
			"zetaB":  {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "zetaGroup"}},
			"alphaA": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "alphaGroup"}},
			"alphaB": {Schema: &FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "alphaGroup"}},
		}},
	}

	dag, err := BuildDAG("test", fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	updates := map[string]any{
		"config.zetaA":  1.0,
		"config.zetaB":  1.0,
		"config.alphaA": 1.0,
		"config.alphaB": 1.0,
	}

	// Repeated runs give map iteration a fresh seed each time, so a single
	// assertion would only catch the regression probabilistically.
	for i := range 20 {
		got, err := dag.NormalizeExclusiveUpdates(updates)
		require.Error(t, err, "run %d", i)
		assert.Nil(t, got, "run %d", i)

		var conflict *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &conflict, "run %d", i)
		assert.Equal(t, "alphaGroup", conflict.Group,
			"run %d: the alphabetically first conflicting group must win", i)
	}
}

// TestNormalizeAndRecalculate pins the PATCH entry point that validates,
// normalizes and recalculates under a single read-lock acquisition (T3): the
// normalized map zeroes the exclusive sibling, the cascading effects reflect
// the active member, and every typed error still bubbles through it.
func TestNormalizeAndRecalculate(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := FieldGroup{
		"stats": {Children: FieldGroup{
			"dexterity": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"config": {Children: FieldGroup{
			"acUsesDex": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0, ExclusiveGroup: "armorClassAbility"}},
		}},
		"derived": {Children: FieldGroup{
			"armorClass": {Schema: &FieldSchema{
				Type: "number", IsEditable: false,
				Formula:   "10 + stats_dexterity * config_acUsesDex",
				DependsOn: [][]string{{"stats", "dexterity"}, {"config", "acUsesDex"}},
			}},
		}},
	}

	dag, err := BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	original := map[string]any{
		"stats":   map[string]any{"dexterity": 12.0},
		"config":  map[string]any{"acUsesDex": 1.0, "acUsesCon": 0.0},
		"derived": map[string]any{"armorClass": 22.0},
	}

	normalized, cascading, err := dag.NormalizeAndRecalculate(
		original,
		map[string]any{"config.acUsesCon": 1.0},
	)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"config.acUsesCon": 1.0, "config.acUsesDex": 0.0}, normalized)
	assert.Equal(t, map[string]any{"derived.armorClass": 10.0}, cascading)

	t.Run("Error: conflict bubbles through the single-lock entry point", func(t *testing.T) {
		_, _, err := dag.NormalizeAndRecalculate(original, map[string]any{
			"config.acUsesDex": 1.0, "config.acUsesCon": 1.0,
		})
		require.Error(t, err)
		var conflict *ExclusiveGroupConflictError
		require.ErrorAs(t, err, &conflict)
	})

	t.Run("Error: non-numeric member bubbles through the single-lock entry point", func(t *testing.T) {
		_, _, err := dag.NormalizeAndRecalculate(original, map[string]any{"config.acUsesDex": "1"})
		require.Error(t, err)
		var invalid *ExclusiveValueTypeError
		require.ErrorAs(t, err, &invalid)
	})

	t.Run("Error: derived field is not editable", func(t *testing.T) {
		_, _, err := dag.NormalizeAndRecalculate(original, map[string]any{"derived.armorClass": 5.0})
		require.Error(t, err)
		var notEditable *NotEditableError
		require.ErrorAs(t, err, &notEditable)
	})

	t.Run("Error: nil original data is rejected before locking", func(t *testing.T) {
		_, _, err := dag.NormalizeAndRecalculate(nil, map[string]any{"config.acUsesDex": 1.0})
		require.ErrorIs(t, err, ErrNilOriginalData)
	})
}
