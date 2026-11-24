package template

import (
	"fmt"

	"legnaris/backend/internal/plan"
)

var fifthStatBlockPromptGuidance = map[plan.SheetKind]string{
	plan.KindNPC: `The subject is a non-player character that the game master controls.
Take the stat block from an SRD non-player character when the concept fits one.
An out-of-SRD concept uses the closest SRD option instead.
A monster outside the SRD becomes an original creature.
Name a creature with an SRD name or an original name.
Set the challenge rating from the concept.
Write trait and action text as a short paraphrase of at most about 300 runes.`,
	plan.KindEnemy: `The subject is an enemy that the game master controls.
Take the stat block from an SRD monster when the concept fits one.
An out-of-SRD concept uses the closest SRD option instead.
A monster outside the SRD becomes an original creature.
Name a creature with an SRD name or an original name.
Set the challenge rating from the concept.
Write trait and action text as a short paraphrase of at most about 300 runes.`,
}

var fifthStatBlockTemplate = Template{
	ID:                   fifthStatBlockID,
	Name:                 "5E Stat Block",
	State:                StateDraft,
	Kinds:                []plan.SheetKind{plan.KindNPC, plan.KindEnemy},
	GameSystemID:         GameSystemFifth,
	PromptGuidanceByKind: fifthStatBlockPromptGuidance,
	NameLeaf:             fifthNameLeaf,
	Fields: FieldGroup{
		fifthIdentitySection:                  {Children: fifthStatBlockIdentityGroup(), Order: fifthStatBlockIdentityOrder},
		fifthAbilitiesSection:                 {Children: fifthAbilitiesGroup(), Order: fifthAbilitiesOrder()},
		fifthSavesSection:                     {Children: fifthSavesGroup(), Order: fifthAbilitiesOrder()},
		fifthSkillsSection:                    {Children: fifthSkillsGroup(false), Order: fifthSkillKeys()},
		fifthExhaustionSection:                {Children: fifthExhaustionGroup(), Order: fifthExhaustionOrder},
		fifthPassiveSection:                   {Children: fifthPassiveGroup(), Order: fifthPassiveOrder},
		fifthArmorSection:                     {Children: fifthArmorGroup(true), Order: fifthArmorOrder(true)},
		fifthStatBlockExperienceSection:       {Children: fifthStatBlockExperienceGroup(), Order: fifthStatBlockExperienceOrder},
		fifthStatBlockHitPointsSection:        {Children: fifthStatBlockHitPointsGroup(), Order: fifthStatBlockHitPointsOrder},
		fifthStatBlockInitiativeSection:       {Children: fifthStatBlockInitiativeGroup(), Order: fifthStatBlockInitiativeOrder},
		fifthStatBlockSpeedSection:            {Children: fifthStatBlockSpeedGroup(), Order: fifthStatBlockSpeedOrder},
		fifthStatBlockDetailsSection:          {Children: fifthStatBlockDetailsGroup(), Order: fifthStatBlockDetailsOrder},
		fifthStatBlockTraitsSection:           {List: fifthStatBlockTraitsList()},
		fifthStatBlockActionsSection:          {List: fifthStatBlockActionsList()},
		fifthStatBlockBonusActionsSection:     {List: fifthStatBlockBonusActionsList()},
		fifthStatBlockReactionsSection:        {List: fifthStatBlockReactionsList()},
		fifthStatBlockLegendaryActionsSection: {List: fifthStatBlockLegendaryActionsList()},
		fifthStatBlockResourcesSection:        {List: fifthStatBlockResourcesList()},
	},
	SectionOrder: []string{
		fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
		fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
		fifthStatBlockExperienceSection, fifthStatBlockHitPointsSection, fifthStatBlockInitiativeSection, fifthStatBlockSpeedSection,
		fifthStatBlockDetailsSection, fifthStatBlockTraitsSection, fifthStatBlockActionsSection, fifthStatBlockBonusActionsSection,
		fifthStatBlockReactionsSection, fifthStatBlockLegendaryActionsSection, fifthStatBlockResourcesSection,
	},
	SectionDice: fifthCheckDice(),
	RowDice:     fifthStatBlockRowDice(),
	NotD20Rows:  fifthStatBlockNotD20Rows(),
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
