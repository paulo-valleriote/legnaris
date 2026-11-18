package template

import (
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f64(v float64) *float64 { return &v }

func intPtr(v int) *int { return &v }

func genLeaf(schema FieldSchema) FieldNode {
	if schema.Type == "" {
		schema.Type = fieldTypeNumber
	}
	schema.IsEditable = true
	return FieldNode{Schema: &schema}
}

func genTemplate(group FieldGroup) Template {
	return Template{ID: "gen_test", Fields: group}
}

func TestValidateGenerationMetadata(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    Template
		wantErr error
	}{
		{
			name: "Edge: a template with no fields is valid",
			tmpl: genTemplate(nil),
		},
		{
			name: "Edge: a leaf with no metadata stays valid",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{})}),
		},
		{
			name: "Happy path: a generatable number leaf with a range is valid",
			tmpl: genTemplate(FieldGroup{"stats": {Children: FieldGroup{
				"strength": genLeaf(FieldSchema{Generatable: true, Min: f64(3), Max: f64(20)}),
			}}}),
		},
		{
			name: "Edge: min equal to max is valid",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{Generatable: true, Min: f64(5), Max: f64(5)})}),
		},
		{
			name: "Edge: an explicit zero minimum is a bound, not an absence",
			tmpl: genTemplate(FieldGroup{"flag": genLeaf(FieldSchema{Generatable: true, Min: f64(0), Max: f64(1)})}),
		},
		{
			name: "Edge: a negative range is valid",
			tmpl: genTemplate(FieldGroup{"bonus": genLeaf(FieldSchema{Generatable: true, Min: f64(-10), Max: f64(-1)})}),
		},
		{
			name: "Edge: a minimum without a maximum is valid",
			tmpl: genTemplate(FieldGroup{"xp": genLeaf(FieldSchema{Generatable: true, Min: f64(0)})}),
		},
		{
			name: "Edge: a maximum without a minimum is valid",
			tmpl: genTemplate(FieldGroup{"xp": genLeaf(FieldSchema{Max: f64(100)})}),
		},
		{
			name: "Happy path: a generatable text leaf with a length bound is valid",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{Type: fieldTypeText, Generatable: true, MaxLength: intPtr(60)})}),
		},
		{
			name: "Edge: a max length of one is the smallest valid bound",
			tmpl: genTemplate(FieldGroup{"initial": genLeaf(FieldSchema{Type: fieldTypeText, MaxLength: intPtr(1)})}),
		},
		{
			name: "Edge: range metadata on a non-generatable editable leaf is valid",
			tmpl: genTemplate(FieldGroup{"rules": {Children: FieldGroup{
				"tuning": genLeaf(FieldSchema{Min: f64(1), Max: f64(2)}),
			}}}),
		},
		{
			name: "Happy path: a generatable leaf restricted to some kinds is valid",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{
				Type: fieldTypeText, Generatable: true,
				GeneratableKinds: []plan.SheetKind{plan.KindCharacter, plan.KindNPC},
			})}),
		},
		{
			name: "Happy path: a generatable leaf that prefers its default inside its range is valid",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{
				Generatable: true, Default: 1, Min: f64(1), Max: f64(20), PreferDefault: true,
			})}),
		},
		{
			name: "Edge: a preferred default equal to the minimum is valid",
			tmpl: genTemplate(FieldGroup{"xp": genLeaf(FieldSchema{
				Generatable: true, Default: 0, Min: f64(0), Max: f64(355000), PreferDefault: true,
			})}),
		},
		{
			name: "Edge: a preferred default equal to the maximum is valid",
			tmpl: genTemplate(FieldGroup{"bonus": genLeaf(FieldSchema{
				Generatable: true, Default: 6, Min: f64(2), Max: f64(6), PreferDefault: true,
			})}),
		},
		{
			name: "Edge: a preferred default without bounds is valid",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{
				Generatable: true, Default: 5, PreferDefault: true,
			})}),
		},
		{
			name:    "Error: a preferred default on a non-generatable leaf is rejected",
			tmpl:    genTemplate(FieldGroup{"level": genLeaf(FieldSchema{Default: 1, PreferDefault: true})}),
			wantErr: ErrPreferDefaultOnNonGeneratable,
		},
		{
			name: "Error: a preferred default below the minimum is rejected",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{
				Generatable: true, Default: 0, Min: f64(1), Max: f64(20), PreferDefault: true,
			})}),
			wantErr: ErrPreferDefaultOutOfBounds,
		},
		{
			name: "Error: a preferred default above the maximum is rejected",
			tmpl: genTemplate(FieldGroup{"level": genLeaf(FieldSchema{
				Generatable: true, Default: 21, Min: f64(1), Max: f64(20), PreferDefault: true,
			})}),
			wantErr: ErrPreferDefaultOutOfBounds,
		},
		{
			name: "Error: a preferred default below a lone minimum is rejected",
			tmpl: genTemplate(FieldGroup{"xp": genLeaf(FieldSchema{
				Generatable: true, Default: -1, Min: f64(0), PreferDefault: true,
			})}),
			wantErr: ErrPreferDefaultOutOfBounds,
		},
		{
			name: "Edge: an empty kind restriction means every kind",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{
				Type: fieldTypeText, Generatable: true, GeneratableKinds: []plan.SheetKind{},
			})}),
		},
		{
			name: "Error: a kind restriction on a non-generatable leaf is rejected",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{
				Type: fieldTypeText, GeneratableKinds: []plan.SheetKind{plan.KindCharacter},
			})}),
			wantErr: ErrGeneratableKindsOnNonGeneratable,
		},
		{
			name: "Error: an unknown generatable kind is rejected",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{
				Type: fieldTypeText, Generatable: true,
				GeneratableKinds: []plan.SheetKind{plan.SheetKind("monster")},
			})}),
			wantErr: ErrGeneratableKindInvalid,
		},
		{
			name: "Error: an empty generatable kind is rejected",
			tmpl: genTemplate(FieldGroup{"name": genLeaf(FieldSchema{
				Type: fieldTypeText, Generatable: true,
				GeneratableKinds: []plan.SheetKind{""},
			})}),
			wantErr: ErrGeneratableKindInvalid,
		},
		{
			name: "Error: a generatable leaf is rejected when it is derived",
			tmpl: genTemplate(FieldGroup{"total": {Schema: &FieldSchema{
				Type: fieldTypeNumber, Formula: "1", Generatable: true,
			}}}),
			wantErr: ErrGeneratableNotEditable,
		},
		{
			name: "Error: a generatable literal that is neither editable nor derived is rejected",
			tmpl: genTemplate(FieldGroup{"fixed": {Schema: &FieldSchema{
				Type: fieldTypeNumber, Generatable: true,
			}}}),
			wantErr: ErrGeneratableNotEditable,
		},
		{
			name:    "Error: a minimum above the maximum is rejected",
			tmpl:    genTemplate(FieldGroup{"level": genLeaf(FieldSchema{Generatable: true, Min: f64(20), Max: f64(1)})}),
			wantErr: ErrRangeInverted,
		},
		{
			name:    "Error: a range on a text leaf is rejected",
			tmpl:    genTemplate(FieldGroup{"name": genLeaf(FieldSchema{Type: fieldTypeText, Min: f64(0)})}),
			wantErr: ErrRangeOnNonNumber,
		},
		{
			name:    "Error: a range on a bool leaf is rejected",
			tmpl:    genTemplate(FieldGroup{"flag": genLeaf(FieldSchema{Type: fieldTypeBool, Max: f64(1)})}),
			wantErr: ErrRangeOnNonNumber,
		},
		{
			name:    "Error: a max length on a number leaf is rejected",
			tmpl:    genTemplate(FieldGroup{"level": genLeaf(FieldSchema{MaxLength: intPtr(3)})}),
			wantErr: ErrMaxLengthOnNonText,
		},
		{
			name:    "Error: a zero max length is rejected",
			tmpl:    genTemplate(FieldGroup{"name": genLeaf(FieldSchema{Type: fieldTypeText, MaxLength: intPtr(0)})}),
			wantErr: ErrMaxLengthInvalid,
		},
		{
			name:    "Error: a negative max length is rejected",
			tmpl:    genTemplate(FieldGroup{"name": genLeaf(FieldSchema{Type: fieldTypeText, MaxLength: intPtr(-1)})}),
			wantErr: ErrMaxLengthInvalid,
		},
		{
			name: "Error: a violation nested under a group is reported",
			tmpl: genTemplate(FieldGroup{"stats": {Children: FieldGroup{
				"strength": genLeaf(FieldSchema{Generatable: true, Min: f64(9), Max: f64(3)}),
			}}}),
			wantErr: ErrRangeInverted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateGenerationMetadata(tc.tmpl)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestValidateGenerationMetadata_ErrorNamesField(t *testing.T) {
	tmpl := genTemplate(FieldGroup{"stats": {Children: FieldGroup{
		"strength": genLeaf(FieldSchema{Generatable: true, Min: f64(9), Max: f64(3)}),
	}}})

	err := ValidateGenerationMetadata(tmpl)

	require.ErrorIs(t, err, ErrRangeInverted)
	assert.Contains(t, err.Error(), "stats.strength")
}

func TestValidateGenerationMetadata_Deterministic(t *testing.T) {
	tmpl := genTemplate(FieldGroup{
		"alpha": genLeaf(FieldSchema{Generatable: true, Min: f64(2), Max: f64(1)}),
		"beta":  genLeaf(FieldSchema{Generatable: true, Min: f64(2), Max: f64(1)}),
	})

	for range 20 {
		err := ValidateGenerationMetadata(tmpl)
		require.ErrorIs(t, err, ErrRangeInverted)
		assert.Contains(t, err.Error(), "alpha")
	}
}

func TestBuildRegistry_RejectsBadGenerationMetadata(t *testing.T) {
	bad := genTemplate(FieldGroup{"level": genLeaf(FieldSchema{Generatable: true, Min: f64(5), Max: f64(1)})})
	bad.Name = "bad"

	_, err := buildRegistry(engine.NewFormulaEngine(), []Template{bad})

	require.ErrorIs(t, err, ErrRangeInverted)
}
