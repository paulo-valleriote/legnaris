package template

import (
	"maps"
	"slices"

	"legnaris/backend/internal/dice"
)

const (
	diceScopeSection = "section"
	diceScopeRow     = "row"
)

func ValidateDiceDefaults(tmpl Template) error {
	sectionRows := sectionRowsForFit(tmpl)
	rowSet := staticRowKeys(tmpl)
	schema := rowDiceSchema(tmpl)

	for _, key := range sortedKeys(tmpl.SectionDice) {
		rows, ok := sectionRows[key]
		if !ok {
			return &UnknownDiceSectionError{Key: key}
		}
		if _, err := dice.ValidateSection(schema, rows, tmpl.SectionDice[key]); err != nil {
			return &InvalidDiceError{Scope: diceScopeSection, Key: key, Err: err}
		}
	}
	for _, key := range sortedKeys(tmpl.RowDice) {
		if !rowSet[key] {
			return &UnknownDiceRowError{Key: key}
		}
		if _, err := dice.ValidateRow(schema, key, tmpl.RowDice[key]); err != nil {
			return &InvalidDiceError{Scope: diceScopeRow, Key: key, Err: err}
		}
	}
	lists := templateLists(tmpl)
	for _, listPath := range slices.Sorted(maps.Keys(lists)) {
		expression := lists[listPath].Dice
		if expression == "" {
			continue
		}
		row := RecordRowKey(listPath, syntheticRecordID)
		if _, err := dice.ValidateRow(schema, row, expression); err != nil {
			return &InvalidDiceError{Scope: diceScopeRow, Key: listPath, Err: err}
		}
	}
	return nil
}

func fieldKinds(group FieldGroup) dice.MapSchema {
	kinds := make(dice.MapSchema)
	var walk func(path string, node FieldNode)
	walk = func(path string, node FieldNode) {
		if node.IsLeaf() {
			kinds[path] = diceKind(node.Schema.Type)
			return
		}
		for name, child := range node.Children {
			walk(path+pathSeparator+name, child)
		}
	}
	for name, node := range group {
		walk(name, node)
	}
	return kinds
}

func diceKind(fieldType string) dice.FieldKind {
	switch fieldType {
	case fieldTypeNumber:
		return dice.FieldNumber
	case fieldTypeBool:
		return dice.FieldBool
	default:
		return dice.FieldText
	}
}

func sortedKeys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}

const MaxSheetDiceOverrides = 64

func ValidateSheetDice(tmpl Template, sectionDice, rowDice map[string]*string) error {
	if count := len(sectionDice) + len(rowDice); count > MaxSheetDiceOverrides {
		return &TooManyDiceOverridesError{Count: count, Max: MaxSheetDiceOverrides}
	}
	if len(sectionDice) == 0 && len(rowDice) == 0 {
		return nil
	}

	sectionRows := sectionRowsForFit(tmpl)
	rowSet := staticRowKeys(tmpl)
	schema := rowDiceSchema(tmpl)

	for _, key := range slices.Sorted(maps.Keys(sectionDice)) {
		rows, ok := sectionRows[key]
		if !ok {
			return &UnknownDiceSectionError{Key: key}
		}
		if expr := sectionDice[key]; expr != nil {
			if _, err := dice.ValidateSection(schema, rows, *expr); err != nil {
				return &InvalidDiceError{Scope: diceScopeSection, Key: key, Err: err}
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(rowDice)) {
		if !rowSet[key] {
			if _, ok := ParseRecordRowKey(tmpl, key); !ok {
				return &UnknownDiceRowError{Key: key}
			}
		}
		if expr := rowDice[key]; expr != nil {
			if _, err := dice.ValidateRow(schema, key, *expr); err != nil {
				return &InvalidDiceError{Scope: diceScopeRow, Key: key, Err: err}
			}
		}
	}
	return nil
}
