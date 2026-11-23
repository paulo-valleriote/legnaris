package template

import (
	"fmt"
	"strings"
)

const (
	fifthCharacterID = "fifth_character_v1"
	fifthStatBlockID = "fifth_stat_block_v1"

	fifthIdentitySection   = "identity"
	fifthAbilitiesSection  = "abilities"
	fifthSavesSection      = "saves"
	fifthSkillsSection     = "skills"
	fifthExhaustionSection = "exhaustion"
	fifthPassiveSection    = "passive"
	fifthArmorSection      = "armor"

	fifthNameLeaf = "identity.name"

	fifthScoreKey = "score"
	fifthModKey   = "mod"
	fifthProfKey  = "prof"
	fifthRankKey  = "rank"
	fifthMiscKey  = "misc"

	fifthTotalColumnLabel = "Total"
	fifthCountColumnLabel = "Count"

	fifthLevelKey           = "level"
	fifthChallengeRatingKey = "challengeRating"

	fifthJackOfAllTradesKey = "jackOfAllTrades"

	fifthExhaustionLevelKey   = "level"
	fifthExhaustionPenaltyKey = "penalty"

	fifthPerceptionKey                = "perception"
	fifthPassiveChoiceNoneKey         = "choiceNone"
	fifthPassiveChoiceAdvantageKey    = "choiceAdvantage"
	fifthPassiveChoiceDisadvantageKey = "choiceDisadvantage"
	fifthPassiveChoiceGroup           = "perceptionChoice"

	fifthArmorBaseKey            = "base"
	fifthArmorShieldKey          = "shield"
	fifthArmorMiscKey            = "misc"
	fifthArmorClassKey           = "armorClass"
	fifthArmorChoiceNoneKey      = "choiceNone"
	fifthArmorChoiceLightKey     = "choiceLight"
	fifthArmorChoiceMediumKey    = "choiceMedium"
	fifthArmorChoiceHeavyKey     = "choiceHeavy"
	fifthArmorChoiceBarbarianKey = "choiceBarbarian"
	fifthArmorChoiceMonkKey      = "choiceMonk"
	fifthArmorChoiceNaturalKey   = "choiceNatural"
	fifthArmorChoiceGroup        = "armorChoice"
)

var fifthSkills = []struct{ key, label, ability string }{
	{"acrobatics", "Acrobatics", abilityDexterity},
	{"animalHandling", "Animal Handling", abilityWisdom},
	{"arcana", "Arcana", abilityIntelligence},
	{"athletics", "Athletics", abilityStrength},
	{"deception", "Deception", abilityCharisma},
	{"history", "History", abilityIntelligence},
	{"insight", "Insight", abilityWisdom},
	{"intimidation", "Intimidation", abilityCharisma},
	{"investigation", "Investigation", abilityIntelligence},
	{"medicine", "Medicine", abilityWisdom},
	{"nature", "Nature", abilityIntelligence},
	{fifthPerceptionKey, "Perception", abilityWisdom},
	{"performance", "Performance", abilityCharisma},
	{"persuasion", "Persuasion", abilityCharisma},
	{"religion", "Religion", abilityIntelligence},
	{"sleightOfHand", "Sleight of Hand", abilityDexterity},
	{"stealth", "Stealth", abilityDexterity},
	{"survival", "Survival", abilityWisdom},
}

func fifthSkillKeys() []string {
	keys := make([]string, 0, len(fifthSkills))
	for _, skill := range fifthSkills {
		keys = append(keys, skill.key)
	}
	return keys
}

func fifthCheckDice() map[string]string {
	return map[string]string{
		fifthAbilitiesSection: "1d20 + @.mod + exhaustion.penalty",
		fifthSavesSection:     "1d20 + @.total + exhaustion.penalty",
		fifthSkillsSection:    "1d20 + @.total + exhaustion.penalty",
	}
}

func fifthAbilityGroup(ability string) FieldGroup {
	scoreKey := Path{fifthAbilitiesSection, ability, fifthScoreKey}.EngineKey()
	score := generatableNumber(numField(upperFirst(ability), 10), 1, 30)
	score.Schema.ColumnLabel = "Score"

	mod := calcField(
		upperFirst(ability)+" modifier",
		fmt.Sprintf("floor((%s - 10) / 2)", scoreKey),
		[]string{fifthAbilitiesSection, ability, fifthScoreKey},
	)
	mod.Schema.ColumnLabel = "Modifier"

	return FieldGroup{
		fifthScoreKey: score,
		fifthModKey:   mod,
	}
}

func fifthAbilitiesGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, ability := range abilities {
		group[ability.key] = FieldNode{
			Children: fifthAbilityGroup(ability.key),
			Columns:  []string{fifthScoreKey, fifthModKey},
			Order:    []string{fifthScoreKey, fifthModKey},
		}
	}
	return group
}

func fifthSaveGroup(ability string) FieldGroup {
	modKey := Path{fifthAbilitiesSection, ability, fifthModKey}.EngineKey()
	profKey := Path{fifthSavesSection, ability, fifthProfKey}.EngineKey()
	miscKey := Path{fifthSavesSection, ability, fifthMiscKey}.EngineKey()
	proficiencyKey := Path{fifthIdentitySection, fieldNameProficiencyBonus}.EngineKey()

	prof := numField(upperFirst(ability)+" save proficiency", 0)
	prof.Schema.Flag = true
	prof.Schema.ColumnLabel = "Proficient"

	misc := generatableNumber(numField(upperFirst(ability)+" save misc", 0), -20, 20)
	misc.Schema.ColumnLabel = "Misc"

	total := calcField(
		upperFirst(ability)+" save total",
		fmt.Sprintf("%s + %s * %s + %s", modKey, proficiencyKey, profKey, miscKey),
		[]string{fifthAbilitiesSection, ability, fifthModKey},
		[]string{fifthIdentitySection, fieldNameProficiencyBonus},
		[]string{fifthSavesSection, ability, fifthProfKey},
		[]string{fifthSavesSection, ability, fifthMiscKey},
	)
	total.Schema.ColumnLabel = fifthTotalColumnLabel

	return FieldGroup{
		fifthProfKey:   prof,
		fifthMiscKey:   misc,
		fieldNameTotal: total,
	}
}

func fifthSavesGroup() FieldGroup {
	group := make(FieldGroup, len(abilities))
	for _, ability := range abilities {
		group[ability.key] = FieldNode{
			Children: fifthSaveGroup(ability.key),
			Columns:  []string{fifthProfKey, fifthMiscKey, fieldNameTotal},
			Order:    []string{fifthProfKey, fifthMiscKey, fieldNameTotal},
		}
	}
	return group
}

func fifthSkillGroup(skill, ability string, withJackOfAllTrades bool) FieldGroup {
	label := fifthSkillLabel(skill)
	modKey := Path{fifthAbilitiesSection, ability, fifthModKey}.EngineKey()
	rankKey := Path{fifthSkillsSection, skill, fifthRankKey}.EngineKey()
	miscKey := Path{fifthSkillsSection, skill, fifthMiscKey}.EngineKey()
	proficiencyKey := Path{fifthIdentitySection, fieldNameProficiencyBonus}.EngineKey()

	rank := generatableNumber(numField(label+" rank", 0), 0, 2)
	rank.Schema.ColumnLabel = "Rank"

	misc := generatableNumber(numField(label+" misc", 0), -20, 20)
	misc.Schema.ColumnLabel = "Misc"

	formula := fmt.Sprintf("%s + %s * %s + %s", modKey, rankKey, proficiencyKey, miscKey)
	dependsOn := [][]string{
		{fifthAbilitiesSection, ability, fifthModKey},
		{fifthSkillsSection, skill, fifthRankKey},
		{fifthIdentitySection, fieldNameProficiencyBonus},
		{fifthSkillsSection, skill, fifthMiscKey},
	}
	if withJackOfAllTrades {
		jackKey := Path{fifthIdentitySection, fifthJackOfAllTradesKey}.EngineKey()
		formula += fmt.Sprintf(" + %s * floor(%s / 2) * (1 - min(%s, 1))",
			jackKey, proficiencyKey, rankKey)
		dependsOn = append(dependsOn, []string{fifthIdentitySection, fifthJackOfAllTradesKey})
	}

	total := calcField(label+" total", formula, dependsOn...)
	total.Schema.ColumnLabel = fifthTotalColumnLabel

	return FieldGroup{
		fifthRankKey:   rank,
		fifthMiscKey:   misc,
		fieldNameTotal: total,
	}
}

