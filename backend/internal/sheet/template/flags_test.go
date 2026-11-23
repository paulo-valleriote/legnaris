package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func flagLeaf(schema FieldSchema) FieldNode {
	return FieldNode{Schema: &schema}
}

func flagTemplate(group FieldGroup) Template {
	return Template{ID: "flag_test", State: StateActive, Fields: group}
}

func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    Template
		wantErr error
	}{
		{
			name: "Edge: a template with no fields is valid",
			tmpl: flagTemplate(nil),
		},
		{
			name: "Edge: a leaf without the marker is valid whatever its type",
			tmpl: flagTemplate(FieldGroup{
				"note":   flagLeaf(FieldSchema{Type: fieldTypeText, IsEditable: true}),
				"hidden": flagLeaf(FieldSchema{Type: fieldTypeBool, IsEditable: true}),
				"total":  flagLeaf(FieldSchema{Type: fieldTypeNumber, Formula: "1"}),
			}),
		},
		{
			name: "Happy path: a flag on an editable number leaf defaulting to zero is valid",
			tmpl: flagTemplate(FieldGroup{"features": {Children: FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 0}),
			}}}),
		},
		{
			name: "Happy path: a flag on an editable number leaf defaulting to one is valid",
			tmpl: flagTemplate(FieldGroup{"features": {Children: FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 1}),
			}}}),
		},
		{
			name: "Error: a flag on a text leaf is rejected",
			tmpl: flagTemplate(FieldGroup{
				"note": flagLeaf(FieldSchema{Type: fieldTypeText, IsEditable: true, Flag: true}),
			}),
			wantErr: ErrFlagNotNumber,
		},
		{
			name: "Error: a flag on a bool leaf is rejected",
			tmpl: flagTemplate(FieldGroup{
				"hidden": flagLeaf(FieldSchema{Type: fieldTypeBool, IsEditable: true, Flag: true}),
			}),
			wantErr: ErrFlagNotNumber,
		},
		{
			name: "Error: a flag on a derived number leaf is rejected",
			tmpl: flagTemplate(FieldGroup{
				"total": flagLeaf(FieldSchema{Type: fieldTypeNumber, Formula: "1", Flag: true}),
			}),
			wantErr: ErrFlagNotEditable,
		},
		{
			name: "Error: a flag on a derived text leaf is rejected",
			tmpl: flagTemplate(FieldGroup{
				"label": flagLeaf(FieldSchema{Type: fieldTypeText, Formula: `"x"`, Flag: true}),
			}),
			wantErr: ErrFlagNotNumber,
		},
		{
			name: "Error: a flag default of two is rejected",
			tmpl: flagTemplate(FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 2}),
			}),
			wantErr: ErrFlagDefaultInvalid,
		},
		{
			name: "Error: a flag default of minus one is rejected",
			tmpl: flagTemplate(FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: -1}),
			}),
			wantErr: ErrFlagDefaultInvalid,
		},
		{
			name: "Error: a fractional flag default is rejected",
			tmpl: flagTemplate(FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 0.5}),
			}),
			wantErr: ErrFlagDefaultInvalid,
		},
		{
			name: "Error: a violation nested under a group is reported",
			tmpl: flagTemplate(FieldGroup{"features": {Children: FieldGroup{
				"enabled": flagLeaf(FieldSchema{Type: fieldTypeText, IsEditable: true, Flag: true}),
			}}}),
			wantErr: ErrFlagNotNumber,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFlags(tc.tmpl)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestValidateFlags_ErrorNamesField(t *testing.T) {
	tmpl := flagTemplate(FieldGroup{"features": {Children: FieldGroup{
		"enabled": flagLeaf(FieldSchema{Type: fieldTypeText, IsEditable: true, Flag: true}),
	}}})

	err := ValidateFlags(tmpl)

	require.ErrorIs(t, err, ErrFlagNotNumber)
	assert.Contains(t, err.Error(), "features.enabled")
}

func TestBuildRegistry_RejectsBadFlag(t *testing.T) {
	bad := flagTemplate(FieldGroup{"enabled": flagLeaf(FieldSchema{
		Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 5,
	})})
	bad.Name = "bad"

	_, err := buildRegistry(engine.NewFormulaEngine(), []Template{bad})

	require.ErrorIs(t, err, ErrFlagDefaultInvalid)
}

func TestFantasyV1_NoFlagLeaves(t *testing.T) {
	schemas, paths := collectLeaves(fantasyV1Template.Fields)
	require.NotEmpty(t, schemas)

	for i, schema := range schemas {
		assert.False(t, schema.Flag, "leaf %s must not carry the flag marker", paths[i].String())
	}
}

func TestFieldSchemaDTO_MarshalJSON_Flag(t *testing.T) {
	tests := []struct {
		name   string
		schema FieldSchema
		want   string
	}{
		{
			name:   "Happy path: a flag marker is serialized",
			schema: FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 1, Flag: true},
			want:   `{"type":"number","label":"","is_editable":true,"default":1,"flag":true}`,
		},
		{
			name:   "Edge: an absent flag marker omits the key",
			schema: FieldSchema{Type: fieldTypeNumber, IsEditable: true},
			want:   `{"type":"number","label":"","is_editable":true,"default":0}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := tc.schema.toDTO()
			require.NotNil(t, dto)
			raw, err := json.Marshal(dto)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(raw))
		})
	}
}

func TestFieldSchemaToDTO_CopiesFlag(t *testing.T) {
	source := &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true}

	dto := source.toDTO()
	require.NotNil(t, dto)
	assert.True(t, dto.Flag)

	dto.Flag = false
	assert.True(t, source.Flag, "mutating the DTO changed the source schema")
}

func flagDAG(t *testing.T) *DAG {
	t.Helper()

	fields := FieldGroup{
		"features": {Children: FieldGroup{
			"enabled": flagLeaf(FieldSchema{
				Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 0,
			}),
		}},
		"derived": {Children: FieldGroup{
			"bonus": flagLeaf(FieldSchema{
				Type:       fieldTypeNumber,
				Formula:    "features_enabled * 5",
				DependsOn:  [][]string{{"features", "enabled"}},
				IsEditable: false,
			}),
		}},
	}

	dag, err := BuildDAG("flag_runtime", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func flagSheet(value any) map[string]any {
	return map[string]any{
		"features": map[string]any{"enabled": value},
		"derived":  map[string]any{"bonus": 0.0},
	}
}

func TestDAG_FlagValues(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "Happy path: zero is accepted", value: 0.0},
		{name: "Happy path: one is accepted", value: 1.0},
		{name: "Edge: an integer one is accepted", value: 1},
		{name: "Error: two is refused", value: 2.0, wantErr: true},
		{name: "Error: minus one is refused", value: -1.0, wantErr: true},
		{name: "Error: one half is refused", value: 0.5, wantErr: true},
		{name: "Error: a numeric string is refused", value: "1", wantErr: true},
		{name: "Error: null is refused", value: nil, wantErr: true},
		{name: "Error: a bool is refused", value: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag := flagDAG(t)

			_, _, err := dag.NormalizeAndRecalculate(flagSheet(0.0), map[string]any{
				"features.enabled": tc.value,
			})

			if !tc.wantErr {
				require.NoError(t, err)
				return
			}
			var invalid *FlagValueError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, "features.enabled", invalid.Key)
		})
	}
}

func TestDAG_FlagValueOnRecalculate(t *testing.T) {
	dag := flagDAG(t)

	_, err := dag.Recalculate(flagSheet(0.0), map[string]any{"features.enabled": 2.0})

	var invalid *FlagValueError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "features.enabled", invalid.Key)
}

func TestDAG_FlagFormulaReadsTheFlag(t *testing.T) {
	dag := flagDAG(t)

	cascade, err := dag.Recalculate(flagSheet(0.0), map[string]any{"features.enabled": 1.0})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"derived.bonus": 5.0}, cascade)
}
