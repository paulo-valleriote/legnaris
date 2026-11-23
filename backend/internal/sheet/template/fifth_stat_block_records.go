package template

import (
	"fmt"
	"strings"
)

const (
	fifthStatBlockDetailsSection          = "details"
	fifthStatBlockTraitsSection           = "traits"
	fifthStatBlockActionsSection          = "actions"
	fifthStatBlockBonusActionsSection     = "bonusActions"
	fifthStatBlockReactionsSection        = "reactions"
	fifthStatBlockLegendaryActionsSection = "legendaryActions"
	fifthStatBlockResourcesSection        = "resources"

	fifthStatBlockDetailsResistancesKey = "resistances"
	fifthStatBlockDetailsImmunitiesKey  = "immunities"
	fifthStatBlockDetailsSensesKey      = "senses"
	fifthStatBlockDetailsLanguagesKey   = "languages"

	fifthStatBlockRecordNameKey       = fieldNameName
	fifthStatBlockRecordTextKey       = "text"
	fifthStatBlockRecordDamageKey     = fieldNameDamage
	fifthStatBlockRecordNotesKey      = fieldNameNotes
	fifthStatBlockRecordProficientKey = "proficient"
	fifthStatBlockRecordMiscKey       = "misc"
	fifthStatBlockRecordToHitKey      = "toHit"
	fifthStatBlockRecordMaxKey        = "max"
	fifthStatBlockRecordUsedKey       = "used"
	fifthStatBlockRecordRemainingKey  = fieldNameRemaining
	fifthStatBlockRecordRechargeKey   = "recharge"

	fifthStatBlockActionAbilityGroup  = "actionAbility"
	fifthStatBlockRecordCapacity      = 50
	fifthStatBlockDetailTextMaxLength = 400

	fifthStatBlockColumnName = "Name"
	fifthStatBlockColumnMisc = "Misc"
)

var fifthStatBlockDetailsOrder = []string{
	fifthStatBlockDetailsResistancesKey, fifthStatBlockDetailsImmunitiesKey,
	fifthStatBlockDetailsSensesKey, fifthStatBlockDetailsLanguagesKey,
}

func fifthStatBlockDetailsGroup() FieldGroup {
	return FieldGroup{
		fifthStatBlockDetailsResistancesKey: fifthStatBlockText("Damage resistances", fifthStatBlockDetailTextMaxLength),
		fifthStatBlockDetailsImmunitiesKey:  fifthStatBlockText("Damage immunities", fifthStatBlockDetailTextMaxLength),
		fifthStatBlockDetailsSensesKey:      fifthStatBlockText("Senses", fifthStatBlockDetailTextMaxLength),
		fifthStatBlockDetailsLanguagesKey:   fifthStatBlockText("Languages", fifthStatBlockDetailTextMaxLength),
	}
}

func fifthStatBlockText(label string, maxLength int) FieldNode {
	node := textField(label)
	node.Schema.MaxLength = &maxLength
	return node
}

func fifthStatBlockTraitsList() *ListSchema {
	return fifthStatBlockTextRecordList("Traits")
}

func fifthStatBlockBonusActionsList() *ListSchema {
	return fifthStatBlockTextRecordList("Bonus actions")
}

func fifthStatBlockReactionsList() *ListSchema {
	return fifthStatBlockTextRecordList("Reactions")
}

func fifthStatBlockLegendaryActionsList() *ListSchema {
	return fifthStatBlockTextRecordList("Legendary actions")
}

func fifthStatBlockTextRecordList(label string) *ListSchema {
	name := fifthStatBlockText("Name", 60)
	name.Schema.ColumnLabel = fifthStatBlockColumnName
	text := fifthStatBlockText("Text", 500)
	text.Schema.ColumnLabel = "Text"

	return &ListSchema{
		Label:    label,
		Columns:  []string{fifthStatBlockRecordNameKey, fifthStatBlockRecordTextKey},
		Capacity: fifthStatBlockRecordCapacity,
		Shape: FieldGroup{
			fifthStatBlockRecordNameKey: name,
			fifthStatBlockRecordTextKey: text,
		},
	}
}

func fifthStatBlockActionsList() *ListSchema {
	return &ListSchema{
		Label:    "Actions",
		Columns:  []string{fifthStatBlockRecordNameKey, fifthStatBlockRecordToHitKey, fifthStatBlockRecordDamageKey, fifthStatBlockRecordNotesKey},
		Capacity: fifthStatBlockRecordCapacity,
		Shape:    fifthStatBlockActionsShape(),
		Dice: fmt.Sprintf(
			"1d20 + @.%s + %s",
			fifthStatBlockRecordToHitKey,
			Path{fifthExhaustionSection, fifthExhaustionPenaltyKey}.String(),
		),
	}
}

