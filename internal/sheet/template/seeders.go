package template

var FantasyV1Seed = map[string]FieldSchema{
	// ---------------------------------------------------------
	// 1. Base Attributes And Proficiency
	// ---------------------------------------------------------
	"stats.dexterity": {
		Type: "number", IsEditable: true, Default: 10,
	},
	"stats.constitution": {
		Type: "number", IsEditable: true, Default: 10,
	},
	"others.proficiency_bonus": {
		Type: "number", IsEditable: true, Default: 2,
	},

	// ---------------------------------------------------------
	// 2. Modifiers
	// ---------------------------------------------------------
	"modifiers.dexterity_mod": {
		Type: "number", IsEditable: false,
		Formula:   "floor((stats.dexterity - 10) / 2)",
		DependsOn: []string{"stats.dexterity"},
	},
	"modifiers.constitution_mod": {
		Type: "number", IsEditable: false,
		Formula:   "floor((stats.constitution - 10) / 2)",
		DependsOn: []string{"stats.constitution"},
	},

	// ---------------------------------------------------------
	// 3. PROXY NODES (Logical Keys)
	// ---------------------------------------------------------

	"config.ac_uses_dex": {
		Type: "number", IsEditable: true, Default: 0,
	},
	"config.ac_uses_con": {
		Type: "number", IsEditable: true, Default: 0,
	},
	"config.stealth_prof_weight": {
		Type: "number", IsEditable: true, Default: 0,
	},
	"config.spell_uses_int": {Type: "number", IsEditable: true, Default: 0},
	"config.spell_uses_wis": {Type: "number", IsEditable: true, Default: 0},

	// ---------------------------------------------------------
	// 4. Deriveds
	// ---------------------------------------------------------
	"base_armor_class": {
		Type: "number", IsEditable: true, Default: 10,
	},
	"derived.armor_class": {
		Type:       "number",
		IsEditable: false,
		Formula:    "base_armor_class + (modifiers.dexterity_mod * config.ac_uses_dex) + (modifiers.constitution_mod * config.ac_uses_con)",
		DependsOn: []string{
			"base_armor_class",
			"modifiers.dexterity_mod",
			"config.ac_uses_dex",
			"modifiers.constitution_mod",
			"config.ac_uses_con",
		},
	},
	"skills.stealth_total": {
		Type:       "number",
		IsEditable: false,
		Formula:    "modifiers.dexterity_mod + floor(others.proficiency_bonus * config.stealth_prof_weight)",
		DependsOn: []string{
			"modifiers.dexterity_mod",
			"others.proficiency_bonus",
			"config.stealth_prof_weight",
		},
	},
}
