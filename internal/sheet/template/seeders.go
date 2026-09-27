package template

const (
	fieldTypeNumber       = "number"
	fieldNameStats        = "stats"
	fieldNameModifiers    = "modifiers"
	fieldNameDexterityMod = "dexterityMod"
	fieldNameConfig       = "config"
)

// DefaultTemplates is the in-memory catalog of code-defined default templates.
// Each entry is validated and compiled into a DAG once at boot (see Registry).
// Templates are immutable by ID: rule changes ship as new versioned IDs
// (e.g. fantasy_v2), never by mutating an existing entry.
// This variable is exported and therefore mutable; BuildRegistry clones every
// entry, so changing it after boot does not touch a running registry (LGN-54).
var DefaultTemplates = []Template{
	FantasyV1Template,
}

// FantasyV1Template defines the fantasy_v1 default template. It lives in code only
// and is served from the in-memory Registry (no MongoDB seed or template
// collection anymore).
var FantasyV1Template = Template{
	ID:      "fantasy_v1",
	Name:    "Fantasy Template",
	Default: true,
	Fields: FieldGroup{
		fieldNameStats: {Children: FieldGroup{
			"dexterity":    {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 10}},
			"constitution": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 10}},
		}},
		"others": {Children: FieldGroup{
			"proficiencyBonus": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 2}},
		}},
		fieldNameModifiers: {Children: FieldGroup{
			fieldNameDexterityMod: {Schema: &FieldSchema{
				Type:       fieldTypeNumber,
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{fieldNameStats, "dexterity"}},
			}},
			"constitutionMod": {Schema: &FieldSchema{
				Type:       fieldTypeNumber,
				IsEditable: false,
				Formula:    "floor((stats_constitution - 10) / 2)",
				DependsOn:  [][]string{{fieldNameStats, "constitution"}},
			}},
		}},
		fieldNameConfig: {Children: FieldGroup{
			"acUsesDex":         {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 1}},
			"acUsesCon":         {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 0}},
			"stealthProfWeight": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 0}},
			"spellUsesInt":      {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 0}},
			"spellUsesWis":      {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 0}},
		}},
		"baseArmorClass": {Schema: &FieldSchema{Type: fieldTypeNumber, IsEditable: true, Default: 10}},
		"derived": {Children: FieldGroup{
			"armorClass": {Schema: &FieldSchema{
				Type:       fieldTypeNumber,
				IsEditable: false,
				Formula:    "baseArmorClass + (modifiers_dexterityMod * config_acUsesDex) + (modifiers_constitutionMod * config_acUsesCon)",
				DependsOn: [][]string{
					{"baseArmorClass"},
					{fieldNameModifiers, fieldNameDexterityMod},
					{fieldNameConfig, "acUsesDex"},
					{fieldNameModifiers, "constitutionMod"},
					{fieldNameConfig, "acUsesCon"},
				},
			}},
			"stealthTotal": {Schema: &FieldSchema{
				Type:       fieldTypeNumber,
				IsEditable: false,
				Formula:    "modifiers_dexterityMod + floor(others_proficiencyBonus * config_stealthProfWeight)",
				DependsOn: [][]string{
					{fieldNameModifiers, fieldNameDexterityMod},
					{"others", "proficiencyBonus"},
					{fieldNameConfig, "stealthProfWeight"},
				},
			}},
		}},
	},
}
