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

func preferDefaultFixtureTemplate() *template.Template {
	return &template.Template{
		ID:   fixtureID,
		Name: "Prefer default",
		Fields: template.FieldGroup{
			"identity": {Children: template.FieldGroup{
				"level": {Schema: &template.FieldSchema{
					Type: "number", Label: "Level", IsEditable: true, Generatable: true,
					Default: 1, Min: ptr(1.0), Max: ptr(20.0), PreferDefault: true,
				}},
				"xp": {Schema: &template.FieldSchema{
					Type: "number", Label: "Experience points", IsEditable: true, Generatable: true,
					Default: 0, Min: ptr(0.0), Max: ptr(355000.0),
				}},
			}},
		},
	}
}

func listedFixtureTemplate() *template.Template {
	tmpl := fixtureTemplate()
	tmpl.Fields["inventory"] = template.FieldNode{Children: template.FieldGroup{
		"items": {List: &template.ListSchema{
			Label:    "Items",
			Capacity: 2,
			Shape: template.FieldGroup{
				"name": {Schema: &template.FieldSchema{
					Type: "text", Label: "Name", IsEditable: true,
				}},
			},
		}},
	}}
	return tmpl
}

func recordFixtureTemplate() *template.Template {
	return &template.Template{
		ID:   fixtureID,
		Name: "Records",
		Fields: template.FieldGroup{
			"inventory": {Children: template.FieldGroup{
				"items": {List: &template.ListSchema{
					Label:      "Items",
					Capacity:   5,
					MaxRecords: 2,
					Shape: template.FieldGroup{
						"name": {Schema: &template.FieldSchema{
							Type: "text", Label: "Name", IsEditable: true,
						}},
						"quantity": {Schema: &template.FieldSchema{
							Type: "number", Label: "Quantity", IsEditable: true, Default: 1,
						}},
						"weight": {Schema: &template.FieldSchema{
							Type: "number", Label: "Weight", IsEditable: true,
						}},
						"equipped": {Schema: &template.FieldSchema{
							Type: "number", Label: "Equipped", IsEditable: true, Flag: true,
						}},
						"swing": {Schema: &template.FieldSchema{
							Type: "number", Label: "Usage", IsEditable: true,
							ExclusiveGroup: "usage", OptionLabel: "Swing",
						}},
						"thrown": {Schema: &template.FieldSchema{
							Type: "number", Label: "Usage", IsEditable: true,
							ExclusiveGroup: "usage", OptionLabel: "Thrown",
						}},
						"lineWeight": {Schema: &template.FieldSchema{
							Type: "number", Label: "Line weight",
							Formula:   "record_quantity * record_weight",
							DependsOn: [][]string{{"quantity"}, {"weight"}},
						}},
					},
				}},
				"total": {Schema: &template.FieldSchema{
					Type: "number", Label: "Total weight",
					Aggregate: &template.AggregateSchema{
						List: "inventory.items", Field: "lineWeight", Op: template.AggregateSum,
					},
				}},
			}},
		},
	}
}

func identityFixtureTemplate() *template.Template {
	names, ok := template.DefaultGameSystems().AllowedNames(template.GameSystemFifth)
	if !ok {
		panic("missing fifth-edition game system")
	}
	text := func(label string, enum []string) template.FieldNode {
		return template.FieldNode{Schema: &template.FieldSchema{
			Type: "text", Label: label, IsEditable: true, Generatable: true,
			MaxLength: ptr(60), GenerationEnum: enum,
		}}
	}
	return &template.Template{
		ID:           fixtureID,
		Name:         "Identity",
		GameSystemID: template.GameSystemFifth,
		ClassSubclass: &template.ClassSubclassRule{
			Class:         []string{"identity", "class"},
			Subclass:      []string{"identity", "subclass"},
			Level:         []string{"identity", "level"},
			SubclassLevel: 3,
		},
		Fields: template.FieldGroup{
			"identity": {Children: template.FieldGroup{
				"level": {Schema: &template.FieldSchema{
					Type: "number", IsEditable: true, Generatable: true,
					Default: 1, Min: ptr(1.0), Max: ptr(20.0),
				}},
				"class":      text("Class", names.Classes),
				"subclass":   text("Subclass", append(append([]string{}, names.Subclasses...), "")),
				"species":    text("Species", names.Species),
				"background": text("Background", names.Backgrounds),
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