func fifthStatBlockActionsShape() FieldGroup {
	name := fifthStatBlockText("Action name", 60)
	name.Schema.ColumnLabel = fifthStatBlockColumnName

	damage := fifthStatBlockText("Damage", 80)
	damage.Schema.ColumnLabel = "Damage"

	notes := fifthStatBlockText("Notes", 120)
	notes.Schema.ColumnLabel = "Notes"

	misc := numField("To hit misc", 0)
	misc.Schema.ColumnLabel = fifthStatBlockColumnMisc

	formula, dependsOn := fifthStatBlockActionToHitFormula()
	toHit := calcField("To hit", formula, dependsOn...)
	toHit.Schema.ColumnLabel = "To hit"

	shape := fifthStatBlockActionAbilityShape()
	shape[fifthStatBlockRecordNameKey] = name
	shape[fifthStatBlockRecordToHitKey] = toHit
	shape[fifthStatBlockRecordDamageKey] = damage
	shape[fifthStatBlockRecordNotesKey] = notes
	shape[fifthStatBlockRecordProficientKey] = fifthRecordFlag("Proficient")
	shape[fifthStatBlockRecordMiscKey] = misc
	return shape
}

func fifthStatBlockActionAbilityShape() FieldGroup {
	shape := make(FieldGroup, len(abilities))
	for _, ability := range abilities {
		shape[fifthAbilityFlagKey(ability.key)] = fifthChoice(
			"Ability", ability.label, 0, fifthStatBlockActionAbilityGroup,
		)
	}
	return shape
}

func fifthStatBlockActionToHitFormula() (string, [][]string) {
	terms := make([]string, 0, len(abilities)+2)
	dependsOn := make([][]string, 0, len(abilities)*2+2)

	for _, ability := range abilities {
		field := fifthAbilityFlagKey(ability.key)
		terms = append(terms, fmt.Sprintf(
			"record_%s * %s",
			field,
			Path{fifthAbilitiesSection, ability.key, fifthModKey}.EngineKey(),
		))
		dependsOn = append(dependsOn,
			[]string{field},
			[]string{fifthAbilitiesSection, ability.key, fifthModKey},
		)
	}

	terms = append(terms, fmt.Sprintf(
		"%s * record_%s",
		Path{fifthIdentitySection, fieldNameProficiencyBonus}.EngineKey(),
		fifthStatBlockRecordProficientKey,
	))
	dependsOn = append(dependsOn,
		[]string{fifthStatBlockRecordProficientKey},
		[]string{fifthIdentitySection, fieldNameProficiencyBonus},
	)

	terms = append(terms, "record_"+fifthStatBlockRecordMiscKey)
	dependsOn = append(dependsOn, []string{fifthStatBlockRecordMiscKey})

	return strings.Join(terms, " + "), dependsOn
}

func fifthStatBlockResourcesList() *ListSchema {
	name := fifthStatBlockText("Resource name", 60)
	name.Schema.ColumnLabel = fifthStatBlockColumnName

	maximum := numField("Maximum uses", 0)
	maximum.Schema.ColumnLabel = "Max"

	used := numField("Used uses", 0)
	used.Schema.ColumnLabel = "Used"

	remaining := calcField(
		"Remaining uses",
		fmt.Sprintf("record_%s - record_%s", fifthStatBlockRecordMaxKey, fifthStatBlockRecordUsedKey),
		[]string{fifthStatBlockRecordMaxKey},
		[]string{fifthStatBlockRecordUsedKey},
	)
	remaining.Schema.ColumnLabel = "Remaining"

	recharge := fifthStatBlockText("Recharge", 40)
	recharge.Schema.ColumnLabel = "Recharge"

	return &ListSchema{
		Label:    "Resources",
		Columns:  []string{fifthStatBlockRecordNameKey, fifthStatBlockRecordMaxKey, fifthStatBlockRecordUsedKey, fifthStatBlockRecordRemainingKey, fifthStatBlockRecordRechargeKey},
		Capacity: fifthStatBlockRecordCapacity,
		Shape: FieldGroup{
			fifthStatBlockRecordNameKey:      name,
			fifthStatBlockRecordMaxKey:       maximum,
			fifthStatBlockRecordUsedKey:      used,
			fifthStatBlockRecordRemainingKey: remaining,
			fifthStatBlockRecordRechargeKey:  recharge,
		},
	}
}
