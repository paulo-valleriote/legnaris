package template

import "fmt"

const (
	fifthCombatSection = "combat"
	fifthStatusSection = "status"
	fifthCoinsSection  = "coins"

	fifthInitiativeKey = "initiative"
	fifthAlertKey      = "alert"

	fifthSpeedKey     = "speed"
	fifthSpeedBaseKey = "base"

	fifthArmorStrengthRequirementKey = "armorStrengthRequirement"

	fifthHitPointsKey   = "hp"
	fifthHPBaseKey      = "base"
	fifthHPPerLevelKey  = "perLevel"
	fifthHPMiscKey      = "misc"
	fifthHPMaxKey       = "max"
	fifthHPCurrentKey   = "current"
	fifthHPTempKey      = "temp"
	fifthCarryingKey    = "carryingCapacity"
	fifthXPKey          = "xp"
	fifthInspirationKey = "heroicInspiration"
	fifthConditionsKey  = "conditions"
	fifthDeathSavesKey  = "deathSaves"
	fifthSuccessesKey   = "successes"
	fifthFailuresKey    = "failures"

	fifthInitiativeDice = "1d20 + @.total + exhaustion.penalty"
	fifthDeathSaveDice  = "1d20 + exhaustion.penalty"
)

var fifthConditions = []struct{ key, label string }{
	{"blinded", "Blinded: can't see and automatically fails ability checks that require sight"},
	{"charmed", "Charmed: can't attack the charmer or target it with harmful abilities or magical effects"},
	{"deafened", "Deafened: can't hear and automatically fails ability checks that require hearing"},
	{"frightened", "Frightened: has Disadvantage on ability checks and attacks while the source of fear is in sight"},
	{"grappled", "Grappled: Speed is 0 and can't benefit from bonuses to Speed"},
	{"incapacitated", "Incapacitated: can't take actions, Bonus Actions or Reactions"},
	{"invisible", "Invisible: attacks against the creature have Disadvantage, and its attacks have Advantage"},
	{"paralyzed", "Paralyzed: Incapacitated and can't move or speak; attacks against the creature have Advantage"},
	{"petrified", "Petrified: transformed into a solid substance, Incapacitated and unaware of its surroundings"},
	{"poisoned", "Poisoned: has Disadvantage on attack rolls and ability checks"},
	{"prone", "Prone: can only crawl or stand up; attacks have Disadvantage unless the attacker is within 5 feet"},
	{"restrained", "Restrained: Speed is 0, attacks against it have Advantage, and its attacks have Disadvantage"},
	{"stunned", "Stunned: Incapacitated, can't move and speaks only falteringly"},
	{"unconscious", "Unconscious: Incapacitated and unaware of its surroundings; attacks against it have Advantage"},
}

var (
	fifthCombatOrder = []string{
		fifthInitiativeKey, fifthSpeedKey, fifthArmorStrengthRequirementKey,
		fifthHitPointsKey, fifthCarryingKey,
	}
	fifthInitiativeOrder = []string{fifthAlertKey, fifthMiscKey, fieldNameTotal}
	fifthSpeedOrder      = []string{fifthSpeedBaseKey, fieldNameTotal}
	fifthHPOrder         = []string{
		fifthHPBaseKey, fifthHPPerLevelKey, fifthHPMiscKey,
		fifthHPMaxKey, fifthHPCurrentKey, fifthHPTempKey,
	}
	fifthStatusOrder = []string{
		fifthXPKey, fifthInspirationKey, fifthConditionsKey, fifthDeathSavesKey,
	}
	fifthCoinsOrder = []string{"cp", "sp", "ep", "gp", "pp"}
)

