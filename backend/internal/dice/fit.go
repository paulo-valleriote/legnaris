package dice

import "fmt"

// FieldKind is the type of a Template Field as far as a Dice expression cares.
type FieldKind int

const (
	// FieldNumber is a numeric Field, the only kind a Dice expression can read.
	FieldNumber FieldKind = iota + 1
	// FieldText is a text Field.
	FieldText
	// FieldBool is a boolean Field.
	FieldBool
)

// Schema answers what a Field path is. It reports false for a path that is not
// a Field (a missing path, a group, a Section or a Row key that is not a leaf).
type Schema interface {
	FieldKind(path string) (FieldKind, bool)
}

// MapSchema is a Schema backed by a map of dotted Field path to kind.
type MapSchema map[string]FieldKind

// FieldKind implements Schema.
func (m MapSchema) FieldKind(path string) (FieldKind, bool) {
	kind, ok := m[path]
	return kind, ok
}

// FitsRow reports whether the expression can be rolled for the Row keyed
// rowKey: every Field it names must exist and be numeric, where `@` and
// `@.<column>` resolve against rowKey. It returns nil when it fits, otherwise
// the first problem as an *Error of class unknown_field or non_numeric_field.
// An expression that names no Field fits any Row key.
func (e Expression) FitsRow(schema Schema, rowKey string) error {
	for _, term := range e.terms {
		if term.Kind != KindField {
			continue
		}
		if err := checkField(schema, term, rowKey); err != nil {
			return err
		}
	}
	return nil
}

func checkField(schema Schema, term Term, rowKey string) error {
	path := term.fieldPath(rowKey, false)
	label := term.fieldPath(rowKey, true)
	var kind FieldKind
	ok := schema != nil
	if ok {
		kind, ok = schema.FieldKind(path)
	}
	if !ok {
		return newError(ClassUnknownField, fmt.Sprintf("%q points at %q, which does not exist", label, path))
	}
	if kind != FieldNumber {
		return newError(ClassNonNumericField, fmt.Sprintf("%q is not a number", path))
	}
	return nil
}

// ValidateRow parses expr and checks that it fits the Row keyed rowKey. It is
// the check a Row-level Dice expression must pass before it is stored.
func ValidateRow(schema Schema, rowKey, expr string) (Expression, error) {
	parsed, err := Parse(expr)
	if err != nil {
		return Expression{}, err
	}
	if err := parsed.FitsRow(schema, rowKey); err != nil {
		return Expression{}, err
	}
	return parsed, nil
}

// ValidateSection parses expr and checks that it fits at least one of the
// Section's Rows, since a Section expression is inherited by every Row but
// only has to be rollable on some. A Section with no Rows fits nothing; when no
// Row fits, the error carries the class of the first Row's problem.
func ValidateSection(schema Schema, rowKeys []string, expr string) (Expression, error) {
	parsed, err := Parse(expr)
	if err != nil {
		return Expression{}, err
	}
	if len(rowKeys) == 0 {
		return Expression{}, ErrNoRows
	}
	var first error
	for _, rowKey := range rowKeys {
		fitErr := parsed.FitsRow(schema, rowKey)
		if fitErr == nil {
			return parsed, nil
		}
		if first == nil {
			first = fitErr
		}
	}
	class, _ := ClassOf(first)
	return Expression{}, newError(class, "it fits none of the Rows in the Section: "+first.Error())
}
