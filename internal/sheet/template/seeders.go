package template

import (
	"fmt"
	"strings"
)

const (
	fieldTypeNumber = "number"
	fieldTypeText   = "text"
	fieldTypeBool   = "bool"

	// Group and leaf keys referenced by generated formulas or DependsOn
	// paths. Keys used only as section names stay inline literals in the
	// field tree below.
	fieldNameStats            = "stats"
	fieldNameModifiers        = "modifiers"
	fieldNameDexterityMod     = "dexterityMod"
	fieldNameConfig           = "config"
	fieldNameIdentity         = "identity"
	fieldNameRules            = "rules"
	fieldNameSkills           = "skills"
	fieldNameCombat           = "combat"
	fieldNameSpells           = "spells"
	fieldNameProficiencyBonus = "proficiencyBonus"
	fieldNameModifierBase     = "modifierBase"
	fieldNameModifierStep     = "modifierStep"
	fieldNamePassiveBase      = "passiveBase"
	fieldNameSpellSaveBase    = "spellSaveBase"
	fieldNameBaseArmorClass   = "baseArmorClass"
	fieldNameSpellAttackBonus = "attackBonus"
	fieldNameTotal            = "total"
	fieldNameBonus            = "bonus"

	// exclusiveGroupArmorClassAbility is the shared group id for the armor-class
	// ability proxies: config.acUsesDex and config.acUsesCon are mutually
	// exclusive (at most one is non-zero), so the API advertises them as one
	// choice and the DAG normalizes them together.
	exclusiveGroupArmorClassAbility = "armorClassAbility"

	// exclusiveGroupSpellcastingAbility is the shared group id for the six
	// config.spellUses<Ability> proxies: spellcasting uses exactly one ability
	// (at most one flag is non-zero), so the API advertises them as one choice
	// and the DAG normalizes them together.
	exclusiveGroupSpellcastingAbility = "spellcastingAbility"

	// Ability keys shared by stats, modifiers, saves and the skill list.
	abilityStrength     = "strength"
	abilityDexterity    = "dexterity"
	abilityConstitution = "constitution"
	abilityIntelligence = "intelligence"
	abilityWisdom       = "wisdom"
	abilityCharisma     = "charisma"
)

// abilities is the set of base scores shared by stats, modifiers, saves and
// the spellcasting formulas; every generated key and label comes from here.
var abilities = []struct{ key, label string }{
	{abilityStrength, "Strength"},
	{abilityDexterity, "Dexterity"},
	{abilityConstitution, "Constitution"},
	{abilityIntelligence, "Intelligence"},
	{abilityWisdom, "Wisdom"},
	{abilityCharisma, "Charisma"},
}

// skills is the generic skill catalog: each entry names the ability its
// total formula reads. Names are plain RPG vocabulary, deliberately free of
// any particular system's skill list.
var skills = []struct{ key, label, ability string }{
	{"athletics", "Athletics", abilityStrength},
	{"acrobatics", "Acrobatics", abilityDexterity},
	{"stealth", "Stealth", abilityDexterity},
	{"perception", "Perception", abilityWisdom},
	{"insight", "Insight", abilityWisdom},
	{"survival", "Survival", abilityWisdom},
	{"persuasion", "Persuasion", abilityCharisma},
	{"deception", "Deception", abilityCharisma},
	{"intimidation", "Intimidation", abilityCharisma},
	{"knowledge", "Knowledge", abilityIntelligence},
	{"analysis", "Analysis", abilityIntelligence},
	{"craft", "Craft", abilityIntelligence},
}

// defaultTemplates is the in-memory catalog of code-defined default templates.
// Each entry is validated and compiled into a DAG once at boot (see Registry).
// Templates are immutable by ID: rule changes ship as new versioned IDs
// (e.g. fantasy_v2), never by mutating an existing entry.
// This variable is unexported; callers use DefaultTemplates() to get a copy.
var defaultTemplates = []Template{
	fantasyV1Template,
}

