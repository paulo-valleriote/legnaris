package template

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
			"proficiency_bonus": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 2}},
		}},
		"modifiers": {Children: FieldGroup{
			"dexterity_mod": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
			"constitution_mod": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_constitution - 10) / 2)",
				DependsOn:  [][]string{{"stats", "constitution"}},
			}},
		}},
		"config": {Children: FieldGroup{
			"ac_uses_dex":         {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 1}},
			"ac_uses_con":         {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"stealth_prof_weight": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"spell_uses_int":      {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
			"spell_uses_wis":      {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 0}},
		}},
		"base_armor_class": {Schema: &FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: FieldGroup{
			"armor_class": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "base_armor_class + (modifiers_dexterity_mod * config_ac_uses_dex) + (modifiers_constitution_mod * config_ac_uses_con)",
				DependsOn: [][]string{
					{"base_armor_class"},
					{"modifiers", "dexterity_mod"},
					{"config", "ac_uses_dex"},
					{"modifiers", "constitution_mod"},
					{"config", "ac_uses_con"},
				},
			}},
			"stealth_total": {Schema: &FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "modifiers_dexterity_mod + floor(others_proficiency_bonus * config_stealth_prof_weight)",
				DependsOn: [][]string{
					{"modifiers", "dexterity_mod"},
					{"others", "proficiency_bonus"},
					{"config", "stealth_prof_weight"},
				},
			}},
		}},
	},
}
