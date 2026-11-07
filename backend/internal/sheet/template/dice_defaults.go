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

// ValidateDiceDefaults checks the template's Dice defaults against the Sections
// and Rows it actually has. Like SuggestedPins it is template content reviewed
// in code, so it is strict and meant to run at boot: a key that is not a
// Section (or Row) → UnknownDiceSectionError (UnknownDiceRowError), an
// expression the dice grammar refuses, a Row default that does not fit its Row
// (an `@` that is not a numeric Field) or a Section default that fits none of
// its Rows → InvalidDiceError wrapping the *dice.Error. Sections are checked
// before Rows and keys in alphabetical order, so the error reported for a given
// template is stable. Nil or empty maps are valid.
func ValidateDiceDefaults(tmpl Template) error {
	if len(tmpl.SectionDice) == 0 && len(tmpl.RowDice) == 0 {
		return nil
	}

	sectionRows := make(map[string][]string)
	rowSet := make(map[string]bool)
	for _, section := range SectionRowKeys(tmpl) {
		sectionRows[section.Section] = section.Rows
		for _, row := range section.Rows {
			rowSet[row] = true
		}
	}
	schema := fieldKinds(tmpl.Fields)

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
	return nil
}

// fieldKinds lists every leaf of the tree by dotted path, which is how a Dice
// expression names a Field.
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

// diceKind maps a Field type to the kind the dice package reasons about. Only
// an explicit number is numeric: a type added later must opt in here rather
// than slip through the boot check as something a Row can roll.
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

// MaxSheetDiceOverrides caps the entries a player's dice block may hold,
// counting Section and Row overrides together (null entries included), so one
// Sheet cannot grow without bound.
const MaxSheetDiceOverrides = 64

// ValidateSheetDice checks a player's Dice overrides against the Template the
// Sheet was built from. It is the strict write-side twin of the tolerant read:
// the cap is checked first, then Sections and Rows in alphabetical order so the
// reported error is stable. A key that is not a Section (Row) →
// UnknownDiceSectionError (UnknownDiceRowError); a non-null value the dice
// grammar refuses, a Row override that does not fit its Row or a Section
// override that fits none of its Rows → InvalidDiceError. A nil value means
// "no dice" on purpose: it needs a known key but no expression. Nil or empty
// maps are valid and never mutated.
func ValidateSheetDice(tmpl Template, sectionDice, rowDice map[string]*string) error {
	if count := len(sectionDice) + len(rowDice); count > MaxSheetDiceOverrides {
		return &TooManyDiceOverridesError{Count: count, Max: MaxSheetDiceOverrides}
	}
	if len(sectionDice) == 0 && len(rowDice) == 0 {
		return nil
	}

	sectionRows := make(map[string][]string)
	rowSet := make(map[string]bool)
	for _, section := range SectionRowKeys(tmpl) {
		sectionRows[section.Section] = section.Rows
		for _, row := range section.Rows {
			rowSet[row] = true
		}
	}
	schema := fieldKinds(tmpl.Fields)

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
			return &UnknownDiceRowError{Key: key}
		}
		if expr := rowDice[key]; expr != nil {
			if _, err := dice.ValidateRow(schema, key, *expr); err != nil {
				return &InvalidDiceError{Scope: diceScopeRow, Key: key, Err: err}
			}
		}
	}
	return nil
}
