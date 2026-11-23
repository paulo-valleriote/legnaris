package template

import (
	"fmt"
	"strings"
)

const (
	fifthAttacksSection   = "attacks"
	fifthInventorySection = "inventory"
	fifthFeaturesSection  = "features"
	fifthSpellsSection    = "spells"
	fifthResourcesSection = "resources"

	fifthRecordListKey = "list"

	fifthAttacksCapacity   = 10
	fifthInventoryCapacity = 100
	fifthFeaturesCapacity  = 50
	fifthSpellsCapacity    = 100
	fifthResourcesCapacity = 20

	fifthAttackAbilityGroup = "attackAbility"

	fifthAttackToHitKey       = "toHit"
	fifthAttackDamageBonusKey = "damageBonus"
	fifthAttackDamageKey      = "damage"
	fifthAttackNotesKey       = "notes"
	fifthAttackProficientKey  = "proficient"
	fifthAttackMiscKey        = "misc"

	fifthInventoryItemsKey       = "items"
	fifthInventoryQuantityKey    = "quantity"
	fifthInventoryWeightKey      = "weight"
	fifthInventoryAttunedKey     = "attuned"
	fifthInventoryNotesKey       = "notes"
	fifthInventoryLineWeightKey  = "totalWeight"
	fifthInventoryTotalWeightKey = "totalWeight"

	fifthFeatureSourceKey = "source"
	fifthFeatureTextKey   = "text"

	fifthSpellLevelKey         = "level"
	fifthSpellPreparedKey      = "prepared"
	fifthSpellConcentrationKey = "concentration"
	fifthSpellRitualKey        = "ritual"
	fifthSpellNotesKey         = "notes"

	fifthResourceMaxKey           = "max"
	fifthResourceUsedKey          = "used"
	fifthResourceRechargeGroup    = "resourceRecharge"
	fifthResourceRechargeShortKey = "rechargeShort"
	fifthResourceRechargeLongKey  = "rechargeLong"
	fifthResourceRechargeNoneKey  = "rechargeNone"
)

var (
	fifthAttacksOrder = []string{fifthRecordListKey}

	fifthInventoryOrder = []string{
		fifthInventoryItemsKey, fifthInventoryTotalWeightKey, fifthInventoryAttunedKey,
	}

	fifthFeaturesOrder = []string{fifthRecordListKey}

	fifthSpellsOrder = []string{fifthRecordListKey}

	fifthResourcesOrder = []string{fifthRecordListKey}
)

func fifthAttacksGroup() FieldGroup {
	return FieldGroup{
		fifthRecordListKey: {List: &ListSchema{
			Label: "Attacks",
			Columns: []string{
				fieldNameName, fifthAttackToHitKey, fifthAttackDamageBonusKey,
				fifthAttackDamageKey, fifthAttackNotesKey,
			},
			Capacity: fifthAttacksCapacity,
			Shape:    fifthAttackShape(),
			Dice:     "1d20 + @.toHit + exhaustion.penalty",
		}},
	}
}

func fifthAttackShape() FieldGroup {
	shape := FieldGroup{
		fieldNameName:            textField("Name"),
		fifthAttackProficientKey: fifthRecordFlag("Proficient"),
		fifthAttackMiscKey:       numField("Misc", 0),
		fifthAttackDamageKey:     textField("Damage"),
		fifthAttackNotesKey:      textField("Notes"),
	}

	toHitTerms := make([]string, 0, len(abilities))
	damageTerms := make([]string, 0, len(abilities))
	dependencies := make([][]string, 0, len(abilities)*2)

	for _, ability := range abilities {
		key := fifthAttackAbilityKey(ability.key)
		def := 0.0
		if ability.key == abilityStrength {
			def = 1
		}
		shape[key] = fifthChoice("Ability", ability.label, def, fifthAttackAbilityGroup)

		modifier := Path{fifthAbilitiesSection, ability.key, fifthModKey}.EngineKey()
		toHitTerms = append(toHitTerms, fmt.Sprintf("record_%s * %s", key, modifier))
		damageTerms = append(damageTerms, fmt.Sprintf("record_%s * %s", key, modifier))
		dependencies = append(dependencies, []string{key}, []string{fifthAbilitiesSection, ability.key, fifthModKey})
	}

	proficiency := Path{fifthIdentitySection, fieldNameProficiencyBonus}.EngineKey()
	toHitFormula := strings.Join(toHitTerms, " + ") +
		fmt.Sprintf(" + record_%s * %s + record_%s", fifthAttackProficientKey, proficiency, fifthAttackMiscKey)
	toHitDependencies := append([][]string{}, dependencies...)
	toHitDependencies = append(toHitDependencies,
		[]string{fifthAttackProficientKey},
		[]string{fifthIdentitySection, fieldNameProficiencyBonus},
		[]string{fifthAttackMiscKey},
	)

	shape[fifthAttackToHitKey] = calcField("To hit", toHitFormula, toHitDependencies...)
	shape[fifthAttackDamageBonusKey] = calcField("Damage bonus", strings.Join(damageTerms, " + "), dependencies...)
	return shape
}

