package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listLeaf() FieldNode {
	return FieldNode{Schema: &FieldSchema{Type: fieldTypeText, Label: "Name", IsEditable: true}}
}

func listDeclaration(capacity int) *ListSchema {
	return &ListSchema{
		Label:    "Items",
		Columns:  []string{"name"},
		Capacity: capacity,
		Shape: FieldGroup{
			"name": listLeaf(),
		},
	}
}

func listFields(list *ListSchema) FieldGroup {
	return FieldGroup{
		"inventory": {Children: FieldGroup{
			"items": {List: list},
		}},
	}
}

func TestBuildDAG_ListDeclaration(t *testing.T) {
	tests := []struct {
		name   string
		fields FieldGroup
		check  func(t *testing.T, err error)
	}{
		{
			name:   "Happy path: a flat list with a capacity builds and is registered",
			fields: listFields(listDeclaration(10)),
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "Edge: capacity one and two hundred are both allowed",
			fields: listFields(listDeclaration(1)),
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Error: reserved position field in the shape",
			fields: listFields(&ListSchema{Capacity: 2, Shape: FieldGroup{
				ListPositionField: listLeaf(),
			}}),
			check: func(t *testing.T, err error) {
				var target *ReservedListFieldError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, ListPositionField, target.Field)
			},
		},
		{
			name: "Error: invalid shape key",
			fields: listFields(&ListSchema{Capacity: 2, Shape: FieldGroup{
				"bad name": listLeaf(),
			}}),
			check: func(t *testing.T, err error) {
				var target *FieldNameError
				require.ErrorAs(t, err, &target)
				assert.Contains(t, target.Field, "bad name")
			},
		},
		{
			name: "Error: a list inside a shape",
			fields: listFields(&ListSchema{Capacity: 2, Shape: FieldGroup{
				"inner": {List: listDeclaration(1)},
			}}),
			check: func(t *testing.T, err error) {
				var target *NestedListError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name: "Error: a group inside a shape",
			fields: listFields(&ListSchema{Capacity: 2, Shape: FieldGroup{
				"group": {Children: FieldGroup{"name": listLeaf()}},
			}}),
			check: func(t *testing.T, err error) {
				var target *InvalidListShapeError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name: "Error: a list path deeper than three segments",
			fields: FieldGroup{
				"a": {Children: FieldGroup{
					"b": {Children: FieldGroup{
						"c": {Children: FieldGroup{
							"d": {List: listDeclaration(1)},
						}},
					}},
				}},
			},
			check: func(t *testing.T, err error) {
				var target *ListPathTooDeepError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, "a.b.c.d", target.Path)
			},
		},
		{
			name:   "Error: capacity below one",
			fields: listFields(listDeclaration(0)),
			check: func(t *testing.T, err error) {
				var target *ListCapacityError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, 0, target.Capacity)
			},
		},
		{
			name:   "Error: capacity above two hundred",
			fields: listFields(listDeclaration(201)),
			check: func(t *testing.T, err error) {
				var target *ListCapacityError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, 201, target.Capacity)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dag, err := BuildDAG("list_test_v1", tc.fields, engine.NewFormulaEngine())
			tc.check(t, err)
			if err == nil {
				require.NotNil(t, dag)
			}
		})
	}
}

func TestBuildDAG_ListShapeFlags(t *testing.T) {
	tests := []struct {
		name   string
		schema *FieldSchema
		check  func(t *testing.T, err error)
	}{
		{
			name:   "Happy path: a flagged number inside a shape validates",
			schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 1},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "Error: a flag on a text leaf inside a shape",
			schema: &FieldSchema{Type: fieldTypeText, IsEditable: true, Flag: true},
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrFlagNotNumber)
			},
		},
		{
			name:   "Error: a flag default other than zero or one inside a shape",
			schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Flag: true, Default: 2},
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrFlagDefaultInvalid)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := listFields(&ListSchema{Capacity: 2, Shape: FieldGroup{"toggle": {Schema: tc.schema}}})
			_, err := BuildDAG("list_test_v1", fields, engine.NewFormulaEngine())
			require.NoError(t, err)
			err = ValidateFlags(Template{ID: "list_test_v1", Fields: fields})
			tc.check(t, err)
		})
	}
}

func TestDAG_ListsAreNotStaticNodes(t *testing.T) {
	dag, err := BuildDAG("list_test_v1", listFields(listDeclaration(2)), engine.NewFormulaEngine())
	require.NoError(t, err)

	assert.False(t, dag.HasNode("inventory.items"))
	assert.False(t, dag.HasNode("inventory.items.name"))
	assert.Empty(t, dag.GetChildren("inventory.items"))
}

func TestListSchema_SeedRecord(t *testing.T) {
	maxLength := 5
	list := &ListSchema{Capacity: 2, Shape: FieldGroup{
		"name":     {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true, MaxLength: &maxLength}},
		"quantity": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 3}},
		"active":   {Schema: &FieldSchema{Type: fieldTypeBool, IsEditable: true, Default: 1}},
	}}

	record := list.SeedRecord(4)

	assert.Equal(t, map[string]any{
		"name":            "",
		"quantity":        3.0,
		"active":          false,
		ListPositionField: 4.0,
	}, record)
}

func TestListErrors(t *testing.T) {
	err := ErrReservedListField("inventory.items", ListPositionField)
	assert.ErrorAs(t, err, new(*ReservedListFieldError))
}

func TestListDTO_MarshalJSON(t *testing.T) {
	tmpl := Template{
		ID:   "list_test_v1",
		Name: "List Test",
		Fields: FieldGroup{
			"inventory": {Children: FieldGroup{
				"items": {List: &ListSchema{
					Label:    "Items",
					Columns:  []string{"name", "quantity"},
					Capacity: 12,
					Dice:     "1d20 + @.quantity",
					NotD20:   true,
					Shape: FieldGroup{
						"name":     {Schema: &FieldSchema{Type: fieldTypeText, Label: "Name", IsEditable: true}},
						"quantity": {Schema: &FieldSchema{Type: fieldTypeNumber, Label: "Qty", IsEditable: true}},
					},
				}},
			}},
		},
	}

	raw, err := json.Marshal(tmpl.ToDTO())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"id": "list_test_v1",
		"name": "List Test",
		"default": false,
		"fields": {
			"inventory": {
				"children": {
					"items": {
						"list": {
							"label": "Items",
							"columns": ["name", "quantity"],
							"capacity": 12,
							"dice": "1d20 + @.quantity",
							"not_d20": true,
							"shape": {
								"name": {"schema": {"type": "text", "label": "Name", "is_editable": true, "default": 0}},
								"quantity": {"schema": {"type": "number", "label": "Qty", "is_editable": true, "default": 0}}
							}
						}
					}
				}
			}
		}
	}`, string(raw))
}

func TestFantasyV1_DTO_HasNoListNodes(t *testing.T) {
	raw, err := json.Marshal(fantasyV1Template.ToDTO())
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"capacity"`)
	assert.NotContains(t, string(raw), `"shape"`)
}
