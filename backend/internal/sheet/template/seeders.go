package template

import (
	"fmt"
	"strings"

	"legnaris/backend/internal/plan"
)

const (
	fieldTypeNumber = "number"
	fieldTypeText   = "text"
	fieldTypeBool   = "bool"

	fieldNameStats            = "stats"
	fieldNameModifiers        = "modifiers"
	fieldNameDexterityMod     = "dexterityMod"
	fieldNameConfig           = "config"
	fieldNameIdentity         = "identity"
	fieldNameRules            = "rules"
	fieldNameSkills           = "skills"
	fieldNameSaves            = "saves"
	fieldNameAttacks          = "attacks"
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
	fieldNameExpended         = "expended"
	fieldNameRemaining        = "remaining"
	fieldNameName             = "name"
	fieldNameACUsesDex        = "acUsesDex"
	fieldNameACUsesCon        = "acUsesCon"
	fieldNameDamage           = "damage"

	exclusiveGroupArmorClassAbility = "armorClassAbility"

	exclusiveGroupSpellcastingAbility = "spellcastingAbility"

	abilityStrength     = "strength"
	abilityDexterity    = "dexterity"
	abilityConstitution = "constitution"
	abilityIntelligence = "intelligence"
	abilityWisdom       = "wisdom"
	abilityCharisma     = "charisma"
)

var abilities = []struct{ key, label string }{
	{abilityStrength, "Strength"},
	{abilityDexterity, "Dexterity"},
	{abilityConstitution, "Constitution"},
	{abilityIntelligence, "Intelligence"},
	{abilityWisdom, "Wisdom"},
	{abilityCharisma, "Charisma"},
}

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

var defaultTemplates = []Template{
	fantasyV1Template,
}

var fantasyV1Template = Template{
	ID:      "fantasy_v1",
	Name:    "Fantasy Template",
	Default: true,
	Kinds:   []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy},
	Fields: FieldGroup{
		fieldNameIdentity:  {Children: identityGroup(), Order: identityOrder},
		fieldNameStats:     {Children: statsGroup(), Order: abilityKeys(identityKey)},
		fieldNameModifiers: {Children: modifiersGroup(), Order: abilityKeys(modKey)},
		fieldNameSaves:     {Children: savesGroup(), Order: abilityKeys(identityKey)},
		fieldNameSkills:    {Children: skillsGroup(), Order: skillKeys()},
		fieldNameCombat:    {Children: combatGroup(), Order: combatOrder},
		fieldNameAttacks:   {Children: attacksGroup(), Order: numberedKeys("attack", 4)},
		fieldNameSpells:    {Children: spellsGroup(), Order: spellsOrder()},
		"derived":          {Children: derivedGroup(), Order: derivedOrder},
		fieldNameConfig:    {Children: configGroup(), Order: configOrder()},
		fieldNameRules:     {Children: rulesGroup(), Order: rulesOrder},
	},
	SectionOrder: []string{
		fieldNameIdentity, fieldNameCombat, "derived", fieldNameAttacks,
		fieldNameStats, fieldNameModifiers, fieldNameSaves, fieldNameSkills,
		fieldNameSpells, fieldNameRules, fieldNameConfig,
	},
	NameLeaf: "identity.characterName",
	SectionDice: map[string]string{
		fieldNameSkills:  "1d20 + @.total",
		fieldNameSaves:   "1d20 + @",
		fieldNameAttacks: "1d20 + @.bonus",
	},
	RowDice: fantasyV1RowDice(),
}

func fantasyV1RowDice() map[string]string {
	rows := make(map[string]string, len(abilities)+1)
	rows[fieldNameCombat+pathSeparator+"initiative"] = "1d20 + @"
	for _, a := range abilities {
		rows[fieldNameStats+pathSeparator+a.key] = fmt.Sprintf("1d20 + %s.%s", fieldNameModifiers, modKey(a.key))
	}
	return rows
}

var (
	identityOrder = []string{"characterName", "playerName", "ancestry", "origin", "level", "xp", fieldNameProficiencyBonus}
	combatOrder   = []string{fieldNameBaseArmorClass, "initiative", "speed", "hitPoints", "hitPointsMax", "hitPointsTemp"}
	derivedOrder  = []string{"armorClass", "passiveAwareness"}
	rulesOrder    = []string{fieldNameModifierBase, fieldNameModifierStep, fieldNamePassiveBase, fieldNameSpellSaveBase}
)

