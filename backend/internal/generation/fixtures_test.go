package generation_test

import (
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/require"
)

const fixtureID = "fixture_v1"

func ptr[T any](v T) *T {
	return &v
}

func fixtureTemplate() *template.Template {
	return &template.Template{
		ID:   fixtureID,
		Name: "Fixture",
		Fields: template.FieldGroup{
			"identity": {Children: template.FieldGroup{
				"name": {Schema: &template.FieldSchema{
					Type: "text", Label: "Name", IsEditable: true, Generatable: true, MaxLength: ptr(12),
				}},
				"title": {Schema: &template.FieldSchema{
					Type: "text", Label: "Title", IsEditable: true, Generatable: true,
				}},
			}},
			"stats": {Children: template.FieldGroup{
				"strength": {Schema: &template.FieldSchema{
					Type: "number", Label: "Strength", IsEditable: true, Generatable: true,
					Default: 10, Min: ptr(1.0), Max: ptr(20.0),
				}},
				"dexterity": {Schema: &template.FieldSchema{
					Type: "number", Label: "Dexterity", IsEditable: true, Generatable: true,
					Default: 10, Min: ptr(1.0), Max: ptr(20.0),
				}},
				"luck": {Schema: &template.FieldSchema{
					Type: "number", Label: "Luck", IsEditable: true, Default: 7,
				}},
				"hero": {Schema: &template.FieldSchema{
					Type: "bool", Label: "Hero", IsEditable: true, Generatable: true,
				}},
			}},
			"modifiers": {Children: template.FieldGroup{
				"dexterityMod": {Schema: &template.FieldSchema{
					Type: "number", Label: "Dexterity modifier", IsEditable: false,
					Formula: "floor((stats_dexterity - 10) / 2)", DependsOn: [][]string{{"stats", "dexterity"}},
				}},
			}},
			"config": {Children: template.FieldGroup{
				"acUsesDex": {Schema: &template.FieldSchema{
					Type: "number", Label: "AC uses dexterity", IsEditable: true, Generatable: true,
					Default: 1, ExclusiveGroup: "armorClassAbility", OptionLabel: "Dexterity",
				}},
				"acUsesCon": {Schema: &template.FieldSchema{
					Type: "number", Label: "AC uses constitution", IsEditable: true, Generatable: true,
					Default: 0, ExclusiveGroup: "armorClassAbility", OptionLabel: "Constitution",
				}},
			}},
			"rules": {Children: template.FieldGroup{
				"base": {Schema: &template.FieldSchema{
					Type: "number", Label: "Base", IsEditable: true, Default: 8,
				}},
			}},
		},
	}
}

func fixtureDAG(t testing.TB, tmpl *template.Template) *template.DAG {
	t.Helper()
	dag, err := template.BuildDAG(tmpl.ID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

func dependentFixtureTemplate() *template.Template {
	return &template.Template{
		ID:   fixtureID,
		Name: "Dependent",
		Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{
				"dexterity": {Schema: &template.FieldSchema{
					Type: "number", Label: "Dexterity", IsEditable: true, Generatable: true,
					Default: 10, Min: ptr(1.0), Max: ptr(20.0),
				}},
			}},
			"modifiers": {Children: template.FieldGroup{
				"x": {Schema: &template.FieldSchema{
					Type:      "number",
					Label:     "X",
					Formula:   "floor((stats_dexterity - rules_base) / 2)",
					DependsOn: [][]string{{"stats", "dexterity"}, {"rules", "base"}},
				}},
			}},
			"rules": {Children: template.FieldGroup{
				"base": {Schema: &template.FieldSchema{
					Type: "number", Label: "Base", IsEditable: true, Default: 10,
				}},
			}},
		},
	}
}

func brokenFormulaFixtureTemplate() *template.Template {
	return &template.Template{
		ID:   fixtureID,
		Name: "Broken formula",
		Fields: template.FieldGroup{
			"identity": {Children: template.FieldGroup{
				"name": {Schema: &template.FieldSchema{
					Type: "text", Label: "Name", IsEditable: true, Generatable: true,
				}},
			}},
			"modifiers": {Children: template.FieldGroup{
				"broken": {Schema: &template.FieldSchema{
					Type:      "number",
					Label:     "Broken",
					Formula:   "identity_name + 1",
					DependsOn: [][]string{{"identity", "name"}},
				}},
			}},
		},
	}
}