func fifthAttackAbilityKey(ability string) string {
	return "ability" + upperFirst(ability)
}

func fifthInventoryGroup() FieldGroup {
	return FieldGroup{
		fifthInventoryItemsKey: {List: &ListSchema{
			Label: "Items",
			Columns: []string{
				fieldNameName, fifthInventoryQuantityKey, fifthInventoryWeightKey,
				fifthInventoryLineWeightKey, fifthInventoryAttunedKey, fifthInventoryNotesKey,
			},
			Capacity: fifthInventoryCapacity,
			Shape:    fifthInventoryShape(),
		}},
		fifthInventoryTotalWeightKey: {Schema: &FieldSchema{
			Type:  fieldTypeNumber,
			Label: "Total weight",
			Aggregate: &AggregateSchema{
				List:  "inventory.items",
				Field: fifthInventoryLineWeightKey,
				Op:    AggregateSum,
			},
		}},
		fifthInventoryAttunedKey: {Schema: &FieldSchema{
			Type:  fieldTypeNumber,
			Label: "Attuned items",
			Aggregate: &AggregateSchema{
				List:  "inventory.items",
				Field: fifthInventoryAttunedKey,
				Op:    AggregateCount,
			},
		}},
	}
}

func fifthInventoryShape() FieldGroup {
	return FieldGroup{
		fieldNameName:             textField("Name"),
		fifthInventoryQuantityKey: numField("Quantity", 1),
		fifthInventoryWeightKey:   numField("Weight", 0),
		fifthInventoryAttunedKey:  fifthRecordFlag("Attuned"),
		fifthInventoryNotesKey:    textField("Notes"),
		fifthInventoryLineWeightKey: calcField(
			"Total weight",
			"record_quantity * record_weight",
			[]string{fifthInventoryQuantityKey},
			[]string{fifthInventoryWeightKey},
		),
	}
}

func fifthFeaturesGroup() FieldGroup {
	return FieldGroup{
		fifthRecordListKey: {List: &ListSchema{
			Label:    "Features and feats",
			Columns:  []string{fieldNameName, fifthFeatureSourceKey, fifthFeatureTextKey},
			Capacity: fifthFeaturesCapacity,
			Shape: FieldGroup{
				fieldNameName:         textField("Name"),
				fifthFeatureSourceKey: textField("Source"),
				fifthFeatureTextKey:   textField("Text"),
			},
		}},
	}
}

func fifthSpellsGroup() FieldGroup {
	return FieldGroup{
		fifthRecordListKey: {List: &ListSchema{
			Label: "Spells",
			Columns: []string{
				fieldNameName, fifthSpellLevelKey, fifthSpellPreparedKey,
				fifthSpellConcentrationKey, fifthSpellRitualKey, fifthSpellNotesKey,
			},
			Capacity: fifthSpellsCapacity,
			Shape: FieldGroup{
				fieldNameName:              textField("Name"),
				fifthSpellLevelKey:         numField("Level", 0),
				fifthSpellPreparedKey:      fifthRecordFlag("Prepared"),
				fifthSpellConcentrationKey: fifthRecordFlag("Concentration"),
				fifthSpellRitualKey:        fifthRecordFlag("Ritual"),
				fifthSpellNotesKey:         textField("Notes"),
			},
		}},
	}
}

func fifthResourcesGroup() FieldGroup {
	return FieldGroup{
		fifthRecordListKey: {List: &ListSchema{
			Label: "Resources",
			Columns: []string{
				fieldNameName, fifthResourceMaxKey, fifthResourceUsedKey,
				fifthResourceRechargeShortKey, fifthResourceRechargeLongKey,
				fifthResourceRechargeNoneKey,
			},
			Capacity: fifthResourcesCapacity,
			Shape: FieldGroup{
				fieldNameName:                 textField("Name"),
				fifthResourceMaxKey:           numField("Max", 0),
				fifthResourceUsedKey:          numField("Used", 0),
				fifthResourceRechargeShortKey: fifthChoice("Recharge", "Short rest", 0, fifthResourceRechargeGroup),
				fifthResourceRechargeLongKey:  fifthChoice("Recharge", "Long rest", 0, fifthResourceRechargeGroup),
				fifthResourceRechargeNoneKey:  fifthChoice("Recharge", "None", 1, fifthResourceRechargeGroup),
			},
		}},
	}
}

func fifthRecordFlag(label string) FieldNode {
	node := numField(label, 0)
	node.Schema.Flag = true
	return node
}
