package template

import (
	"fmt"

	"legnaris/backend/internal/plan"
)

var fifthStatBlockTemplate = Template{
	ID:           fifthStatBlockID,
	Name:         "5E Stat Block",
	State:        StateDraft,
	Kinds:        []plan.SheetKind{plan.KindNPC, plan.KindEnemy},
	GameSystemID: GameSystemFifth,
	NameLeaf:     fifthNameLeaf,
	Fields: FieldGroup{
		fifthIdentitySection:   {Children: fifthStatBlockIdentityGroup(), Order: fifthStatBlockIdentityOrder},
		fifthAbilitiesSection:  {Children: fifthAbilitiesGroup(), Order: fifthAbilitiesOrder()},
		fifthSavesSection:      {Children: fifthSavesGroup(), Order: fifthAbilitiesOrder()},
		fifthSkillsSection:     {Children: fifthSkillsGroup(false), Order: fifthSkillKeys()},
		fifthExhaustionSection: {Children: fifthExhaustionGroup(), Order: fifthExhaustionOrder},
		fifthPassiveSection:    {Children: fifthPassiveGroup(), Order: fifthPassiveOrder},
		fifthArmorSection:      {Children: fifthArmorGroup(true), Order: fifthArmorOrder(true)},
	},
	SectionOrder: []string{
		fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
		fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
	},
	SectionDice: fifthCheckDice(),
}

var fifthStatBlockIdentityOrder = []string{
	fieldNameName, fifthChallengeRatingKey, fieldNameProficiencyBonus,
}

func fifthStatBlockIdentityGroup() FieldGroup {
	challengeRating := preferDefault(generatableNumber(numField("Challenge rating", 0), 0, 30))
	proficiency := calcField(
		"Proficiency bonus",
		fmt.Sprintf("2 + floor((max(%s, 1) - 1) / 4)", Path{fifthIdentitySection, fifthChallengeRatingKey}.EngineKey()),
		[]string{fifthIdentitySection, fifthChallengeRatingKey},
	)

	return FieldGroup{
		fieldNameName:             generatableText(textField("Name"), 60),
		fifthChallengeRatingKey:   challengeRating,
		fieldNameProficiencyBonus: proficiency,
	}
}