func fifthSkillsGroup(withJackOfAllTrades bool) FieldGroup {
	group := make(FieldGroup, len(fifthSkills))
	for _, skill := range fifthSkills {
		group[skill.key] = FieldNode{
			Children: fifthSkillGroup(skill.key, skill.ability, withJackOfAllTrades),
			Columns:  []string{fifthRankKey, fifthMiscKey, fieldNameTotal},
			Order:    []string{fifthRankKey, fifthMiscKey, fieldNameTotal},
		}
	}
	return group
}

func fifthSkillLabel(key string) string {
	for _, skill := range fifthSkills {
		if skill.key == key {
			return skill.label
		}
	}
	return key
}

func fifthExhaustionGroup() FieldGroup {
	levelKey := Path{fifthExhaustionSection, fifthExhaustionLevelKey}.EngineKey()

	level := generatableNumber(numField("Exhaustion level", 0), 0, 6)
	penalty := calcField(
		"Exhaustion penalty",
		fmt.Sprintf("-2 * %s", levelKey),
		[]string{fifthExhaustionSection, fifthExhaustionLevelKey},
	)

	return FieldGroup{
		fifthExhaustionLevelKey:   level,
		fifthExhaustionPenaltyKey: penalty,
	}
}

func fifthPassiveGroup() FieldGroup {
	advantageKey := Path{fifthPassiveSection, fifthPassiveChoiceAdvantageKey}.EngineKey()
	disadvantageKey := Path{fifthPassiveSection, fifthPassiveChoiceDisadvantageKey}.EngineKey()
	perceptionKey := Path{fifthSkillsSection, fifthPerceptionKey, fieldNameTotal}.EngineKey()

	none := fifthChoice("Passive Perception", "None", 1, fifthPassiveChoiceGroup)
	advantage := fifthChoice("Passive Perception with advantage", "+5 (Advantage)", 0, fifthPassiveChoiceGroup)
	disadvantage := fifthChoice("Passive Perception with disadvantage", "-5 (Disadvantage)", 0, fifthPassiveChoiceGroup)

	perception := calcField(
		"Passive Perception",
		fmt.Sprintf("10 + %s + 5 * %s - 5 * %s", perceptionKey, advantageKey, disadvantageKey),
		[]string{fifthSkillsSection, fifthPerceptionKey, fieldNameTotal},
		[]string{fifthPassiveSection, fifthPassiveChoiceAdvantageKey},
		[]string{fifthPassiveSection, fifthPassiveChoiceDisadvantageKey},
	)

	return FieldGroup{
		fifthPassiveChoiceNoneKey:         none,
		fifthPassiveChoiceAdvantageKey:    advantage,
		fifthPassiveChoiceDisadvantageKey: disadvantage,
		fifthPerceptionKey:                perception,
	}
}