func fifthCharacterCombatGroup() FieldGroup {
	dexterityKey := Path{fifthAbilitiesSection, abilityDexterity, fifthModKey}.EngineKey()
	constitutionKey := Path{fifthAbilitiesSection, abilityConstitution, fifthModKey}.EngineKey()
	strengthKey := Path{fifthAbilitiesSection, abilityStrength, fifthScoreKey}.EngineKey()
	proficiencyKey := Path{fifthIdentitySection, fieldNameProficiencyBonus}.EngineKey()
	levelKey := Path{fifthIdentitySection, fifthLevelKey}.EngineKey()
	exhaustionKey := Path{fifthExhaustionSection, fifthExhaustionLevelKey}.EngineKey()
	heavyArmorKey := Path{fifthArmorSection, fifthArmorChoiceHeavyKey}.EngineKey()

	alertKey := Path{fifthCombatSection, fifthInitiativeKey, fifthAlertKey}.EngineKey()
	initiativeMiscKey := Path{fifthCombatSection, fifthInitiativeKey, fifthMiscKey}.EngineKey()
	speedBaseKey := Path{fifthCombatSection, fifthSpeedKey, fifthSpeedBaseKey}.EngineKey()
	requirementKey := Path{fifthCombatSection, fifthArmorStrengthRequirementKey}.EngineKey()
	hpBaseKey := Path{fifthCombatSection, fifthHitPointsKey, fifthHPBaseKey}.EngineKey()
	hpPerLevelKey := Path{fifthCombatSection, fifthHitPointsKey, fifthHPPerLevelKey}.EngineKey()
	hpMiscKey := Path{fifthCombatSection, fifthHitPointsKey, fifthHPMiscKey}.EngineKey()

	alert := numField("Alert", 0)
	alert.Schema.Flag = true
	alert.Schema.ColumnLabel = "Alert"

	initiativeMisc := generatableNumber(numField("Initiative misc", 0), -20, 20)
	initiativeMisc.Schema.ColumnLabel = upperFirst(fifthMiscKey)

	initiative := calcField(
		"Initiative",
		fmt.Sprintf("%s + %s * %s + %s", dexterityKey, alertKey, proficiencyKey, initiativeMiscKey),
		[]string{fifthAbilitiesSection, abilityDexterity, fifthModKey},
		[]string{fifthIdentitySection, fieldNameProficiencyBonus},
		[]string{fifthCombatSection, fifthInitiativeKey, fifthAlertKey},
		[]string{fifthCombatSection, fifthInitiativeKey, fifthMiscKey},
	)
	initiative.Schema.ColumnLabel = fifthTotalColumnLabel

	speedBase := generatableNumber(numField("Speed base", 30), 0, 120)
	speedBase.Schema.ColumnLabel = "Base"

	speed := calcField(
		"Speed",
		fmt.Sprintf(
			"%s - 5 * %s - 10 * %s * (%s > 0 && %s < %s ? 1 : 0)",
			speedBaseKey, exhaustionKey, heavyArmorKey, requirementKey, strengthKey, requirementKey,
		),
		[]string{fifthCombatSection, fifthSpeedKey, fifthSpeedBaseKey},
		[]string{fifthExhaustionSection, fifthExhaustionLevelKey},
		[]string{fifthArmorSection, fifthArmorChoiceHeavyKey},
		[]string{fifthCombatSection, fifthArmorStrengthRequirementKey},
		[]string{fifthAbilitiesSection, abilityStrength, fifthScoreKey},
	)
	speed.Schema.ColumnLabel = "Speed"

	requirement := numField("Heavy armor Strength requirement", 0)

	hpBase := numField("Hit point base", 0)
	hpBase.Schema.ColumnLabel = "Base"
	hpPerLevel := numField("Hit points per level", 0)
	hpPerLevel.Schema.ColumnLabel = "Per level"
	hpMisc := numField("Hit point misc", 0)
	hpMisc.Schema.ColumnLabel = upperFirst(fifthMiscKey)

	hpMax := calcField(
		"Hit point maximum",
		fmt.Sprintf(
			"%s + %s * %s + %s * %s + %s",
			hpBaseKey, constitutionKey, levelKey, hpPerLevelKey, levelKey, hpMiscKey,
		),
		[]string{fifthCombatSection, fifthHitPointsKey, fifthHPBaseKey},
		[]string{fifthAbilitiesSection, abilityConstitution, fifthModKey},
		[]string{fifthIdentitySection, fifthLevelKey},
		[]string{fifthCombatSection, fifthHitPointsKey, fifthHPPerLevelKey},
		[]string{fifthCombatSection, fifthHitPointsKey, fifthHPMiscKey},
	)
	hpMax.Schema.ColumnLabel = "Maximum"

	hpCurrent := numField("Current hit points", 0)
	hpCurrent.Schema.ColumnLabel = "Current"
	hpTemp := numField("Temporary hit points", 0)
	hpTemp.Schema.ColumnLabel = "Temporary"

	carrying := calcField(
		"Carrying capacity (lb)",
		fmt.Sprintf("%s * 15", strengthKey),
		[]string{fifthAbilitiesSection, abilityStrength, fifthScoreKey},
	)

	return FieldGroup{
		fifthInitiativeKey: FieldNode{
			Children: FieldGroup{
				fifthAlertKey:  alert,
				fifthMiscKey:   initiativeMisc,
				fieldNameTotal: initiative,
			},
			Columns: fifthInitiativeOrder,
			Order:   fifthInitiativeOrder,
		},
		fifthSpeedKey: FieldNode{
			Children: FieldGroup{
				fifthSpeedBaseKey: speedBase,
				fieldNameTotal:    speed,
			},
			Columns: fifthSpeedOrder,
			Order:   fifthSpeedOrder,
		},
		fifthArmorStrengthRequirementKey: requirement,
		fifthHitPointsKey: FieldNode{
			Children: FieldGroup{
				fifthHPBaseKey:     hpBase,
				fifthHPPerLevelKey: hpPerLevel,
				fifthHPMiscKey:     hpMisc,
				fifthHPMaxKey:      hpMax,
				fifthHPCurrentKey:  hpCurrent,
				fifthHPTempKey:     hpTemp,
			},
			Columns: fifthHPOrder,
			Order:   fifthHPOrder,
		},
		fifthCarryingKey: carrying,
	}
}

