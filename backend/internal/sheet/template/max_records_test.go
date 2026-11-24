package template

import (
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listTemplate(list *ListSchema) Template {
	return genTemplate(FieldGroup{"records": {List: list}})
}

func TestValidateGenerationMetadata_MaxRecords(t *testing.T) {
	tests := []struct {
		name       string
		maxRecords int
		capacity   int
		wantErr    bool
	}{
		{
			name:       "Edge: an undeclared cap is valid",
			maxRecords: 0,
			capacity:   10,
		},
		{
			name:       "Happy path: a cap of one is the smallest declared cap",
			maxRecords: 1,
			capacity:   1,
		},
		{
			name:       "Happy path: a cap below capacity is valid",
			maxRecords: 6,
			capacity:   10,
		},
		{
			name:       "Edge: a cap equal to capacity is valid",
			maxRecords: 10,
			capacity:   10,
		},
		{
			name:       "Error: a negative cap is rejected",
			maxRecords: -1,
			capacity:   10,
			wantErr:    true,
		},
		{
			name:       "Error: a cap above capacity is rejected",
			maxRecords: 11,
			capacity:   10,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := listTemplate(&ListSchema{
				Label:      "Records",
				Capacity:   tc.capacity,
				MaxRecords: tc.maxRecords,
				Shape: FieldGroup{
					"name": genLeaf(FieldSchema{Type: fieldTypeText}),
				},
			})

			err := ValidateGenerationMetadata(tmpl)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrMaxRecordsInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBuildRegistry_RejectsInvalidMaxRecords(t *testing.T) {
	tests := []struct {
		name    string
		max     int
		wantErr bool
	}{
		{name: "Happy path: a cap inside the capacity builds", max: 6},
		{name: "Error: a cap above the capacity fails boot", max: 11, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := listTemplate(&ListSchema{
				Label:      "Records",
				Capacity:   10,
				MaxRecords: tc.max,
				Shape: FieldGroup{
					"name": genLeaf(FieldSchema{Type: fieldTypeText}),
				},
			})

			_, err := buildRegistry(engine.NewFormulaEngine(), []Template{tmpl})
			if tc.wantErr {
				require.ErrorIs(t, err, ErrMaxRecordsInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestFifthTemplates_GenerationRecordCaps(t *testing.T) {
	tests := []struct {
		name       string
		templateID string
		want       map[string]int
	}{
		{
			name:       "Happy path: the 5E Character caps each record list below its capacity",
			templateID: fifthCharacterID,
			want: map[string]int{
				"attacks.list":    6,
				"inventory.items": 20,
				"features.list":   15,
				"spells.list":     30,
				"resources.list":  6,
			},
		},
		{
			name:       "Happy path: the 5E Stat Block caps each record list below its capacity",
			templateID: fifthStatBlockID,
			want: map[string]int{
				"traits":           6,
				"actions":          6,
				"bonusActions":     6,
				"reactions":        6,
				"legendaryActions": 6,
				"resources":        4,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := fifthTemplateByID(t, tc.templateID)
			lists, err := collectLists(tmpl.Fields, Path{})
			require.NoError(t, err)
			require.Len(t, lists, len(tc.want))

			for path, want := range tc.want {
				list, ok := lists[path]
				require.Truef(t, ok, "missing list %s", path)
				assert.Equal(t, want, list.MaxRecords, path)
				assert.Positive(t, list.MaxRecords, path)
				assert.LessOrEqual(t, list.MaxRecords, list.Capacity, path)
			}
		})
	}
}