// fantasyV1Template defines the fantasy_v1 default template. It lives in code only
// and is served from the in-memory Registry (no MongoDB seed or template
// collection anymore).
// The tree is a complete, system-agnostic fantasy character sheet: identity
// (four free-text fields plus level, XP and proficiency bonus), the six base
// scores with modifiers, saves and a generic skill list, combat numbers,
// attacks and spellcasting. Mechanics are calibrated through the `rules`
// group (defaults reproduce the classic score-10/step-2 curve) and wired
// through `config` 0/1 flags, so another system retunes the sheet without
// new formulas. SectionOrder is the display order the API must emit: the
// sheet reads on top, the tuning machinery (rules/config) at the bottom.
var fantasyV1Template = Template{
	ID:      "fantasy_v1",
	Name:    "Fantasy Template",
	Default: true,
	Fields: FieldGroup{
		fieldNameIdentity:  {Children: identityGroup()},
		fieldNameStats:     {Children: statsGroup()},
		fieldNameModifiers: {Children: modifiersGroup()},
		"saves":            {Children: savesGroup()},
		fieldNameSkills:    {Children: skillsGroup()},
		fieldNameCombat:    {Children: combatGroup()},
		"attacks":          {Children: attacksGroup()},
		fieldNameSpells:    {Children: spellsGroup()},
		"derived":          {Children: derivedGroup()},
		fieldNameConfig:    {Children: configGroup()},
		fieldNameRules:     {Children: rulesGroup()},
	},
	SectionOrder: []string{
		fieldNameIdentity, fieldNameStats, fieldNameModifiers, "saves",
		fieldNameSkills, fieldNameCombat, "attacks", fieldNameSpells,
		"derived", fieldNameRules, fieldNameConfig,
	},
}

// numField builds an editable numeric leaf.
func numField(label string, def float64) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeNumber,
		Label:      label,
		IsEditable: true,
		Default:    def,
	}}
}

// textField builds an editable text leaf. Its sheet value is seeded as an
// empty string by SeedValue: Default stays a float64 because the API
// contract emits a numeric `default` (the desktop client decodes it as num).
func textField(label string) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeText,
		Label:      label,
		IsEditable: true,
	}}
}

// calcField builds a derived (computed, non-editable) numeric leaf.
// deps is the exact set of leaves the formula reads: the DAG builds the
// cascade from it, and seeders_test pins it against the formula.
func calcField(label, formula string, deps ...[]string) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeNumber,
		Label:      label,
		Formula:    formula,
		DependsOn:  deps,
		IsEditable: false,
	}}
}

// upperFirst uppercases the first byte of an ASCII key segment, turning a
// field key into the CamelCase suffix of a generated key (strength → Strength).
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// modKey is the modifier leaf key of an ability (strength → strengthMod).
func modKey(ability string) string { return ability + "Mod" }

// saveProfKey is the config flag arming a save proficiency.
func saveProfKey(ability string) string { return "save" + upperFirst(ability) + "Prof" }

// skillProfKey is the config flag arming a skill proficiency.
func skillProfKey(skill string) string { return "skill" + upperFirst(skill) + "Prof" }

// spellUsesKey is the config flag pointing spellcasting at an ability.
func spellUsesKey(ability string) string { return "spellUses" + upperFirst(ability) }

func identityGroup() FieldGroup {
	return FieldGroup{
		"characterName":    textField("Character name"),
		"playerName":       textField("Player name"),
		"ancestry":         textField("Ancestry"),
		"origin":           textField("Origin"),
		"level":            numField("Level", 1),
		"xp":               numField("Experience points", 0),
		"proficiencyBonus": numField("Proficiency bonus", 2),
	}
}

func statsGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, a := range abilities {
		group[a.key] = numField(a.label, 10)
	}
	return group
}

func modifiersGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, a := range abilities {
		group[modKey(a.key)] = calcField(
			a.label+" modifier",
			fmt.Sprintf("floor((%s_%s - %s_%s) / %s_%s)",
				fieldNameStats, a.key,
				fieldNameRules, fieldNameModifierBase,
				fieldNameRules, fieldNameModifierStep),
			[]string{fieldNameStats, a.key},
			[]string{fieldNameRules, fieldNameModifierBase},
			[]string{fieldNameRules, fieldNameModifierStep},
		)
	}
	return group
}

func savesGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, a := range abilities {
		group[a.key] = calcField(
			a.label+" save",
			fmt.Sprintf("%s_%s + floor(%s_%s * %s_%s)",
				fieldNameModifiers, modKey(a.key),
				fieldNameIdentity, fieldNameProficiencyBonus,
				fieldNameConfig, saveProfKey(a.key)),
			[]string{fieldNameModifiers, modKey(a.key)},
			[]string{fieldNameIdentity, fieldNameProficiencyBonus},
			[]string{fieldNameConfig, saveProfKey(a.key)},
		)
	}
	return group
}

func skillsGroup() FieldGroup {
	group := make(FieldGroup, len(skills))
	for _, s := range skills {
		group[s.key] = FieldNode{Children: FieldGroup{
			fieldNameBonus: numField(s.label+" bonus", 0),
			fieldNameTotal: calcField(
				s.label+" total",
				fmt.Sprintf("%s_%sMod + floor(%s_%s * %s_%s) + %s_%s_bonus",
					fieldNameModifiers, s.ability,
					fieldNameIdentity, fieldNameProficiencyBonus,
					fieldNameConfig, skillProfKey(s.key),
					fieldNameSkills, s.key),
				[]string{fieldNameModifiers, modKey(s.ability)},
				[]string{fieldNameIdentity, fieldNameProficiencyBonus},
				[]string{fieldNameConfig, skillProfKey(s.key)},
				[]string{fieldNameSkills, s.key, fieldNameBonus},
			),
		}}
	}
	return group
}

func combatGroup() FieldGroup {
	return FieldGroup{
		"baseArmorClass": numField("Base armor class", 10),
		"initiative":     numField("Initiative", 0),
		"speed":          numField("Speed", 30),
		"hitPointsMax":   numField("Hit points (maximum)", 10),
		"hitPoints":      numField("Current hit points", 10),
		"hitPointsTemp":  numField("Temporary hit points", 0),
	}
}

func derivedGroup() FieldGroup {
	return FieldGroup{
		"armorClass": calcField(
			"Armor class",
			fmt.Sprintf("%s_%s + (%s_%s * %s_acUsesDex) + (%s_constitutionMod * %s_acUsesCon)",
				fieldNameCombat, fieldNameBaseArmorClass,
				fieldNameModifiers, fieldNameDexterityMod, fieldNameConfig,
				fieldNameModifiers, fieldNameConfig),
			[]string{fieldNameCombat, fieldNameBaseArmorClass},
			[]string{fieldNameModifiers, fieldNameDexterityMod},
			[]string{fieldNameConfig, "acUsesDex"},
			[]string{fieldNameModifiers, "constitutionMod"},
			[]string{fieldNameConfig, "acUsesCon"},
		),
		"passiveAwareness": calcField(
			"Passive awareness",
			fmt.Sprintf("%s_%s + %s_perception_total",
				fieldNameRules, fieldNamePassiveBase,
				fieldNameSkills),
			[]string{fieldNameRules, fieldNamePassiveBase},
			[]string{fieldNameSkills, "perception", "total"},
		),
	}
}

func attacksGroup() FieldGroup {
	group := FieldGroup{}
	for i := 1; i <= 4; i++ {
		group[fmt.Sprintf("attack%d", i)] = FieldNode{Children: FieldGroup{
			"name":         textField(fmt.Sprintf("Attack %d name", i)),
			fieldNameBonus: numField(fmt.Sprintf("Attack %d bonus", i), 0),
			"damage":       textField(fmt.Sprintf("Attack %d damage", i)),
		}}
	}
	return group
}

