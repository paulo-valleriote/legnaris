package template

// DefaultTemplates is the in-memory catalog of code-defined default templates.
// Each entry is validated and compiled into a DAG once at boot (see Registry).
// Templates are immutable by ID: rule changes ship as new versioned IDs
// (e.g. fantasy_v2), never by mutating an existing entry.
// This variable is exported and therefore mutable; BuildRegistry clones every
// entry, so changing it after boot does not touch a running registry (LGN-54).
var DefaultTemplates = []Template{
	FantasyV1Seed,
}

// FantasyV1Seed defines the fantasy_v1 default template. It lives in code only
// and is served from the in-memory Registry (no MongoDB seed or template
// collection anymore).
var FantasyV1Seed = Template{
	ID:      "fantasy_v1",
	Name:    "Fantasy Template",
	Default: true,
	Fields: FieldGroup{
		"stats": {Children: FieldGroup{
			"dexterity":    {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"constitution": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"others": {Children: FieldGroup{
			"proficiencyBonus": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 2}},
		}},
		"modifiers": {Children: FieldGroup{
			"dexterityMod": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
			"constitutionMod": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_constitution - 10) / 2)",
				DependsOn:  [][]string{{"stats", "constitution"}},
			}},
		}},
		"config": {Children: FieldGroup{
			"acUsesDex":         {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1}},
			"acUsesCon":         {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"stealthProfWeight": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"spellUsesInt":      {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"spellUsesWis":      {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
		}},
		"baseArmorClass": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: FieldGroup{
			"armorClass": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "baseArmorClass + (modifiers_dexterityMod * config_acUsesDex) + (modifiers_constitutionMod * config_acUsesCon)",
				DependsOn: [][]string{
					{"baseArmorClass"},
					{"modifiers", "dexterityMod"},
					{"config", "acUsesDex"},
					{"modifiers", "constitutionMod"},
					{"config", "acUsesCon"},
				},
			}},
			"stealthTotal": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "modifiers_dexterityMod + floor(others_proficiencyBonus * config_stealthProfWeight)",
				DependsOn: [][]string{
					{"modifiers", "dexterityMod"},
					{"others", "proficiencyBonus"},
					{"config", "stealthProfWeight"},
				},
			}},
		}},
	},
}
