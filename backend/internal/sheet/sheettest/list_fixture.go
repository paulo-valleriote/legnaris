package sheettest

import (
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet/template"
)

const (
	ListTemplateID = "records_test_v1"

	ListPath = "inventory.items"

	ListRecordID  = "r0123456789abcdef0123456789abcdef"
	ListRecordID2 = "r0123456789abcdef0123456789abcdee"
	ListRecordID3 = "r0123456789abcdef0123456789abcde1"

	listNameField     = "name"
	listLevelField    = "level"
	listQuantityField = "quantity"
	listWeightField   = "weight"
	listNumberType    = "number"
)

var ListFields = template.FieldGroup{
	listNameField: {Schema: &template.FieldSchema{Type: "text", IsEditable: true}},
	listLevelField: {Schema: &template.FieldSchema{
		Type:       listNumberType,
		IsEditable: true,
		Default:    1,
	}},
	"inventory": {Children: template.FieldGroup{
		"items": {List: &template.ListSchema{
			Label:    "Items",
			Columns:  []string{"name", listQuantityField, listWeightField, "active"},
			Capacity: 2,
			Shape:    ListShape(),
		}},
	}},
	"inventoryValue": {Schema: &template.FieldSchema{
		Type: listNumberType,
		Aggregate: &template.AggregateSchema{
			List:  ListPath,
			Field: "derivedTotal",
			Op:    template.AggregateSum,
		},
	}},
	"inventoryCount": {Schema: &template.FieldSchema{
		Type: listNumberType,
		Aggregate: &template.AggregateSchema{
			List:  ListPath,
			Field: listQuantityField,
			Op:    template.AggregateCount,
		},
	}},
}

func ListShape() template.FieldGroup {
	maxLength := 20
	return template.FieldGroup{
		listNameField: {Schema: &template.FieldSchema{
			Type:       "text",
			IsEditable: true,
			MaxLength:  &maxLength,
		}},
		listQuantityField: {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: true,
			Default:    1,
		}},
		listWeightField: {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: true,
			Default:    1,
		}},
		"active": {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: true,
			Flag:       true,
			Default:    0,
		}},
		"optionA": {Schema: &template.FieldSchema{
			Type:           listNumberType,
			IsEditable:     true,
			ExclusiveGroup: "pick",
		}},
		"optionB": {Schema: &template.FieldSchema{
			Type:           listNumberType,
			IsEditable:     true,
			ExclusiveGroup: "pick",
		}},
		"derivedTotal": {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: false,
			Formula:    "record_quantity * record_weight",
			DependsOn:  [][]string{{"quantity"}, {"weight"}},
		}},
		"scaled": {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: false,
			Formula:    "record_quantity * level",
			DependsOn:  [][]string{{"quantity"}, {"level"}},
		}},
	}
}

func ListTemplate() template.Template {
	return template.Template{
		ID:     ListTemplateID,
		Name:   "Records Test",
		Fields: ListFields,
	}
}

func ListDAG(t *testing.T) *template.DAG {
	t.Helper()
	dag, err := template.BuildDAG(ListTemplateID, ListFields, engine.NewFormulaEngine())
	if err != nil {
		t.Fatalf("build list DAG: %v", err)
	}
	return dag
}

func ListSeedData() map[string]any {
	return map[string]any{
		listNameField:  "",
		listLevelField: 1.0,
		"inventory": map[string]any{
			"items": map[string]any{},
		},
		"inventoryValue": 0.0,
		"inventoryCount": 0.0,
	}
}
