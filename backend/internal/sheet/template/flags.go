package template

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var (
	ErrFlagNotEditable = errors.New("flag is only allowed on editable leaves")

	ErrFlagNotNumber = errors.New("flag is only allowed on number leaves")

	ErrFlagDefaultInvalid = errors.New("flag default must be 0 or 1")
)

func ValidateFlags(tmpl Template) error {
	return validateFlagGroup(tmpl.Fields, Path{})
}

func validateFlagGroup(group FieldGroup, prefix Path) error {
	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		current := append(append(Path{}, prefix...), name)
		if node.IsLeaf() {
			if err := validateFlagLeaf(node.Schema); err != nil {
				return fmt.Errorf("field %q: %w", current.String(), err)
			}
			continue
		}
		if node.IsGroup() {
			if err := validateFlagGroup(node.Children, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFlagLeaf(schema *FieldSchema) error {
	if schema == nil || !schema.Flag {
		return nil
	}
	if schema.Type != fieldTypeNumber {
		return ErrFlagNotNumber
	}
	if !schema.IsEditable {
		return ErrFlagNotEditable
	}
	if schema.Default != 0 && schema.Default != 1 {
		return ErrFlagDefaultInvalid
	}
	return nil
}
