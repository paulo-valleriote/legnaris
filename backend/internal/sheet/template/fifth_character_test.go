package template

import (
	"testing"

	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFifthCharacter_Structure(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)

	t.Run("Happy path: the sections follow the sheet reading order", func(t *testing.T) {
		want := []string{
			fifthIdentitySection, fifthAbilitiesSection, fifthSavesSection,
			fifthSkillsSection, fifthExhaustionSection, fifthPassiveSection, fifthArmorSection,
			fifthHitDiceSection, fifthSpellcastingSection,
			fifthCombatSection, fifthStatusSection, fifthCoinsSection,
			fifthAttacksSection, fifthInventorySection, fifthFeaturesSection,
			fifthSpellsSection, fifthResourcesSection,
		}
		assert.Equal(t, want, tmpl.SectionOrder)
		assert.ElementsMatch(t, groupKeys(tmpl.Fields), tmpl.SectionOrder)
	})

	t.Run("Happy path: identity enters the level and derives the proficiency bonus", func(t *testing.T) {
		identity := tmpl.Fields[fifthIdentitySection].Children

		name := identity[fieldNameName].Schema
		require.NotNil(t, name)
		assert.Equal(t, fieldTypeText, name.Type)
		assert.True(t, name.IsEditable)
		assert.True(t, name.Generatable)
		require.NotNil(t, name.MaxLength)
		assert.Equal(t, 60, *name.MaxLength)

		level := identity[fifthLevelKey].Schema
		require.NotNil(t, level)
		assert.Equal(t, fieldTypeNumber, level.Type)
		assert.True(t, level.IsEditable)
		assert.True(t, level.PreferDefault)
		assert.InDelta(t, 1.0, level.Default, 1e-9)
		require.NotNil(t, level.Min)
		require.NotNil(t, level.Max)
		assert.InDelta(t, 1.0, *level.Min, 1e-9)
		assert.InDelta(t, 20.0, *level.Max, 1e-9)
		assert.Empty(t, level.Formula)

		jack := identity[fifthJackOfAllTradesKey].Schema
		require.NotNil(t, jack)
		assert.True(t, jack.IsEditable)
		assert.True(t, jack.Flag)
		assert.InDelta(t, 0.0, jack.Default, 1e-9)

		proficiency := identity[fieldNameProficiencyBonus].Schema
		require.NotNil(t, proficiency)
		assert.False(t, proficiency.IsEditable)
		assert.NotEmpty(t, proficiency.Formula)
		assert.NotEmpty(t, proficiency.DependsOn)
	})

	t.Run("Happy path: abilities separate the entered score from the derived modifier", func(t *testing.T) {
		group := tmpl.Fields[fifthAbilitiesSection].Children
		require.Len(t, group, len(abilities))
		for _, ability := range abilities {
			node := group[ability.key]
			require.NotNil(t, node.Children, "missing ability %s", ability.key)
			assert.Equal(t, []string{fifthScoreKey, fifthModKey}, node.Columns)

			score := node.Children[fifthScoreKey].Schema
			require.NotNil(t, score)
			assert.Equal(t, fieldTypeNumber, score.Type)
			assert.True(t, score.IsEditable)
			assert.InDelta(t, 10.0, score.Default, 1e-9)

			mod := node.Children[fifthModKey].Schema
			require.NotNil(t, mod)
			assert.False(t, mod.IsEditable)
			assert.NotEmpty(t, mod.Formula)
		}
	})

	t.Run("Happy path: saves enter proficiency and misc behind a derived total", func(t *testing.T) {
		group := tmpl.Fields[fifthSavesSection].Children
		require.Len(t, group, len(abilities))
		for _, ability := range abilities {
			node := group[ability.key]
			require.NotNil(t, node.Children, "missing save %s", ability.key)
			assert.Equal(t, []string{fifthProfKey, fifthMiscKey, fieldNameTotal}, node.Columns)

			prof := node.Children[fifthProfKey].Schema
			require.NotNil(t, prof)
			assert.True(t, prof.IsEditable)
			assert.True(t, prof.Flag)
			assert.InDelta(t, 0.0, prof.Default, 1e-9)

			misc := node.Children[fifthMiscKey].Schema
			require.NotNil(t, misc)
			assert.True(t, misc.IsEditable)

			total := node.Children[fieldNameTotal].Schema
			require.NotNil(t, total)
			assert.False(t, total.IsEditable)
			assert.NotEmpty(t, total.Formula)
		}
	})

	t.Run("Happy path: every skill enters its rank and misc behind a derived total", func(t *testing.T) {
		group := tmpl.Fields[fifthSkillsSection].Children
		require.Len(t, group, len(fifthSkills))
		for _, skill := range fifthSkills {
			node := group[skill.key]
			require.NotNil(t, node.Children, "missing skill %s", skill.key)
			assert.Equal(t, []string{fifthRankKey, fifthMiscKey, fieldNameTotal}, node.Columns)

			rank := node.Children[fifthRankKey].Schema
			require.NotNil(t, rank)
			assert.True(t, rank.IsEditable)
			assert.Equal(t, fieldTypeNumber, rank.Type)
			assert.InDelta(t, 0.0, rank.Default, 1e-9)
			require.NotNil(t, rank.Min)
			require.NotNil(t, rank.Max)
			assert.InDelta(t, 0.0, *rank.Min, 1e-9)
			assert.InDelta(t, 2.0, *rank.Max, 1e-9)

			misc := node.Children[fifthMiscKey].Schema
			require.NotNil(t, misc)
			assert.True(t, misc.IsEditable)

			total := node.Children[fieldNameTotal].Schema
			require.NotNil(t, total)
			assert.False(t, total.IsEditable)
			assert.NotEmpty(t, total.Formula)
		}
	})

	t.Run("Happy path: exhaustion enters a level and derives the penalty", func(t *testing.T) {
		group := tmpl.Fields[fifthExhaustionSection].Children

		level := group[fifthExhaustionLevelKey].Schema
		require.NotNil(t, level)
		assert.True(t, level.IsEditable)
		require.NotNil(t, level.Min)
		require.NotNil(t, level.Max)
		assert.InDelta(t, 0.0, *level.Min, 1e-9)
		assert.InDelta(t, 6.0, *level.Max, 1e-9)

		penalty := group[fifthExhaustionPenaltyKey].Schema
		require.NotNil(t, penalty)
		assert.False(t, penalty.IsEditable)
		assert.Contains(t, penalty.Formula, "-2")
	})

	t.Run("Happy path: the passive choice is one exclusive option with none as default", func(t *testing.T) {
		group := tmpl.Fields[fifthPassiveSection].Children

		none := group[fifthPassiveChoiceNoneKey].Schema
		require.NotNil(t, none)
		assert.True(t, none.IsEditable)
		assert.Equal(t, fifthPassiveChoiceGroup, none.ExclusiveGroup)
		assert.InDelta(t, 1.0, none.Default, 1e-9)

		for _, key := range []string{fifthPassiveChoiceAdvantageKey, fifthPassiveChoiceDisadvantageKey} {
			schema := group[key].Schema
			require.NotNil(t, schema, key)
			assert.Equal(t, fifthPassiveChoiceGroup, schema.ExclusiveGroup)
			assert.InDelta(t, 0.0, schema.Default, 1e-9)
		}

		perception := group[fifthPerceptionKey].Schema
		require.NotNil(t, perception)
		assert.False(t, perception.IsEditable)
		assert.NotEmpty(t, perception.Formula)
	})

	t.Run("Happy path: armor is one exclusive choice with a derived class", func(t *testing.T) {
		group := tmpl.Fields[fifthArmorSection].Children

		for _, key := range []string{
			fifthArmorChoiceNoneKey, fifthArmorChoiceLightKey, fifthArmorChoiceMediumKey,
			fifthArmorChoiceHeavyKey, fifthArmorChoiceBarbarianKey, fifthArmorChoiceMonkKey,
		} {
			schema := group[key].Schema
			require.NotNil(t, schema, key)
			assert.True(t, schema.IsEditable)
			assert.Equal(t, fifthArmorChoiceGroup, schema.ExclusiveGroup)
		}
		assert.InDelta(t, 1.0, group[fifthArmorChoiceNoneKey].Schema.Default, 1e-9)

		base := group[fifthArmorBaseKey].Schema
		require.NotNil(t, base)
		assert.True(t, base.IsEditable)
		assert.InDelta(t, 10.0, base.Default, 1e-9)

		shield := group[fifthArmorShieldKey].Schema
		require.NotNil(t, shield)
		assert.True(t, shield.Flag)
		assert.InDelta(t, 0.0, shield.Default, 1e-9)

		armorClass := group[fifthArmorClassKey].Schema
		require.NotNil(t, armorClass)
		assert.False(t, armorClass.IsEditable)
		assert.NotEmpty(t, armorClass.Formula)
	})

	t.Run("Happy path: the dice defaults cover the d20 checks", func(t *testing.T) {
		assert.Equal(t, fifthCheckDice(), tmpl.SectionDice)
		assert.Equal(t, fifthCharacterRowDice(), tmpl.RowDice)
		assert.Equal(t, []string{fifthHitDiceSection}, tmpl.NotD20Sections)
		assert.Empty(t, tmpl.NotD20Rows)
	})
}

func TestFifthCharacter_JackOfAllTradesSkipsSaves(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	updates := map[string]any{
		"identity.level":           float64(5),
		"identity.jackOfAllTrades": float64(1),
		"abilities.wisdom.score":   float64(14),
		"saves.wisdom.misc":        float64(0),
		"saves.wisdom.prof":        float64(0),
	}

	data := fifthSheet(t, tmpl, updates)
	assert.InDelta(t, 2.0, fifthValue(t, data, "saves.wisdom.total"), 1e-9)
	assert.InDelta(t, 3.0, fifthValue(t, data, "skills.insight.total"), 1e-9)
}

func TestFifthCharacter_Kinds(t *testing.T) {
	tmpl := fifthTemplateByID(t, fifthCharacterID)
	assert.True(t, tmpl.AcceptsKind(plan.KindCharacter))
	assert.False(t, tmpl.AcceptsKind(plan.KindNPC))
	assert.False(t, tmpl.AcceptsKind(plan.KindEnemy))
}
