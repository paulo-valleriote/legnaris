package template

import (
	"fmt"

	"legnaris/backend/internal/plan"
)

var fifthCharacterTemplate = Template{
	ID:           fifthCharacterID,
	Name:         "5E Character",
	State:        StateDraft,
	Kinds:        []plan.SheetKind{plan.KindCharacter},
	GameSystemID: GameSystemFifth,
	NameLeaf:     fifthNameLeaf,
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
	Actions:        fifthCharacterRestActions(),
}

var fifthCharacterIdentityOrder = []string{
	fieldNameName, fifthLevelKey, fifthJackOfAllTradesKey, fieldNameProficiencyBonus,
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

	return FieldGroup{
		fieldNameName:             generatableText(textField("Name"), 60),
		fifthLevelKey:             level,
		fifthJackOfAllTradesKey:   jack,
		fieldNameProficiencyBonus: proficiency,
	}
}
