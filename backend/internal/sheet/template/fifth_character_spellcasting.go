package template

import (
	"fmt"
	"strings"
)

const (
	fifthSpellcastingSection = "spellcasting"

	fifthSpellAbilityModKey = "abilityMod"
	fifthSpellAttackKey     = "attack"
	fifthSpellSaveDCKey     = "saveDC"
	fifthPreparedLimitKey   = "preparedLimit"
	fifthSpellSlotKeyPrefix = "slot"
	fifthSpellSlotSpentKey  = "spent"
	fifthPactKey            = "pact"
	fifthPactCountKey       = "count"
	fifthPactLevelKey       = "level"
	fifthPactUsedKey        = "used"
)

func fifthSpellcastingGroup() FieldGroup {
	group := make(FieldGroup, len(abilities)+4+9+1)
	modTerms := make([]string, 0, len(abilities))
	modDependencies := make([][]string, 0, len(abilities)*2)

	for _, ability := range abilities {
		abilityKey := fifthAbilityFlagKey(ability.key)
		group[abilityKey] = fifthChoice(
			"Spellcasting ability", ability.label, 0, exclusiveGroupSpellcastingAbility,
		)
		modTerms = append(modTerms, fmt.Sprintf(
			"%s * %s",
			Path{fifthAbilitiesSection, ability.key, fifthModKey}.EngineKey(),
			Path{fifthSpellcastingSection, abilityKey}.EngineKey(),
		))
		modDependencies = append(
			modDependencies,
			[]string{fifthAbilitiesSection, ability.key, fifthModKey},
			[]string{fifthSpellcastingSection, abilityKey},
		)
	}

	abilityModPath := []string{fifthSpellcastingSection, fifthSpellAbilityModKey}
	proficiencyPath := []string{fifthIdentitySection, fieldNameProficiencyBonus}

	abilityMod := calcField(
		"Spellcasting ability modifier",
		strings.Join(modTerms, " + "),
		modDependencies...,
	)
	attack := calcField(
		"Spell attack bonus",
		fmt.Sprintf("%s + %s", Path(abilityModPath).EngineKey(), Path(proficiencyPath).EngineKey()),
		abilityModPath,
		proficiencyPath,
	)
	saveDC := calcField(
		"Spell save DC",
		fmt.Sprintf("8 + %s + %s", Path(abilityModPath).EngineKey(), Path(proficiencyPath).EngineKey()),
		abilityModPath,
		proficiencyPath,
	)

	group[fifthSpellAbilityModKey] = abilityMod
	group[fifthSpellAttackKey] = attack
	group[fifthSpellSaveDCKey] = saveDC
	group[fifthPreparedLimitKey] = numField("Prepared spell limit", 0)

	for level := 1; level <= 9; level++ {
		key := fifthSpellSlotKey(level)
		group[key] = fifthSpellSlotGroup(level, key)
	}

	group[fifthPactKey] = fifthPactMagicGroup()

	return group
}

func fifthSpellcastingOrder() []string {
	order := make([]string, 0, len(abilities)+4+9+1)
	for _, ability := range abilities {
		order = append(order, fifthAbilityFlagKey(ability.key))
	}
	order = append(order,
		fifthSpellAbilityModKey, fifthSpellAttackKey, fifthSpellSaveDCKey, fifthPreparedLimitKey,
	)
	for level := 1; level <= 9; level++ {
		order = append(order, fifthSpellSlotKey(level))
	}
	return append(order, fifthPactKey)
}

func fifthSpellSlotKey(level int) string {
	return fmt.Sprintf("%s%d", fifthSpellSlotKeyPrefix, level)
}

func fifthSpellSlotGroup(level int, key string) FieldNode {
	totalPath := []string{fifthSpellcastingSection, key, fieldNameTotal}
	spentPath := []string{fifthSpellcastingSection, key, fifthSpellSlotSpentKey}

	total := numField(fmt.Sprintf("Level %d spell slots", level), 0)
	total.Schema.ColumnLabel = fifthTotalColumnLabel
	spent := numField(fmt.Sprintf("Level %d spell slots spent", level), 0)
	spent.Schema.ColumnLabel = fifthSpentColumnLabel
	remaining := calcField(
		fmt.Sprintf("Level %d spell slots remaining", level),
		fmt.Sprintf("max(0, %s - %s)", Path(totalPath).EngineKey(), Path(spentPath).EngineKey()),
		totalPath,
		spentPath,
	)
	remaining.Schema.ColumnLabel = fifthRemainingColumnLabel

	return FieldNode{
		Children: FieldGroup{
			fieldNameTotal:         total,
			fifthSpellSlotSpentKey: spent,
			fieldNameRemaining:     remaining,
		},
		Label:   fmt.Sprintf("Level %d", level),
		Columns: []string{fieldNameTotal, fifthSpellSlotSpentKey, fieldNameRemaining},
		Order:   []string{fieldNameTotal, fifthSpellSlotSpentKey, fieldNameRemaining},
	}
}

func fifthPactMagicGroup() FieldNode {
	countPath := []string{fifthSpellcastingSection, fifthPactKey, fifthPactCountKey}
	usedPath := []string{fifthSpellcastingSection, fifthPactKey, fifthPactUsedKey}

	count := numField("Pact Magic slots", 0)
	count.Schema.ColumnLabel = fifthCountColumnLabel
	level := numField("Pact Magic slot level", 0)
	level.Schema.ColumnLabel = "Level"
	used := numField("Pact Magic slots used", 0)
	used.Schema.ColumnLabel = "Used"
	remaining := calcField(
		"Pact Magic slots remaining",
		fmt.Sprintf("max(0, %s - %s)", Path(countPath).EngineKey(), Path(usedPath).EngineKey()),
		countPath,
		usedPath,
	)
	remaining.Schema.ColumnLabel = fifthRemainingColumnLabel

	return FieldNode{
		Children: FieldGroup{
			fifthPactCountKey:  count,
			fifthPactLevelKey:  level,
			fifthPactUsedKey:   used,
			fieldNameRemaining: remaining,
		},
		Label:   "Pact Magic",
		Columns: []string{fifthPactCountKey, fifthPactLevelKey, fifthPactUsedKey, fieldNameRemaining},
		Order:   []string{fifthPactCountKey, fifthPactLevelKey, fifthPactUsedKey, fieldNameRemaining},
	}
}

func fifthSpellcastingRowDice() map[string]string {
	return map[string]string{
		Path{fifthSpellcastingSection, fifthSpellAttackKey}.String(): "1d20 + @ + exhaustion.penalty",
	}
}
