package sheettest

import (
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
)

const (
	CapabilitiesTemplateID = "capabilities_test_v1"

	CapabilitiesListPath = "inventory.items"

	CapabilitiesRecordActive = "r0123456789abcdef0123456789abcdef"
	CapabilitiesRecordIdle   = "r0123456789abcdef0123456789abcdee"

	CapabilitiesActionRest      = "rest"
	CapabilitiesActionRestLabel = "Long rest"

	CapabilitiesFieldDice      = "{stats.level}d{stats.proficiency} min 50"
	CapabilitiesRowDice        = "1d20 + stats.level"
	CapabilitiesInsightFormula = "min(stats_level * 2, lookup(stats_level, 1, 10, 5, 20))"

	capabilitiesStatsSection       = "stats"
	capabilitiesFlagsSection       = "flags"
	capabilitiesTotalsSection      = "totals"
	capabilitiesInventorySection   = "inventory"
	capabilitiesItemsField         = "items"
	capabilitiesProficiencyField   = "proficiency"
	capabilitiesInsightField       = "insight"
	capabilitiesInspiredField      = "inspired"
	capabilitiesOptionAField       = "optionA"
	capabilitiesOptionBField       = "optionB"
	capabilitiesPickGroup          = "pick"
	capabilitiesDerivedTotalField  = "derivedTotal"
	capabilitiesScaledField        = "scaled"
	capabilitiesValueField         = "inventoryValue"
	capabilitiesCountField         = "inventoryCount"
	capabilitiesTextType           = "text"
	capabilitiesDerivedFormula     = "record_quantity * record_weight"
	capabilitiesScaledFormula      = "record_quantity * stats_level"
	capabilitiesLevelRowKey        = "stats.level"
	capabilitiesInsightRowKey      = "stats.insight"
	capabilitiesQuantityResetValue = "record_quantity + 1"
	capabilitiesLevelResetValue    = "max(1, stats_level - 1)"
)

func CapabilitiesShape() template.FieldGroup {
	maxLength := 20
	return template.FieldGroup{
		listNameField: {Schema: &template.FieldSchema{
			Type:       capabilitiesTextType,
			Label:      "Name",
			IsEditable: true,
			MaxLength:  &maxLength,
		}},
		listQuantityField: {Schema: &template.FieldSchema{
			Type:       listNumberType,
			Label:      "Quantity",
			IsEditable: true,
			Default:    1,
		}},
		listWeightField: {Schema: &template.FieldSchema{
			Type:       listNumberType,
			Label:      "Weight",
			IsEditable: true,
			Default:    1,
		}},
		listActiveField: {Schema: &template.FieldSchema{
			Type:       listNumberType,
			Label:      "Active",
			IsEditable: true,
			Flag:       true,
			Default:    0,
		}},
		capabilitiesOptionAField: {Schema: &template.FieldSchema{
			Type:           listNumberType,
			Label:          "Option A",
			IsEditable:     true,
			ExclusiveGroup: capabilitiesPickGroup,
			OptionLabel:    "A",
		}},
		capabilitiesOptionBField: {Schema: &template.FieldSchema{
			Type:           listNumberType,
			Label:          "Option B",
			IsEditable:     true,
			ExclusiveGroup: capabilitiesPickGroup,
			OptionLabel:    "B",
		}},
		capabilitiesDerivedTotalField: {Schema: &template.FieldSchema{
			Type:      listNumberType,
			Label:     "Derived total",
			Formula:   capabilitiesDerivedFormula,
			DependsOn: [][]string{{listQuantityField}, {listWeightField}},
		}},
		capabilitiesScaledField: {Schema: &template.FieldSchema{
			Type:      listNumberType,
			Label:     "Scaled",
			Formula:   capabilitiesScaledFormula,
			DependsOn: [][]string{{listQuantityField}, {capabilitiesStatsSection, listLevelField}},
		}},
	}
}

func CapabilitiesFields() template.FieldGroup {
	return template.FieldGroup{
		capabilitiesStatsSection: {Label: "Stats", Children: template.FieldGroup{
			listLevelField: {Schema: &template.FieldSchema{
				Type:       listNumberType,
				Label:      "Level",
				IsEditable: true,
				Default:    1,
			}},
			capabilitiesProficiencyField: {Schema: &template.FieldSchema{
				Type:       listNumberType,
				Label:      "Proficiency",
				IsEditable: true,
				Default:    2,
			}},
			capabilitiesInsightField: {Schema: &template.FieldSchema{
				Type:      listNumberType,
				Label:     "Insight",
				Formula:   CapabilitiesInsightFormula,
				DependsOn: [][]string{{capabilitiesStatsSection, listLevelField}},
			}},
		}},
		capabilitiesFlagsSection: {Label: "Flags", Children: template.FieldGroup{
			capabilitiesInspiredField: {Schema: &template.FieldSchema{
				Type:       listNumberType,
				Label:      "Inspired",
				IsEditable: true,
				Flag:       true,
				Default:    0,
			}},
		}},
		capabilitiesInventorySection: {Label: "Inventory", Children: template.FieldGroup{
			capabilitiesItemsField: {List: &template.ListSchema{
				Label:    "Items",
				Columns:  []string{listNameField, listQuantityField, listWeightField, listActiveField},
				Capacity: 3,
				Dice:     "1d6",
				Shape:    CapabilitiesShape(),
			}},
		}},
		capabilitiesTotalsSection: {Label: "Totals", Children: template.FieldGroup{
			capabilitiesValueField: {Schema: &template.FieldSchema{
				Type:  listNumberType,
				Label: "Inventory value",
				Aggregate: &template.AggregateSchema{
					List:  CapabilitiesListPath,
					Field: capabilitiesDerivedTotalField,
					Op:    template.AggregateSum,
				},
			}},
			capabilitiesCountField: {Schema: &template.FieldSchema{
				Type:  listNumberType,
				Label: "Inventory count",
				Aggregate: &template.AggregateSchema{
					List:  CapabilitiesListPath,
					Field: listQuantityField,
					Op:    template.AggregateCount,
				},
			}},
		}},
	}
}