func spellsGroup() FieldGroup {
	attackBonusFormula := fieldNameIdentity + "_" + fieldNameProficiencyBonus
	attackBonusDeps := [][]string{{fieldNameIdentity, fieldNameProficiencyBonus}}
	for _, a := range abilities {
		attackBonusFormula += fmt.Sprintf(" + %s_%s * %s_%s",
			fieldNameModifiers, modKey(a.key),
			fieldNameConfig, spellUsesKey(a.key))
		attackBonusDeps = append(attackBonusDeps,
			[]string{fieldNameModifiers, modKey(a.key)},
			[]string{fieldNameConfig, spellUsesKey(a.key)},
		)
	}

	group := FieldGroup{
		"attackBonus": calcField("Spell attack bonus", attackBonusFormula, attackBonusDeps...),
		"saveDC": calcField(
			"Spell save DC",
			fmt.Sprintf("%s_%s + %s_%s",
				fieldNameRules, fieldNameSpellSaveBase,
				fieldNameSpells, fieldNameSpellAttackBonus),
			[]string{fieldNameRules, fieldNameSpellSaveBase},
			[]string{fieldNameSpells, fieldNameSpellAttackBonus},
		),
		"atWill": textField("At-will spells"),
		"list":   textField("Spell list"),
	}
	for level := 1; level <= 9; level++ {
		key := fmt.Sprintf("slot%d", level)
		group[key] = FieldNode{Children: FieldGroup{
			fieldNameTotal: numField(fmt.Sprintf("Level %d total", level), 0),
			"expended":     numField(fmt.Sprintf("Level %d expended", level), 0),
			"remaining": calcField(
				fmt.Sprintf("Level %d remaining", level),
				fmt.Sprintf("%s_%s_total - %s_%s_expended", fieldNameSpells, key, fieldNameSpells, key),
				[]string{fieldNameSpells, key, fieldNameTotal},
				[]string{fieldNameSpells, key, "expended"},
			),
		}}
	}
	return group
}

func configGroup() FieldGroup {
	acUsesDex := numField("Armor class uses dexterity", 1)
	acUsesDex.Schema.ExclusiveGroup = exclusiveGroupArmorClassAbility
	acUsesCon := numField("Armor class uses constitution", 0)
	acUsesCon.Schema.ExclusiveGroup = exclusiveGroupArmorClassAbility

	group := FieldGroup{
		"acUsesDex": acUsesDex,
		"acUsesCon": acUsesCon,
	}
	for _, a := range abilities {
		group[saveProfKey(a.key)] = numField(a.label+" save proficiency", 0)
		spellUses := numField("Spellcasting uses "+strings.ToLower(a.label), 0)
		spellUses.Schema.ExclusiveGroup = exclusiveGroupSpellcastingAbility
		group[spellUsesKey(a.key)] = spellUses
	}
	for _, s := range skills {
		group[skillProfKey(s.key)] = numField(s.label+" proficiency", 0)
	}
	return group
}

// rulesGroup holds the tuning numbers every generated mechanic reads.
// The defaults reproduce the classic D&D-like curve (modifier = (score-10)/2,
// passive base 10, spell save base 8); another system changes these values
// instead of the formulas.
func rulesGroup() FieldGroup {
	return FieldGroup{
		"modifierBase":  numField("Ability modifier base", 10),
		"modifierStep":  numField("Ability modifier step", 2),
		"passiveBase":   numField("Passive score base", 10),
		"spellSaveBase": numField("Spell save DC base", 8),
	}
}

// DefaultTemplates returns a deep copy of the default template catalog.
// Callers can mutate the returned slice without affecting the package state.
func DefaultTemplates() []Template {
	result := make([]Template, len(defaultTemplates))
	for i, tmpl := range defaultTemplates {
		result[i] = cloneTemplate(tmpl)
	}
	return result
}