func fifthArmorGroup(includeNatural bool) FieldGroup {
	baseKey := Path{fifthArmorSection, fifthArmorBaseKey}.EngineKey()
	shieldKey := Path{fifthArmorSection, fifthArmorShieldKey}.EngineKey()
	miscKey := Path{fifthArmorSection, fifthArmorMiscKey}.EngineKey()
	dexterityKey := Path{fifthAbilitiesSection, abilityDexterity, fifthModKey}.EngineKey()
	constitutionKey := Path{fifthAbilitiesSection, abilityConstitution, fifthModKey}.EngineKey()
	wisdomKey := Path{fifthAbilitiesSection, abilityWisdom, fifthModKey}.EngineKey()
	monkKey := Path{fifthArmorSection, fifthArmorChoiceMonkKey}.EngineKey()

	terms := []string{
		fmt.Sprintf("%s * (10 + %s)", Path{fifthArmorSection, fifthArmorChoiceNoneKey}.EngineKey(), dexterityKey),
		fmt.Sprintf("%s * (%s + %s)", Path{fifthArmorSection, fifthArmorChoiceLightKey}.EngineKey(), baseKey, dexterityKey),
		fmt.Sprintf("%s * (%s + min(%s, 2))", Path{fifthArmorSection, fifthArmorChoiceMediumKey}.EngineKey(), baseKey, dexterityKey),
		fmt.Sprintf("%s * %s", Path{fifthArmorSection, fifthArmorChoiceHeavyKey}.EngineKey(), baseKey),
		fmt.Sprintf("%s * (10 + %s + %s)", Path{fifthArmorSection, fifthArmorChoiceBarbarianKey}.EngineKey(), dexterityKey, constitutionKey),
		fmt.Sprintf("%s * (10 + %s + %s * (1 - %s))", monkKey, dexterityKey, wisdomKey, shieldKey),
	}
	if includeNatural {
		terms = append(terms,
			fmt.Sprintf("%s * (%s + %s)", Path{fifthArmorSection, fifthArmorChoiceNaturalKey}.EngineKey(), baseKey, dexterityKey))
	}

	formula := fmt.Sprintf("(%s) + %s * 2 + %s",
		strings.Join(terms, " + "), shieldKey, miscKey)

	dependsOn := [][]string{
		{fifthArmorSection, fifthArmorChoiceNoneKey},
		{fifthArmorSection, fifthArmorChoiceLightKey},
		{fifthArmorSection, fifthArmorChoiceMediumKey},
		{fifthArmorSection, fifthArmorChoiceHeavyKey},
		{fifthArmorSection, fifthArmorChoiceBarbarianKey},
		{fifthArmorSection, fifthArmorChoiceMonkKey},
		{fifthArmorSection, fifthArmorBaseKey},
		{fifthArmorSection, fifthArmorShieldKey},
		{fifthArmorSection, fifthArmorMiscKey},
		{fifthAbilitiesSection, abilityDexterity, fifthModKey},
		{fifthAbilitiesSection, abilityConstitution, fifthModKey},
		{fifthAbilitiesSection, abilityWisdom, fifthModKey},
	}
	if includeNatural {
		dependsOn = append(dependsOn, []string{fifthArmorSection, fifthArmorChoiceNaturalKey})
	}

	armorClass := calcField("Armor class", formula, dependsOn...)

	base := generatableNumber(numField("Armor class base", 10), 1, 30)
	shield := numField("Shield", 0)
	shield.Schema.Flag = true
	misc := generatableNumber(numField("Armor class misc", 0), -10, 10)

	group := FieldGroup{
		fifthArmorChoiceNoneKey:      fifthChoice("Armor", "None", 1, fifthArmorChoiceGroup),
		fifthArmorChoiceLightKey:     fifthChoice("Armor", "Light armor", 0, fifthArmorChoiceGroup),
		fifthArmorChoiceMediumKey:    fifthChoice("Armor", "Medium armor", 0, fifthArmorChoiceGroup),
		fifthArmorChoiceHeavyKey:     fifthChoice("Armor", "Heavy armor", 0, fifthArmorChoiceGroup),
		fifthArmorChoiceBarbarianKey: fifthChoice("Unarmored defense", "Barbarian", 0, fifthArmorChoiceGroup),
		fifthArmorChoiceMonkKey:      fifthChoice("Unarmored defense", "Monk", 0, fifthArmorChoiceGroup),
		fifthArmorBaseKey:            base,
		fifthArmorShieldKey:          shield,
		fifthArmorMiscKey:            misc,
		fifthArmorClassKey:           armorClass,
	}
	if includeNatural {
		group[fifthArmorChoiceNaturalKey] = fifthChoice("Armor", "Natural armor", 0, fifthArmorChoiceGroup)
	}
	return group
}

func fifthChoice(label, option string, def float64, group string) FieldNode {
	node := numField(label, def)
	node.Schema.ExclusiveGroup = group
	node.Schema.OptionLabel = option
	return node
}

func fifthAbilitiesOrder() []string {
	return abilityKeys(identityKey)
}

func fifthArmorOrder(includeNatural bool) []string {
	order := []string{
		fifthArmorChoiceNoneKey, fifthArmorChoiceLightKey, fifthArmorChoiceMediumKey,
		fifthArmorChoiceHeavyKey, fifthArmorChoiceBarbarianKey, fifthArmorChoiceMonkKey,
	}
	if includeNatural {
		order = append(order, fifthArmorChoiceNaturalKey)
	}
	return append(order, fifthArmorBaseKey, fifthArmorShieldKey, fifthArmorMiscKey, fifthArmorClassKey)
}