func fifthCharacterStatusGroup() FieldGroup {
	xp := numField("Experience points", 0)
	xp.Schema.ColumnLabel = "XP"

	inspiration := numField("Heroic Inspiration", 0)
	inspiration.Schema.Flag = true

	conditions := make(FieldGroup, len(fifthConditions))
	for _, condition := range fifthConditions {
		node := numField(condition.label, 0)
		node.Schema.Flag = true
		conditions[condition.key] = node
	}

	successes := fifthBoundedNumber("Death save successes", 0, 0, 3)
	successes.Schema.ColumnLabel = "Successes"
	failures := fifthBoundedNumber("Death save failures", 0, 0, 3)
	failures.Schema.ColumnLabel = "Failures"

	return FieldGroup{
		fifthXPKey:          xp,
		fifthInspirationKey: inspiration,
		fifthConditionsKey: FieldNode{
			Label:    "Conditions",
			Children: conditions,
			Order:    fifthConditionOrder(),
		},
		fifthDeathSavesKey: FieldNode{
			Label: "Death saves (10 or higher succeeds; a natural 1 is two failures; a natural 20 regains 1 hit point and resets both)",
			Children: FieldGroup{
				fifthSuccessesKey: successes,
				fifthFailuresKey:  failures,
			},
			Columns: []string{fifthSuccessesKey, fifthFailuresKey},
			Order:   []string{fifthSuccessesKey, fifthFailuresKey},
		},
	}
}

func fifthCharacterCoinsGroup() FieldGroup {
	coins := []struct{ key, label string }{
		{"cp", "Copper pieces"},
		{"sp", "Silver pieces"},
		{"ep", "Electrum pieces"},
		{"gp", "Gold pieces"},
		{"pp", "Platinum pieces"},
	}
	group := make(FieldGroup, len(coins))
	for _, coin := range coins {
		group[coin.key] = numField(coin.label, 0)
	}
	return group
}

func fifthCharacterRowDice() map[string]string {
	return map[string]string{
		Path{fifthCombatSection, fifthInitiativeKey}.String(): fifthInitiativeDice,
		Path{fifthStatusSection, fifthDeathSavesKey}.String(): fifthDeathSaveDice,
	}
}

func fifthBoundedNumber(label string, def, lo, hi float64) FieldNode {
	node := numField(label, def)
	node.Schema.Min = &lo
	node.Schema.Max = &hi
	return node
}

func fifthConditionOrder() []string {
	keys := make([]string, 0, len(fifthConditions))
	for _, condition := range fifthConditions {
		keys = append(keys, condition.key)
	}
	return keys
}
