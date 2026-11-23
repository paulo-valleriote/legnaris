package template

import (
	"encoding/json"
	"testing"

	"legnaris/backend/internal/engine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aggregateShape() FieldGroup {
	return FieldGroup{
		"name":     {Schema: &FieldSchema{Type: fieldTypeText, IsEditable: true}},
		"quantity": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 1}},
		"weight":   {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 2}},
		"total": {Schema: &FieldSchema{
			Type:      fieldTypeNumber,
			Formula:   "record_quantity * record_weight",
			DependsOn: [][]string{{"quantity"}, {"weight"}},
		}},
	}
}

func aggregateSchema(list, field, op string) *AggregateSchema {
	return &AggregateSchema{List: list, Field: field, Op: op}
}

func aggregateFields(aggregates map[string]*AggregateSchema) FieldGroup {
	fields := FieldGroup{
		"inventory": {Children: FieldGroup{
			"items": {List: &ListSchema{
				Label:    "Items",
				Capacity: 10,
				Shape:    aggregateShape(),
			}},
		}},
	}
	for name, aggregate := range aggregates {
		fields[name] = FieldNode{Schema: &FieldSchema{
			Type:      fieldTypeNumber,
			Aggregate: aggregate,
		}}
	}
	return fields
}

func buildAggregateDAG(t *testing.T, fields FieldGroup) (*DAG, error) {
	t.Helper()
	return BuildDAG("aggregate_test_v1", fields, engine.NewFormulaEngine())
}

func TestBuildDAG_Aggregate(t *testing.T) {
	tests := []struct {
		name   string
		fields FieldGroup
		check  func(t *testing.T, err error)
	}{
		{
			name: "Happy path: sum and count aggregates build",
			fields: aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
				"inventoryCount":  aggregateSchema("inventory.items", "quantity", AggregateCount),
			}),
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Error: an aggregate on a missing list",
			fields: aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.missing", "total", AggregateSum),
			}),
			check: func(t *testing.T, err error) {
				var target *InvalidAggregateError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, "inventoryWeight", target.Path)
			},
		},
		{
			name: "Error: an aggregate on a missing shape field",
			fields: aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.items", "missing", AggregateSum),
			}),
			check: func(t *testing.T, err error) {
				var target *InvalidAggregateError
				require.ErrorAs(t, err, &target)
			},
		},
		{
			name: "Error: an unknown operation",
			fields: aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.items", "total", "average"),
			}),
			check: func(t *testing.T, err error) {
				var target *InvalidAggregateError
				require.ErrorAs(t, err, &target)
				assert.Contains(t, target.Reason, "sum")
			},
		},
		{
			name: "Error: an aggregate that also declares a formula",
			fields: func() FieldGroup {
				fields := aggregateFields(map[string]*AggregateSchema{
					"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
				})
				fields["inventoryWeight"].Schema.Formula = "1 + 1"
				return fields
			}(),
			check: func(t *testing.T, err error) {
				var target *InvalidAggregateError
				require.ErrorAs(t, err, &target)
				assert.Contains(t, target.Reason, "formula")
			},
		},
		{
			name: "Error: an aggregate on a text shape field",
			fields: aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.items", "name", AggregateSum),
			}),
			check: func(t *testing.T, err error) {
				var target *InvalidAggregateError
				require.ErrorAs(t, err, &target)
				assert.Contains(t, target.Reason, "number")
			},
		},
		{
			name: "Error: a top-level record field is reserved",
			fields: func() FieldGroup {
				fields := aggregateFields(nil)
				fields["record"] = FieldNode{Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true}}
				return fields
			}(),
			check: func(t *testing.T, err error) {
				var target *FieldNameError
				require.ErrorAs(t, err, &target)
				assert.Equal(t, "record", target.Field)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildAggregateDAG(t, tc.fields)
			tc.check(t, err)
		})
	}
}

func TestBuildDAG_AggregateMustBeDerived(t *testing.T) {
	fields := aggregateFields(map[string]*AggregateSchema{
		"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
	})
	fields["inventoryWeight"].Schema.IsEditable = true

	_, err := buildAggregateDAG(t, fields)
	var target *InvalidAggregateError
	require.ErrorAs(t, err, &target)
	assert.Contains(t, target.Reason, "derived")
}

func TestTemplateToDTO_Aggregate(t *testing.T) {
	fields := aggregateFields(map[string]*AggregateSchema{
		"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
	})
	dag, err := buildAggregateDAG(t, fields)
	require.NoError(t, err)
	require.NotNil(t, dag)

	t.Run("Happy path: the aggregate is exposed as list, field and op", func(t *testing.T) {
		raw, err := json.Marshal(Template{ID: "aggregate_test_v1", Fields: fields}.ToDTO())
		require.NoError(t, err)
		assert.Contains(t, string(raw),
			`"aggregate":{"list":"inventory.items","field":"total","op":"sum"}`)
	})

	t.Run("Edge: the DTO does not alias the aggregate", func(t *testing.T) {
		tmpl := Template{ID: "aggregate_test_v1", Fields: fields}
		dto := tmpl.ToDTO()
		dto.Fields["inventoryWeight"].Schema.Aggregate.Op = AggregateCount
		assert.Equal(t, AggregateSum, fields["inventoryWeight"].Schema.Aggregate.Op)
	})
}

func TestBuildDAG_AggregateCycle(t *testing.T) {
	fields := aggregateFields(map[string]*AggregateSchema{
		"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
	})
	shape := fields["inventory"].Children["items"].List.Shape
	shape["total"].Schema.DependsOn = [][]string{{"quantity"}, {"weight"}, {"inventoryWeight"}}

	_, err := buildAggregateDAG(t, fields)
	var cycle *CycleError
	require.ErrorAs(t, err, &cycle)
	assert.Contains(t, cycle.Nodes, "inventoryWeight")
	assert.Contains(t, cycle.Nodes, "inventory.items.total")
}

func TestBuildDAG_RecordFormulaDependencies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(shape FieldGroup)
		check  func(t *testing.T, err error)
	}{
		{
			name:   "Happy path: a record field and a static field are declared",
			mutate: func(FieldGroup) {},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "Error: an undeclared record field read fails boot",
			mutate: func(shape FieldGroup) {
				shape["total"].Schema.DependsOn = [][]string{{"quantity"}}
			},
			check: func(t *testing.T, err error) {
				var compile *FormulaCompileError
				require.ErrorAs(t, err, &compile)
				assert.Contains(t, compile.Error(), "record_weight")
			},
		},
		{
			name: "Error: a declared record field absent from the shape",
			mutate: func(shape FieldGroup) {
				shape["total"].Schema.DependsOn = [][]string{{"quantity"}, {"weight"}, {"missing"}}
			},
			check: func(t *testing.T, err error) {
				var unknown *UnknownRecordFieldError
				require.ErrorAs(t, err, &unknown)
				assert.Equal(t, "missing", unknown.Field)
			},
		},
		{
			name: "Error: a declared static field absent from the sheet",
			mutate: func(shape FieldGroup) {
				shape["total"].Schema.DependsOn = [][]string{{"quantity"}, {"weight"}, {"stats", "missing"}}
			},
			check: func(t *testing.T, err error) {
				var dependency *DependencyError
				require.ErrorAs(t, err, &dependency)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := aggregateFields(map[string]*AggregateSchema{
				"inventoryWeight": aggregateSchema("inventory.items", "total", AggregateSum),
			})
			tc.mutate(fields["inventory"].Children["items"].List.Shape)
			_, err := buildAggregateDAG(t, fields)
			tc.check(t, err)
		})
	}
}