func identityKey(key string) string { return key }

func abilityKeys(keyOf func(string) string) []string {
	keys := make([]string, 0, len(abilities))
	for _, a := range abilities {
		keys = append(keys, keyOf(a.key))
	}
	return keys
}

func skillKeys() []string {
	keys := make([]string, 0, len(skills))
	for _, s := range skills {
		keys = append(keys, s.key)
	}
	return keys
}

func numberedKeys(prefix string, count int) []string {
	keys := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		keys = append(keys, fmt.Sprintf("%s%d", prefix, i))
	}
	return keys
}

func spellsOrder() []string {
	return append([]string{"attackBonus", "saveDC", "atWill", "list"}, numberedKeys("slot", 9)...)
}

func configOrder() []string {
	keys := []string{fieldNameACUsesDex, fieldNameACUsesCon}
	keys = append(keys, abilityKeys(saveProfKey)...)
	keys = append(keys, abilityKeys(spellUsesKey)...)
	for _, s := range skills {
		keys = append(keys, skillProfKey(s.key))
	}
	return keys
}

func numField(label string, def float64) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeNumber,
		Label:      label,
		IsEditable: true,
		Default:    def,
	}}
}

func textField(label string) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeText,
		Label:      label,
		IsEditable: true,
	}}
}

func generatableNumber(node FieldNode, lo, hi float64) FieldNode {
	node.Schema.Generatable = true
	node.Schema.Min = &lo
	node.Schema.Max = &hi
	return node
}

func generatableText(node FieldNode, maxLength int) FieldNode {
	node.Schema.Generatable = true
	node.Schema.MaxLength = &maxLength
	return node
}

func preferDefault(node FieldNode) FieldNode {
	node.Schema.PreferDefault = true
	return node
}

func calcField(label, formula string, deps ...[]string) FieldNode {
	return FieldNode{Schema: &FieldSchema{
		Type:       fieldTypeNumber,
		Label:      label,
		Formula:    formula,
		DependsOn:  deps,
		IsEditable: false,
	}}
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func modKey(ability string) string { return ability + "Mod" }

func saveProfKey(ability string) string { return "save" + upperFirst(ability) + "Prof" }

func skillProfKey(skill string) string { return "skill" + upperFirst(skill) + "Prof" }

func spellUsesKey(ability string) string { return "spellUses" + upperFirst(ability) }

func identityGroup() FieldGroup {
	return FieldGroup{
		"characterName":    generatableText(textField("Character name"), 60),
		"playerName":       textField("Player name"),
		"ancestry":         generatableText(textField("Ancestry"), 40),
		"origin":           generatableText(textField("Origin"), 40),
		"level":            preferDefault(generatableNumber(numField("Level", 1), 1, 20)),
		"xp":               preferDefault(generatableNumber(numField("Experience points", 0), 0, 355000)),
		"proficiencyBonus": preferDefault(generatableNumber(numField("Proficiency bonus", 2), 2, 6)),
	}
}

func statsGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, a := range abilities {
		group[a.key] = generatableNumber(numField(a.label, 10), 3, 20)
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
		bonus := generatableNumber(numField(s.label+" bonus", 0), -10, 20)
		bonus.Schema.ColumnLabel = "Bonus"
		total := calcField(
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
		)
		total.Schema.ColumnLabel = "Total"
		group[s.key] = FieldNode{
			Children: FieldGroup{
				fieldNameBonus: bonus,
				fieldNameTotal: total,
			},
			Columns: []string{fieldNameBonus, fieldNameTotal},
			Order:   []string{fieldNameBonus, fieldNameTotal},
		}
	}
	return group
}

