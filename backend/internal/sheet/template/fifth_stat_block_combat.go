package template

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	fifthStatBlockExperienceSection = "experience"
	fifthStatBlockHitPointsSection  = "hitPoints"
	fifthStatBlockInitiativeSection = "initiative"
	fifthStatBlockSpeedSection      = "speed"

	fifthStatBlockExperienceZeroKey   = "zero"
	fifthStatBlockExperienceInLairKey = "inLair"
	fifthStatBlockExperienceXPKey     = "xp"

	fifthStatBlockHitPointsCountKey    = "count"
	fifthStatBlockHitPointsSizeKey     = "size"
	fifthStatBlockHitPointsConBonusKey = "conBonus"
	fifthStatBlockHitPointsMaximumKey  = "maximum"
	fifthStatBlockHitPointsCurrentKey  = "current"
	fifthStatBlockHitPointsTempKey     = "temp"

	fifthStatBlockInitiativeModifierKey = "modifier"
	fifthStatBlockInitiativeScoreKey    = "score"

	fifthStatBlockColumnConstitution = "Constitution"
)

type fifthStatBlockXPEntry struct {
	CR float64
	XP float64
}

var fifthStatBlockXPTable = []fifthStatBlockXPEntry{
	{CR: 0, XP: 10},
	{CR: 0.125, XP: 25},
	{CR: 0.25, XP: 50},
	{CR: 0.5, XP: 100},
	{CR: 1, XP: 200},
	{CR: 2, XP: 450},
	{CR: 3, XP: 700},
	{CR: 4, XP: 1100},
	{CR: 5, XP: 1800},
	{CR: 6, XP: 2300},
	{CR: 7, XP: 2900},
	{CR: 8, XP: 3900},
	{CR: 9, XP: 5000},
	{CR: 10, XP: 5900},
	{CR: 11, XP: 7200},
	{CR: 12, XP: 8400},
	{CR: 13, XP: 10000},
	{CR: 14, XP: 11500},
	{CR: 15, XP: 13000},
	{CR: 16, XP: 15000},
	{CR: 17, XP: 18000},
	{CR: 18, XP: 20000},
	{CR: 19, XP: 22000},
	{CR: 20, XP: 25000},
	{CR: 21, XP: 33000},
	{CR: 22, XP: 41000},
	{CR: 23, XP: 50000},
	{CR: 24, XP: 62000},
	{CR: 25, XP: 75000},
	{CR: 26, XP: 90000},
	{CR: 27, XP: 105000},
	{CR: 28, XP: 120000},
	{CR: 29, XP: 135000},
	{CR: 30, XP: 155000},
}

var fifthStatBlockSpeedNames = []string{"walk", "fly", "swim", "climb", "burrow"}

var (
	fifthStatBlockExperienceOrder = []string{fifthStatBlockExperienceXPKey, fifthStatBlockExperienceZeroKey, fifthStatBlockExperienceInLairKey}
	fifthStatBlockHitPointsOrder  = []string{
		fifthStatBlockHitPointsCountKey, fifthStatBlockHitPointsSizeKey, fifthStatBlockHitPointsConBonusKey,
		fifthStatBlockHitPointsMaximumKey, fifthStatBlockHitPointsCurrentKey, fifthStatBlockHitPointsTempKey,
	}
	fifthStatBlockInitiativeOrder = []string{fifthStatBlockInitiativeModifierKey, fifthStatBlockInitiativeScoreKey}
	fifthStatBlockSpeedOrder      = fifthStatBlockSpeedKeys()
)

func fifthStatBlockSpeedKeys() []string {
	keys := make([]string, 0, len(fifthStatBlockSpeedNames)*2)
	for _, name := range fifthStatBlockSpeedNames {
		keys = append(keys, name, fifthStatBlockSpeedTotalKey(name))
	}
	return keys
}

func fifthStatBlockSpeedTotalKey(name string) string {
	return name + "Total"
}

func fifthStatBlockExperienceGroup() FieldGroup {
	crKey := Path{fifthIdentitySection, fifthChallengeRatingKey}.EngineKey()
	zeroKey := Path{fifthStatBlockExperienceSection, fifthStatBlockExperienceZeroKey}.EngineKey()
	inLairKey := Path{fifthStatBlockExperienceSection, fifthStatBlockExperienceInLairKey}.EngineKey()

	zero := fifthRecordFlag("XP 0")
	inLair := fifthRecordFlag("In lair")

	xp := calcField(
		"Experience points",
		fmt.Sprintf(
			"%s == 1 && %s == 0 ? 0 : (%s == 1 && floor(%s) == %s && min(max(%s, 1), 29) == %s ? %s : %s)",
			zeroKey, crKey,
			inLairKey, crKey, crKey, crKey, crKey,
			fifthStatBlockXPLookup(fmt.Sprintf("(%s + 1)", crKey)),
			fifthStatBlockXPLookup(crKey),
		),
		[]string{fifthStatBlockExperienceSection, fifthStatBlockExperienceZeroKey},
		[]string{fifthIdentitySection, fifthChallengeRatingKey},
		[]string{fifthStatBlockExperienceSection, fifthStatBlockExperienceInLairKey},
	)

	return FieldGroup{
		fifthStatBlockExperienceZeroKey:   zero,
		fifthStatBlockExperienceInLairKey: inLair,
		fifthStatBlockExperienceXPKey:     xp,
	}
}

