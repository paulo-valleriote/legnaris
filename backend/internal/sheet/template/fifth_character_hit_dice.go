package template

import "fmt"

const (
	fifthHitDiceSection = "hitDice"

	fifthHitDiePrimaryKey = "primary"
	fifthHitDieSizeKey    = "size"
	fifthHitDieCountKey   = "count"
	fifthHitDieExtraKey   = "extra"
	fifthHitDieSpentKey   = "spent"

	fifthSpentColumnLabel     = "Spent"
	fifthRemainingColumnLabel = "Remaining"
)

var fifthExtraHitDice = []string{"d6", "d8", "d10", "d12"}

func fifthHitDiceGroup() FieldGroup {
	group := FieldGroup{fifthHitDiePrimaryKey: fifthPrimaryHitDiePool()}
	for _, size := range fifthExtraHitDice {
		group[size] = fifthExtraHitDiePool(size)
	}
	return group
}

func fifthHitDiceOrder() []string {
	return append([]string{fifthHitDiePrimaryKey}, fifthExtraHitDice...)
}

func fifthPrimaryHitDiePool() FieldNode {
	countPath := []string{fifthHitDiceSection, fifthHitDiePrimaryKey, fifthHitDieCountKey}
	spentPath := []string{fifthHitDiceSection, fifthHitDiePrimaryKey, fifthHitDieSpentKey}

	size := generatableNumber(numField("Hit Die size", 8), 4, 12)
	size.Schema.ColumnLabel = "Size"
	count := calcField(
		"Primary Hit Dice",
		fmt.Sprintf("max(0, %s - (%s))", fifthIdentityLevelEngineKey(), fifthExtraHitDiceSum()),
		fifthPrimaryHitDieDependencies()...,
	)
	count.Schema.ColumnLabel = "Count"
	spent := numField("Primary Hit Dice spent", 0)
	spent.Schema.ColumnLabel = fifthSpentColumnLabel
	remaining := calcField(
		"Primary Hit Dice remaining",
		fmt.Sprintf("max(0, %s - %s)", Path(countPath).EngineKey(), Path(spentPath).EngineKey()),
		countPath,
		spentPath,
	)
	remaining.Schema.ColumnLabel = fifthRemainingColumnLabel

	return FieldNode{
		Children: FieldGroup{
			fifthHitDieSizeKey:  size,
			fifthHitDieCountKey: count,
			fifthHitDieSpentKey: spent,
			fieldNameRemaining:  remaining,
		},
		Label:   "Primary Hit Dice",
		Columns: []string{fifthHitDieSizeKey, fifthHitDieCountKey, fifthHitDieSpentKey, fieldNameRemaining},
		Order:   []string{fifthHitDieSizeKey, fifthHitDieCountKey, fifthHitDieSpentKey, fieldNameRemaining},
	}
}

func fifthExtraHitDiePool(size string) FieldNode {
	extraPath := []string{fifthHitDiceSection, size, fifthHitDieExtraKey}
	spentPath := []string{fifthHitDiceSection, size, fifthHitDieSpentKey}

	extra := numField(size+" Hit Dice extra", 0)
	extra.Schema.ColumnLabel = "Extra"
	spent := numField(size+" Hit Dice spent", 0)
	spent.Schema.ColumnLabel = fifthSpentColumnLabel
	remaining := calcField(
		size+" Hit Dice remaining",
		fmt.Sprintf("max(0, %s - %s)", Path(extraPath).EngineKey(), Path(spentPath).EngineKey()),
		extraPath,
		spentPath,
	)
	remaining.Schema.ColumnLabel = fifthRemainingColumnLabel

	return FieldNode{
		Children: FieldGroup{
			fifthHitDieExtraKey: extra,
			fifthHitDieSpentKey: spent,
			fieldNameRemaining:  remaining,
		},
		Label:   size + " Hit Dice",
		Columns: []string{fifthHitDieExtraKey, fifthHitDieSpentKey, fieldNameRemaining},
		Order:   []string{fifthHitDieExtraKey, fifthHitDieSpentKey, fieldNameRemaining},
	}
}

func fifthExtraHitDiceSum() string {
	sum := ""
	for i, size := range fifthExtraHitDice {
		if i > 0 {
			sum += " + "
		}
		sum += Path{fifthHitDiceSection, size, fifthHitDieExtraKey}.EngineKey()
	}
	return sum
}

func fifthPrimaryHitDieDependencies() [][]string {
	deps := [][]string{{fifthIdentitySection, fifthLevelKey}}
	for _, size := range fifthExtraHitDice {
		deps = append(deps, []string{fifthHitDiceSection, size, fifthHitDieExtraKey})
	}
	return deps
}

func fifthIdentityLevelEngineKey() string {
	return Path{fifthIdentitySection, fifthLevelKey}.EngineKey()
}

func fifthHitDiceRowDice() map[string]string {
	rows := make(map[string]string, len(fifthExtraHitDice)+1)
	rows[Path{fifthHitDiceSection, fifthHitDiePrimaryKey}.String()] = fmt.Sprintf(
		"1d{%s} + abilities.constitution.mod min 1",
		Path{fifthHitDiceSection, fifthHitDiePrimaryKey, fifthHitDieSizeKey}.String(),
	)
	for _, size := range fifthExtraHitDice {
		rows[Path{fifthHitDiceSection, size}.String()] = fmt.Sprintf(
			"1%s + abilities.constitution.mod min 1", size,
		)
	}
	return rows
}