func combatGroup() FieldGroup {
	return FieldGroup{
		"baseArmorClass": generatableNumber(numField("Base armor class", 10), 1, 30),
		"initiative":     generatableNumber(numField("Initiative", 0), -5, 20),
		"speed":          generatableNumber(numField("Speed", 30), 0, 120),
		"hitPointsMax":   generatableNumber(numField("Hit points (maximum)", 10), 1, 500),
		"hitPoints":      generatableNumber(numField("Current hit points", 10), 0, 500),
		"hitPointsTemp":  generatableNumber(numField("Temporary hit points", 0), 0, 100),
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
			[]string{fieldNameConfig, fieldNameACUsesDex},
			[]string{fieldNameModifiers, "constitutionMod"},
			[]string{fieldNameConfig, fieldNameACUsesCon},
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
		name := generatableText(textField(fmt.Sprintf("Attack %d name", i)), 40)
		name.Schema.ColumnLabel = "Name"
		bonus := generatableNumber(numField(fmt.Sprintf("Attack %d bonus", i), 0), -5, 20)
		bonus.Schema.ColumnLabel = "Bonus"
		damage := generatableText(textField(fmt.Sprintf("Attack %d damage", i)), 60)
		damage.Schema.ColumnLabel = "Damage"
		group[fmt.Sprintf("attack%d", i)] = FieldNode{
			Children: FieldGroup{
				fieldNameName:   name,
				fieldNameBonus:  bonus,
				fieldNameDamage: damage,
			},
			Label:   fmt.Sprintf("Attack %d", i),
			Columns: []string{fieldNameName, fieldNameDamage, fieldNameBonus},
			Order:   []string{fieldNameName, fieldNameDamage, fieldNameBonus},
		}
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
		"atWill": generatableText(textField("At-will spells"), 200),
		"list":   generatableText(textField("Spell list"), 2000),
	}
	for level := 1; level <= 9; level++ {
		key := fmt.Sprintf("slot%d", level)
		total := generatableNumber(numField(fmt.Sprintf("Level %d total", level), 0), 0, 10)
		total.Schema.ColumnLabel = "Total"
		expended := generatableNumber(numField(fmt.Sprintf("Level %d expended", level), 0), 0, 10)
		expended.Schema.ColumnLabel = "Expended"
		remaining := calcField(
			fmt.Sprintf("Level %d remaining", level),
			fmt.Sprintf("%s_%s_total - %s_%s_expended", fieldNameSpells, key, fieldNameSpells, key),
			[]string{fieldNameSpells, key, fieldNameTotal},
			[]string{fieldNameSpells, key, fieldNameExpended},
		)
		remaining.Schema.ColumnLabel = "Remaining"
		group[key] = FieldNode{
			Children: FieldGroup{
				fieldNameTotal:     total,
				fieldNameExpended:  expended,
				fieldNameRemaining: remaining,
			},
			Label:   fmt.Sprintf("Level %d", level),
			Columns: []string{fieldNameTotal, fieldNameExpended, fieldNameRemaining},
			Order:   []string{fieldNameTotal, fieldNameExpended, fieldNameRemaining},
		}
	}
	return group
}

func configGroup() FieldGroup {
	acUsesDex := generatableNumber(numField("Armor class uses dexterity", 1), 0, 1)
	acUsesDex.Schema.ExclusiveGroup = exclusiveGroupArmorClassAbility
	acUsesDex.Schema.OptionLabel = "Dexterity"
	acUsesCon := generatableNumber(numField("Armor class uses constitution", 0), 0, 1)
	acUsesCon.Schema.ExclusiveGroup = exclusiveGroupArmorClassAbility
	acUsesCon.Schema.OptionLabel = "Constitution"

	group := FieldGroup{
		fieldNameACUsesDex: acUsesDex,
		fieldNameACUsesCon: acUsesCon,
	}
	for _, a := range abilities {
		group[saveProfKey(a.key)] = generatableNumber(numField(a.label+" save proficiency", 0), 0, 1)
		spellUses := generatableNumber(numField("Spellcasting uses "+strings.ToLower(a.label), 0), 0, 1)
		spellUses.Schema.ExclusiveGroup = exclusiveGroupSpellcastingAbility
		spellUses.Schema.OptionLabel = a.label
		group[spellUsesKey(a.key)] = spellUses
	}
	for _, s := range skills {
		group[skillProfKey(s.key)] = generatableNumber(numField(s.label+" proficiency", 0), 0, 1)
	}
	return group
}

func rulesGroup() FieldGroup {
	return FieldGroup{
		"modifierBase":  numField("Ability modifier base", 10),
		"modifierStep":  numField("Ability modifier step", 2),
		"passiveBase":   numField("Passive score base", 10),
		"spellSaveBase": numField("Spell save DC base", 8),
	}
}

func DefaultTemplates() []Template {
	result := make([]Template, len(defaultTemplates))
	for i, tmpl := range defaultTemplates {
		result[i] = cloneTemplate(tmpl)
	}
	return result
}