func fifthStatBlockXPLookup(input string) string {
	arguments := make([]string, 0, len(fifthStatBlockXPTable)*2)
	for _, entry := range fifthStatBlockXPTable {
		arguments = append(arguments, fifthStatBlockDecimal(entry.CR), fifthStatBlockDecimal(entry.XP))
	}
	return fmt.Sprintf("lookup(%s, %s)", input, strings.Join(arguments, ", "))
}

func fifthStatBlockDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func fifthStatBlockHitPointsGroup() FieldGroup {
	countKey := Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsCountKey}.EngineKey()
	sizeKey := Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsSizeKey}.EngineKey()
	conBonusKey := Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsConBonusKey}.EngineKey()
	constitutionKey := Path{fifthAbilitiesSection, abilityConstitution, fifthModKey}.EngineKey()

	count := generatableNumber(numField("Hit point dice count", 1), 1, 20)
	count.Schema.ColumnLabel = fifthCountColumnLabel

	size := generatableNumber(numField("Hit point die size", 8), 4, 20)
	size.Schema.ColumnLabel = "Die"

	conBonus := calcField(
		"Constitution bonus",
		fmt.Sprintf("%s * %s", countKey, constitutionKey),
		[]string{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsCountKey},
		[]string{fifthAbilitiesSection, abilityConstitution, fifthModKey},
	)
	conBonus.Schema.ColumnLabel = fifthStatBlockColumnConstitution

	maximum := calcField(
		"Hit points",
		fmt.Sprintf("floor(%s * (%s + 1) / 2) + %s", countKey, sizeKey, conBonusKey),
		[]string{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsCountKey},
		[]string{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsSizeKey},
		[]string{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsConBonusKey},
	)
	maximum.Schema.ColumnLabel = "Maximum"

	current := numField("Current hit points", 0)
	current.Schema.ColumnLabel = "Current"

	temp := numField("Temporary hit points", 0)
	temp.Schema.ColumnLabel = "Temporary"

	return FieldGroup{
		fifthStatBlockHitPointsCountKey:    count,
		fifthStatBlockHitPointsSizeKey:     size,
		fifthStatBlockHitPointsConBonusKey: conBonus,
		fifthStatBlockHitPointsMaximumKey:  maximum,
		fifthStatBlockHitPointsCurrentKey:  current,
		fifthStatBlockHitPointsTempKey:     temp,
	}
}

func fifthStatBlockInitiativeGroup() FieldGroup {
	modifierKey := Path{fifthStatBlockInitiativeSection, fifthStatBlockInitiativeModifierKey}.EngineKey()

	modifier := generatableNumber(numField("Initiative modifier", 0), -30, 30)
	modifier.Schema.ColumnLabel = "Modifier"

	score := calcField(
		"Initiative score",
		fmt.Sprintf("10 + %s", modifierKey),
		[]string{fifthStatBlockInitiativeSection, fifthStatBlockInitiativeModifierKey},
	)
	score.Schema.ColumnLabel = "Score"

	return FieldGroup{
		fifthStatBlockInitiativeModifierKey: modifier,
		fifthStatBlockInitiativeScoreKey:    score,
	}
}

func fifthStatBlockSpeedGroup() FieldGroup {
	exhaustionKey := Path{fifthExhaustionSection, fifthExhaustionLevelKey}.EngineKey()

	group := make(FieldGroup, len(fifthStatBlockSpeedNames)*2)
	for _, name := range fifthStatBlockSpeedNames {
		label := upperFirst(name)
		base := 0.0
		if name == "walk" {
			base = 30
		}

		entered := generatableNumber(numField(label+" speed", base), 0, 1000)
		entered.Schema.ColumnLabel = label

		speedKey := Path{fifthStatBlockSpeedSection, name}.EngineKey()
		total := calcField(
			label+" speed with exhaustion",
			fmt.Sprintf("%s > 0 ? %s - 5 * %s : 0", speedKey, speedKey, exhaustionKey),
			[]string{fifthStatBlockSpeedSection, name},
			[]string{fifthExhaustionSection, fifthExhaustionLevelKey},
		)
		total.Schema.ColumnLabel = label + " with exhaustion"

		group[name] = entered
		group[fifthStatBlockSpeedTotalKey(name)] = total
	}
	return group
}

func fifthStatBlockRowDice() map[string]string {
	initiativeModifier := Path{fifthStatBlockInitiativeSection, fifthStatBlockInitiativeModifierKey}.String()
	exhaustionPenalty := Path{fifthExhaustionSection, fifthExhaustionPenaltyKey}.String()

	return map[string]string{
		Path{fifthStatBlockInitiativeSection, fifthStatBlockInitiativeScoreKey}.String(): fmt.Sprintf(
			"1d20 + %s + %s", initiativeModifier, exhaustionPenalty,
		),
		Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsMaximumKey}.String(): fmt.Sprintf(
			"{%s}d{%s} + %s",
			Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsCountKey}.String(),
			Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsSizeKey}.String(),
			Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsConBonusKey}.String(),
		),
	}
}

func fifthStatBlockNotD20Rows() []string {
	return []string{Path{fifthStatBlockHitPointsSection, fifthStatBlockHitPointsMaximumKey}.String()}
}
