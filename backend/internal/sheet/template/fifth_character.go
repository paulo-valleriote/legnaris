package template

import (
	"fmt"
	"slices"

	"legnaris/backend/internal/plan"
)

const (
	fifthClassKey      = "class"
	fifthSubclassKey   = "subclass"
	fifthSpeciesKey    = "species"
	fifthBackgroundKey = "background"

	fifthSubclassMinLevel = 3
)

var fifthCharacterPromptGuidance = map[plan.SheetKind]string{
	plan.KindCharacter: `The subject is a player character with one class and its SRD subclass.
Choose the class, subclass, species and background from the SRD when the concept fits them.
An out-of-SRD concept uses the closest SRD option instead.
A multiclass concept becomes its dominant class.
Name the character with an original name.
A character with no stated level is level 1 with 0 experience points.
Write feature text as a short paraphrase of at most about 300 runes.`,
}

var fifthCharacterTemplate = Template{
	ID:                   fifthCharacterID,
	Name:                 "5E Character",
	State:                StateDraft,
	Kinds:                []plan.SheetKind{plan.KindCharacter},
	GameSystemID:         GameSystemFifth,
	PromptGuidanceByKind: fifthCharacterPromptGuidance,
	NameLeaf:             fifthNameLeaf,
	Fields: FieldGroup{
		fifthIdentitySection:     {Children: fifthCharacterIdentityGroup(), Order: fifthCharacterIdentityOrder},
		fifthAbilitiesSection:    {Children: fifthAbilitiesGroup(), Order: fifthAbilitiesOrder()},
		fifthSavesSection:        {Children: fifthSavesGroup(), Order: fifthAbilitiesOrder()},
		fifthSkillsSection:       {Children: fifthSkillsGroup(true), Order: fifthSkillKeys()},
		fifthExhaustionSection:   {Children: fifthExhaustionGroup(), Order: fifthExhaustionOrder},
		fifthPassiveSection:      {Children: fifthPassiveGroup(), Order: fifthPassiveOrder},
		fifthArmorSection:        {Children: fifthArmorGroup(false), Order: fifthArmorOrder(false)},
		fifthHitDiceSection:      {Children: fifthHitDiceGroup(), Order: fifthHitDiceOrder()},
		fifthSpellcastingSection: {Children: fifthSpellcastingGroup(), Order: fifthSpellcastingOrder()},
		fifthCombatSection:       {Children: fifthCharacterCombatGroup(), Order: fifthCombatOrder},
		fifthStatusSection:       {Children: fifthCharacterStatusGroup(), Order: fifthStatusOrder},
		fifthCoinsSection:        {Children: fifthCharacterCoinsGroup(), Order: fifthCoinsOrder},
		fifthAttacksSection:      {Children: fifthAttacksGroup(), Order: fifthAttacksOrder},
		fifthInventorySection:    {Children: fifthInventoryGroup(), Order: fifthInventoryOrder},
		fifthFeaturesSection:     {Children: fifthFeaturesGroup(), Order: fifthFeaturesOrder},
		fifthSpellsSection:       {Children: fifthSpellsGroup(), Order: fifthSpellsOrder},
		fifthResourcesSection:    {Children: fifthResourcesGroup(), Order: fifthResourcesOrder},
	},
	SectionOrder: []string{
		fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
		fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
		fifthHitDiceSection, fifthSpellcastingSection,
		fifthCombatSection, fifthStatusSection, fifthCoinsSection,
		fifthAttacksSection, fifthInventorySection, fifthFeaturesSection,
		fifthSpellsSection, fifthResourcesSection,
	},
	SectionDice:    fifthCheckDice(),
	RowDice:        fifthCharacterRowDice(),
	NotD20Sections: []string{fifthHitDiceSection},
	ClassSubclass: &ClassSubclassRule{
		Class:         []string{fifthIdentitySection, fifthClassKey},
		Subclass:      []string{fifthIdentitySection, fifthSubclassKey},
		Level:         []string{fifthIdentitySection, fifthLevelKey},
		SubclassLevel: fifthSubclassMinLevel,
	},
	Actions: fifthCharacterRestActions(),
}

var fifthCharacterIdentityOrder = []string{
	fieldNameName, fifthClassKey, fifthSubclassKey, fifthSpeciesKey, fifthBackgroundKey,
	fifthLevelKey, fifthJackOfAllTradesKey, fieldNameProficiencyBonus,
}

var fifthExhaustionOrder = []string{fifthExhaustionLevelKey, fifthExhaustionPenaltyKey}

var fifthPassiveOrder = []string{
	fifthPassiveChoiceNoneKey, fifthPassiveChoiceAdvantageKey,
	fifthPassiveChoiceDisadvantageKey, fifthPerceptionKey,
}

func fifthCharacterIdentityGroup() FieldGroup {
	level := preferDefault(generatableNumber(numField("Level", 1), 1, 20))
	jack := numField("Jack of All Trades", 0)
	jack.Schema.Flag = true
	proficiency := calcField(
		"Proficiency bonus",
		fmt.Sprintf("2 + floor((%s - 1) / 4)", Path{fifthIdentitySection, fifthLevelKey}.EngineKey()),
		[]string{fifthIdentitySection, fifthLevelKey},
	)

	class := generatableText(textField("Class"), 60)
	class.Schema.GenerationEnum = slices.Clone(fifthEditionSystemNames.Classes)
	subclass := generatableText(textField("Subclass"), 60)
	subclass.Schema.GenerationEnum = append(slices.Clone(fifthEditionSystemNames.Subclasses), "")
	species := generatableText(textField("Species"), 60)
	species.Schema.GenerationEnum = slices.Clone(fifthEditionSystemNames.Species)
	background := generatableText(textField("Background"), 60)
	background.Schema.GenerationEnum = slices.Clone(fifthEditionSystemNames.Backgrounds)

	return FieldGroup{
		fieldNameName:             generatableText(textField("Name"), 60),
		fifthClassKey:             class,
		fifthSubclassKey:          subclass,
		fifthSpeciesKey:           species,
		fifthBackgroundKey:        background,
		fifthLevelKey:             level,
		fifthJackOfAllTradesKey:   jack,
		fieldNameProficiencyBonus: proficiency,
	}
}
