package template

import (
	"legnaris/internal/engine"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareData(t *testing.T) {
	baseDAG := func() *DAG {
		dag, err := BuildDAG("test", FieldGroup{
			"stats": {Children: FieldGroup{
				"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
			}},
		}, engine.NewFormulaEngine())
		require.NoError(t, err)
		return dag
	}

	tests := []struct {
		name          string
		originalData  map[string]any
		updates       map[string]any
		wantSheetData map[string]any
		wantEngineEnv map[string]any
		assertErr     func(t *testing.T, err error)
	}{
		{
			name: "Happy path: single update merged into copy",
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			updates: map[string]any{
				"stats.str": 16.0,
			},
			wantSheetData: map[string]any{
				"stats": map[string]any{"str": 16.0},
			},
			wantEngineEnv: map[string]any{
				"stats_str": 16.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Happy path: nested new path created by update",
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			updates: map[string]any{
				"derived.bonus": 5.0,
			},
			wantSheetData: map[string]any{
				"stats":   map[string]any{"str": 10.0},
				"derived": map[string]any{"bonus": 5.0},
			},
			wantEngineEnv: map[string]any{
				"stats_str":     10.0,
				"derived_bonus": 5.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Happy path: no updates - returns copy and env unchanged",
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			updates: map[string]any{},
			wantSheetData: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			wantEngineEnv: map[string]any{
				"stats_str": 10.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:         "Edge: empty original data",
			originalData: map[string]any{},
			updates: map[string]any{
				"a.b": 1.0,
			},
			wantSheetData: map[string]any{
				"a": map[string]any{"b": 1.0},
			},
			wantEngineEnv: map[string]any{
				"a_b": 1.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: multi-level deep nesting",
			originalData: map[string]any{
				"character": map[string]any{
					"attributes": map[string]any{
						"physical": map[string]any{"strength": 10.0},
					},
				},
			},
			updates: map[string]any{
				"character.attributes.physical.strength": 15.0,
			},
			wantSheetData: map[string]any{
				"character": map[string]any{
					"attributes": map[string]any{
						"physical": map[string]any{"strength": 15.0},
					},
				},
			},
			wantEngineEnv: map[string]any{
				"character_attributes_physical_strength": 15.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: original data must not be mutated (deep copy)",
			originalData: map[string]any{
				"stats": map[string]any{"str": 10.0},
			},
			updates: map[string]any{
				"stats.str": 16.0,
			},
			wantSheetData: map[string]any{
				"stats": map[string]any{"str": 16.0},
			},
			wantEngineEnv: map[string]any{
				"stats_str": 16.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag := baseDAG()

			originalBefore := DeepCopyForTest(tc.originalData)
			sheetData, engineEnv, err := dag.prepareData(tc.originalData, tc.updates)
			tc.assertErr(t, err)

			if err == nil {
				assert.Equal(t, tc.wantSheetData, sheetData)
				assert.Equal(t, tc.wantEngineEnv, engineEnv)
				assert.Equal(t, originalBefore, tc.originalData, "original data must not be mutated")
			}
		})
	}
}

func TestWalkTopological(t *testing.T) {
	buildDAG := func(fields FieldGroup) *DAG {
		dag, err := BuildDAG("test", fields, engine.NewFormulaEngine())
		require.NoError(t, err)
		return dag
	}

	// chainDAG: stats.str → mod.str → derived.dmg
	chainDAG := func() *DAG {
		return buildDAG(FieldGroup{
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
				"dmg": {Schema: &FieldSchema{
					Type: "number", IsEditable: false,
					Formula:   "mod_str + 5",
					DependsOn: [][]string{{"mod", "str"}},
				}},
			}},
		})
	}

	// leafOnlyDAG: single editable field, no derived nodes
	leafOnlyDAG := func() *DAG {
		return buildDAG(FieldGroup{
			"stats": {Children: FieldGroup{
				"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
			}},
		})
	}

	// errorDAG: derived node with runtime type error formula
	errorDAG := func() *DAG {
		return buildDAG(FieldGroup{
			"input": {Schema: &FieldSchema{Type: "string", IsEditable: true}},
			"output": {Schema: &FieldSchema{
				Type: "number", IsEditable: false,
				Formula:   "input + 10",
				DependsOn: [][]string{{"input"}},
			}},
		})
	}

	tests := []struct {
		name        string
		dag         *DAG
		sheetData   map[string]any
		engineEnv   map[string]any
		needsRecalc map[string]bool
		wantChanged map[string]any
		assertErr   func(t *testing.T, err error)
	}{
		{
			name: "Happy path: full cascade chain computed",
			dag:  chainDAG(),
			sheetData: map[string]any{
				"stats":   map[string]any{"str": 16.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			engineEnv: map[string]any{
				"stats_str":   16.0,
				"mod_str":     0.0,
				"derived_dmg": 5.0,
			},
			needsRecalc: map[string]bool{
				"stats.str": true,
			},
			wantChanged: map[string]any{
				"mod.str":     3.0,
				"derived.dmg": 8.0,
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: no node needs recalc - returns empty map",
			dag:  chainDAG(),
			sheetData: map[string]any{
				"stats":   map[string]any{"str": 16.0},
				"mod":     map[string]any{"str": 3.0},
				"derived": map[string]any{"dmg": 8.0},
			},
			engineEnv: map[string]any{
				"stats_str":   16.0,
				"mod_str":     3.0,
				"derived_dmg": 8.0,
			},
			needsRecalc: map[string]bool{},
			wantChanged: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: empty topological order returns empty map",
			dag:  &DAG{nodes: map[string]*Node{}, adjacencyList: map[string][]string{}},
			sheetData: map[string]any{
				"a": 1.0,
			},
			engineEnv:   map[string]any{"a": 1.0},
			needsRecalc: map[string]bool{"a": true},
			wantChanged: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: leaf node only - no derived changes recorded",
			dag:  leafOnlyDAG(),
			sheetData: map[string]any{
				"stats": map[string]any{"str": 16.0},
			},
			engineEnv: map[string]any{
				"stats_str": 16.0,
			},
			needsRecalc: map[string]bool{
				"stats.str": true,
			},
			wantChanged: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: derived value unchanged - short-circuits propagation, not in changed",
			dag:  chainDAG(),
			sheetData: map[string]any{
				"stats":   map[string]any{"str": 16.0},
				"mod":     map[string]any{"str": 3.0},
				"derived": map[string]any{"dmg": 8.0},
			},
			engineEnv: map[string]any{
				"stats_str":   16.0,
				"mod_str":     3.0,
				"derived_dmg": 8.0,
			},
			needsRecalc: map[string]bool{
				"stats.str":   true,
				"mod.str":     true,
				"derived.dmg": true,
			},
			wantChanged: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Edge: runtime formula error returns wrapped error",
			dag:  errorDAG(),
			sheetData: map[string]any{
				"input":  "world",
				"output": 0.0,
			},
			engineEnv: map[string]any{
				"input":  "world",
				"output": 0.0,
			},
			needsRecalc: map[string]bool{
				"input": true,
			},
			wantChanged: nil,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorContains(t, err, "formula evaluation failed for field 'output'")
				assert.ErrorContains(t, err, "invalid operation")
			},
		},
		{
			name: "Edge: needsRecalc has unknown keys (not in DAG) - ignored",
			dag:  chainDAG(),
			sheetData: map[string]any{
				"stats":   map[string]any{"str": 16.0},
				"mod":     map[string]any{"str": 0.0},
				"derived": map[string]any{"dmg": 5.0},
			},
			engineEnv: map[string]any{
				"stats_str":   16.0,
				"mod_str":     0.0,
				"derived_dmg": 5.0,
			},
			needsRecalc: map[string]bool{
				"unknown.key": true,
			},
			wantChanged: map[string]any{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changed, err := tc.dag.walkTopological(tc.sheetData, tc.engineEnv, tc.needsRecalc)
			tc.assertErr(t, err)

			if err == nil {
				assert.Equal(t, tc.wantChanged, changed)
			}
		})
	}
}

func TestPropagateChanges(t *testing.T) {
	tests := []struct {
		name            string
		adj             map[string][]string
		nodeKey         string
		needsRecalc     map[string]bool
		wantNeedsRecalc map[string]bool
	}{
		{
			name: "Happy path: marks all direct children",
			adj: map[string][]string{
				"a": {"b", "c"},
			},
			nodeKey:     "a",
			needsRecalc: map[string]bool{"a": true},
			wantNeedsRecalc: map[string]bool{
				"a": true, "b": true, "c": true,
			},
		},
		{
			name:            "Edge: node has no children - no-op",
			adj:             map[string][]string{},
			nodeKey:         "a",
			needsRecalc:     map[string]bool{"a": true},
			wantNeedsRecalc: map[string]bool{"a": true},
		},
		{
			name:            "Edge: node not in adjacency list - no-op",
			adj:             map[string][]string{"b": {"c"}},
			nodeKey:         "a",
			needsRecalc:     map[string]bool{"a": true},
			wantNeedsRecalc: map[string]bool{"a": true},
		},
		{
			name: "Edge: children already marked - idempotent",
			adj: map[string][]string{
				"a": {"b"},
			},
			nodeKey:     "a",
			needsRecalc: map[string]bool{"a": true, "b": true},
			wantNeedsRecalc: map[string]bool{
				"a": true, "b": true,
			},
		},
		{
			name: "Edge: empty needsRecalc map - children added",
			adj: map[string][]string{
				"a": {"b"},
			},
			nodeKey:         "a",
			needsRecalc:     map[string]bool{},
			wantNeedsRecalc: map[string]bool{"b": true},
		},
		{
			name: "Edge: diamond fan-out - all branches marked",
			adj: map[string][]string{
				"a": {"b", "c", "d"},
			},
			nodeKey:     "a",
			needsRecalc: map[string]bool{},
			wantNeedsRecalc: map[string]bool{
				"b": true, "c": true, "d": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &DAG{adjacencyList: tc.adj}
			d.propagateChanges(tc.nodeKey, tc.adj, tc.needsRecalc)
			assert.Equal(t, tc.wantNeedsRecalc, tc.needsRecalc)
		})
	}
}

func TestValidateRecalculateInputs(t *testing.T) {
	tests := []struct {
		name        string
		original    map[string]any
		updated     map[string]any
		wantErrText string
	}{
		{
			name:        "nil originalData",
			original:    nil,
			updated:     map[string]any{"a": 1.0},
			wantErrText: "original data is nil",
		},
		{
			name:        "nil updatedData",
			original:    map[string]any{"a": 1.0},
			updated:     nil,
			wantErrText: "updated data is nil",
		},
		{
			name:        "both nil",
			original:    nil,
			updated:     nil,
			wantErrText: "original data is nil",
		},
		{
			name:     "both valid - no error",
			original: map[string]any{"a": 1.0},
			updated:  map[string]any{"a": 2.0},
		},
		{
			name:     "empty maps - no error",
			original: map[string]any{},
			updated:  map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRecalculateInputs(tc.original, tc.updated)
			if tc.wantErrText == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErrText)
		})
	}
}

func TestChangedUpdates(t *testing.T) {
	original := map[string]any{
		"stats": map[string]any{"str": 10.0},
		"mod":   map[string]any{"str": 0.0},
	}

	tests := []struct {
		name     string
		original map[string]any
		updates  map[string]any
		want     map[string]bool
	}{
		{
			name:     "changed leaf key is kept",
			original: original,
			updates:  map[string]any{"stats.str": 16.0},
			want:     map[string]bool{"stats.str": true},
		},
		{
			name:     "deep flat key is kept as a single key",
			original: map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": 1.0}}}},
			updates:  map[string]any{"a.b.c.d": 2.0},
			want:     map[string]bool{"a.b.c.d": true},
		},
		{
			name:     "unchanged leaf key is dropped",
			original: original,
			updates:  map[string]any{"stats.str": 10.0},
			want:     map[string]bool{},
		},
		{
			name:     "leaf key differing within epsilon is dropped",
			original: original,
			updates:  map[string]any{"stats.str": 10.0 + 1e-10},
			want:     map[string]bool{},
		},
		{
			name:     "leaf key absent from original counts as a change",
			original: original,
			updates:  map[string]any{"stats.dex": 14.0},
			want:     map[string]bool{"stats.dex": true},
		},
		{
			name:     "nil value for a key absent from original counts as a change",
			original: original,
			updates:  map[string]any{"stats.bonus": nil},
			want:     map[string]bool{"stats.bonus": true},
		},
		{
			name:     "group key expands to changed leaf keys",
			original: original,
			updates:  map[string]any{"stats": map[string]any{"str": 16.0}},
			want:     map[string]bool{"stats.str": true},
		},
		{
			name:     "group key with unchanged leaf is dropped",
			original: original,
			updates:  map[string]any{"stats": map[string]any{"str": 10.0}},
			want:     map[string]bool{},
		},
		{
			name:     "nested group key expands to full leaf path",
			original: original,
			updates: map[string]any{
				"outer": map[string]any{"inner": map[string]any{"str": 16.0}},
			},
			want: map[string]bool{"outer.inner.str": true},
		},
		{
			name:     "mixed leaf and group keys are expanded independently",
			original: original,
			updates: map[string]any{
				"mod.str": 5.0,
				"stats":   map[string]any{"str": 16.0},
			},
			want: map[string]bool{
				"mod.str":   true,
				"stats.str": true,
			},
		},
		{
			name:     "empty original keeps every update",
			original: map[string]any{},
			updates:  map[string]any{"stats.str": 10.0},
			want:     map[string]bool{"stats.str": true},
		},
		{
			name:     "empty updates yield an empty set",
			original: original,
			updates:  map[string]any{},
			want:     map[string]bool{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, changedUpdates(tc.original, tc.updates))
		})
	}
}

func TestRecalculate_NilDAG(t *testing.T) {
	var dag *DAG
	result, err := dag.Recalculate(map[string]any{"a": 1.0}, map[string]any{"a": 2.0})
	require.Error(t, err)
	assert.ErrorContains(t, err, "DAG is nil")
	assert.Nil(t, result)
}

func TestProcessNode_NilNode(t *testing.T) {
	d := &DAG{formulaEngine: engine.NewFormulaEngine()}
	propagated, err := d.processNode(nil, map[string]any{}, map[string]any{}, map[string]any{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "node is nil")
	assert.False(t, propagated)
}

func TestBuildDAG_NilEngine(t *testing.T) {
	dag, err := BuildDAG("test", FieldGroup{
		"stats": {Children: FieldGroup{
			"str": {Schema: &FieldSchema{Type: "number", IsEditable: true}},
		}},
	}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "formula engine is required")
	assert.Nil(t, dag)
}
