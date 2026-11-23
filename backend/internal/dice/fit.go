package dice

import "fmt"

type FieldKind int

const (
	FieldNumber FieldKind = iota + 1
	FieldText
	FieldBool
)

type Schema interface {
	FieldKind(path string) (FieldKind, bool)
}

type MapSchema map[string]FieldKind

func (m MapSchema) FieldKind(path string) (FieldKind, bool) {
	kind, ok := m[path]
	return kind, ok
}

func (e Expression) FitsRow(schema Schema, rowKey string) error {
	for _, term := range e.terms {
		switch term.Kind {
		case KindField:
			if err := checkField(schema, term.fieldPath(rowKey, false), term.fieldPath(rowKey, true)); err != nil {
				return err
			}
		case KindDice:
			if term.CountRef != nil {
				err := checkReference(schema, term.CountRef, rowKey)
				if err != nil {
					return err
				}
			}
			if term.FacesRef != nil {
				err := checkReference(schema, term.FacesRef, rowKey)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkReference(schema Schema, ref *Reference, rowKey string) error {
	path := ref.fieldPath(rowKey, false)
	label := "{" + ref.fieldPath(rowKey, true) + "}"
	return checkField(schema, path, label)
}

func checkField(schema Schema, path, label string) error {
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
