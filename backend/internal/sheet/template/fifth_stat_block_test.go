package template

import (
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFifthStatBlock_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthStatBlockID)

	t.Run("Happy path: the sections follow the sheet reading order", func(t *testing.T) {
		want := []string{
			fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
			fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
			fifthStatBlockExperienceSection, fifthStatBlockHitPointsSection, fifthStatBlockInitiativeSection, fifthStatBlockSpeedSection,
			fifthStatBlockDetailsSection, fifthStatBlockTraitsSection, fifthStatBlockActionsSection, fifthStatBlockBonusActionsSection,
			fifthStatBlockReactionsSection, fifthStatBlockLegendaryActionsSection, fifthStatBlockResourcesSection,
		}
		assert.Equal(t, want, tmpl.SectionOrder)
		assert.ElementsMatch(t, groupKeys(tmpl.Fields), tmpl.SectionOrder)
	})

	t.Run("Happy path: identity enters the challenge rating and derives the proficiency bonus", func(t *testing.T) {
		identity := tmpl.Fields[fifthIdentitySection].Children

		name := identity[fieldNameName].Schema
		require.NotNil(t, name)
		assert.Equal(t, fieldTypeText, name.Type)
		assert.True(t, name.IsEditable)
		assert.True(t, name.Generatable)

		cr := identity[fifthChallengeRatingKey].Schema
		require.NotNil(t, cr)
		assert.Equal(t, fieldTypeNumber, cr.Type)
		assert.True(t, cr.IsEditable)
		assert.True(t, cr.PreferDefault)
		assert.InDelta(t, 0.0, cr.Default, 1e-9)
		require.NotNil(t, cr.Min)
		require.NotNil(t, cr.Max)
		assert.InDelta(t, 0.0, *cr.Min, 1e-9)
		assert.InDelta(t, 30.0, *cr.Max, 1e-9)
		assert.Empty(t, cr.Formula)

		proficiency := identity[fieldNameProficiencyBonus].Schema
		require.NotNil(t, proficiency)
		assert.False(t, proficiency.IsEditable)
		assert.Contains(t, proficiency.Formula, "challengeRating")

		_, hasLevel := identity[fifthLevelKey]
		assert.False(t, hasLevel)
		_, hasJack := identity[fifthJackOfAllTradesKey]
		assert.False(t, hasJack)
	})

	t.Run("Happy path: the armor choice adds natural armor", func(t *testing.T) {
		group := tmpl.Fields[fifthArmorSection].Children

		natural := group[fifthArmorChoiceNaturalKey].Schema
		require.NotNil(t, natural)
		assert.True(t, natural.IsEditable)
		assert.Equal(t, fifthArmorChoiceGroup, natural.ExclusiveGroup)
		assert.InDelta(t, 0.0, natural.Default, 1e-9)

		assert.Contains(t, tmpl.Fields[fifthArmorSection].Order, fifthArmorChoiceNaturalKey)
	})

	t.Run("Happy path: the dice defaults cover the d20 checks and the creature rolls", func(t *testing.T) {
		assert.Equal(t, fifthCheckDice(), tmpl.SectionDice)
		assert.Equal(t, fifthStatBlockRowDice(), tmpl.RowDice)
		assert.Empty(t, tmpl.NotD20Sections)
		assert.Equal(t, fifthStatBlockNotD20Rows(), tmpl.NotD20Rows)
	})
}

func TestFifthStatBlock_SkillsNeverTakeJackOfAllTrades(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthStatBlockID)
	data := fifthSheet(t, tmpl, map[string]any{
		"identity.challengeRating": float64(17),
		"abilities.wisdom.score":   float64(14),
		"skills.insight.rank":      float64(0),
	})

	assert.InDelta(t, 2.0, fifthValue(t, data, "skills.insight.total"), 1e-9)
	assert.InDelta(t, 6.0, fifthValue(t, data, "identity.proficiencyBonus"), 1e-9)
}

func TestFifthStatBlock_Kinds(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthStatBlockID)
	assert.False(t, tmpl.AcceptsKind(plan.KindCharacter))
	assert.True(t, tmpl.AcceptsKind(plan.KindNPC))
	assert.True(t, tmpl.AcceptsKind(plan.KindEnemy))
}
