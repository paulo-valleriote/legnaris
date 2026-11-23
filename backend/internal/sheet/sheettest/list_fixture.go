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

	listNameField  = "name"
	listNumberType = "number"
)

var ListFields = template.FieldGroup{
	listNameField: {Schema: &template.FieldSchema{Type: "text", IsEditable: true}},
	"inventory": {Children: template.FieldGroup{
		"items": {List: &template.ListSchema{
			Label:    "Items",
			Columns:  []string{"name", "quantity", "active"},
			Capacity: 2,
			Shape:    ListShape(),
		}},
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
		"quantity": {Schema: &template.FieldSchema{
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
		"derivedTotal": {Schema: &template.FieldSchema{
			Type:       listNumberType,
			IsEditable: false,
			Formula:    "quantity * 2",
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
		"name": "",
		"inventory": map[string]any{
			"items": map[string]any{},
		},
	}
}