func CapabilitiesResetActions() []template.ResetAction {
	return []template.ResetAction{
		{
			ID:    CapabilitiesActionRest,
			Label: CapabilitiesActionRestLabel,
			Assignments: []template.ResetAssignment{
				{Target: capabilitiesLevelRowKey, Expression: capabilitiesLevelResetValue},
				{
					Target:     CapabilitiesListPath + "." + listQuantityField,
					Expression: capabilitiesQuantityResetValue,
					Filter:     listActiveField,
				},
			},
		},
	}
}

func CapabilitiesTemplate() template.Template {
	return template.Template{
		ID:          CapabilitiesTemplateID,
		Name:        "Capabilities Test",
		Kinds:       []plan.SheetKind{plan.KindNPC, plan.KindEnemy},
		Fields:      CapabilitiesFields(),
		SectionDice: map[string]string{capabilitiesStatsSection: CapabilitiesFieldDice},
		RowDice:     map[string]string{capabilitiesInsightRowKey: CapabilitiesRowDice},
		NotD20Rows:  []string{capabilitiesInsightRowKey},
		Actions:     CapabilitiesResetActions(),
	}
}

func CapabilitiesDAG(t *testing.T) *template.DAG {
	t.Helper()

	dag, err := template.BuildDAGWithActions(
		CapabilitiesTemplateID,
		CapabilitiesFields(),
		CapabilitiesResetActions(),
		engine.NewFormulaEngine(),
	)
	if err != nil {
		t.Fatalf("build capabilities DAG: %v", err)
	}
	return dag
}

func CapabilitiesBoot(t *testing.T) {
	t.Helper()

	tmpl := CapabilitiesTemplate()
	CapabilitiesDAG(t)
	if err := template.ValidateDiceDefaults(tmpl); err != nil {
		t.Fatalf("capabilities dice defaults: %v", err)
	}
	if err := template.ValidateNotD20Tests(tmpl); err != nil {
		t.Fatalf("capabilities not-d20 marking: %v", err)
	}
	if err := template.ValidateFlags(tmpl); err != nil {
		t.Fatalf("capabilities flags: %v", err)
	}
	if err := template.ValidateGenerationMetadata(tmpl); err != nil {
		t.Fatalf("capabilities generation metadata: %v", err)
	}
	if err := template.ValidateNameLeaf(tmpl); err != nil {
		t.Fatalf("capabilities name leaf: %v", err)
	}
}

func CapabilitiesSeedData() map[string]any {
	return map[string]any{
		capabilitiesStatsSection: map[string]any{
			listLevelField:               3.0,
			capabilitiesProficiencyField: 2.0,
			capabilitiesInsightField:     6.0,
		},
		capabilitiesFlagsSection: map[string]any{capabilitiesInspiredField: 1.0},
		capabilitiesInventorySection: map[string]any{
			capabilitiesItemsField: map[string]any{
				CapabilitiesRecordActive: map[string]any{
					listNameField:                 "Torch",
					listQuantityField:             2.0,
					listWeightField:               2.0,
					listActiveField:               1.0,
					capabilitiesOptionAField:      1.0,
					capabilitiesOptionBField:      0.0,
					template.ListPositionField:    1.0,
					capabilitiesDerivedTotalField: 4.0,
					capabilitiesScaledField:       6.0,
				},
				CapabilitiesRecordIdle: map[string]any{
					listNameField:                 "Rope",
					listQuantityField:             1.0,
					listWeightField:               1.0,
					listActiveField:               0.0,
					capabilitiesOptionAField:      0.0,
					capabilitiesOptionBField:      1.0,
					template.ListPositionField:    2.0,
					capabilitiesDerivedTotalField: 1.0,
					capabilitiesScaledField:       3.0,
				},
			},
		},
		capabilitiesTotalsSection: map[string]any{
			capabilitiesValueField: 5.0,
			capabilitiesCountField: 1.0,
		},
	}
}
