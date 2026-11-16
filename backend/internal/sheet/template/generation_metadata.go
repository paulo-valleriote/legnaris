package template

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var (
	ErrGeneratableNotEditable = errors.New("generatable leaf must be editable")

	ErrRangeInverted = errors.New("min must not exceed max")

	ErrRangeOnNonNumber = errors.New("min and max are only allowed on number leaves")

	ErrMaxLengthInvalid = errors.New("max length must be greater than zero")

	ErrMaxLengthOnNonText = errors.New("max length is only allowed on text leaves")
)

func ValidateGenerationMetadata(tmpl Template) error {
	return validateGenerationGroup(tmpl.Fields, Path{})
}

func validateGenerationGroup(group FieldGroup, prefix Path) error {
	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		current := append(append(Path{}, prefix...), name)
		if node.IsLeaf() {
			if err := validateLeafGeneration(node.Schema); err != nil {
				return fmt.Errorf("field %q: %w", current.String(), err)
			}
			continue
		}
		if node.IsGroup() {
			if err := validateGenerationGroup(node.Children, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLeafGeneration(schema *FieldSchema) error {
	if schema.Generatable && !schema.IsEditable {
		return ErrGeneratableNotEditable
	}
	if err := validateRange(schema); err != nil {
		return err
	}
	return validateMaxLength(schema)
}

func validateRange(schema *FieldSchema) error {
	if schema.Min == nil && schema.Max == nil {
		return nil
	}
	if schema.Type != fieldTypeNumber {
		return ErrRangeOnNonNumber
	}
	if schema.Min != nil && schema.Max != nil && *schema.Min > *schema.Max {
		return ErrRangeInverted
	}
	return nil
}

func validateMaxLength(schema *FieldSchema) error {
	if schema.MaxLength == nil {
		return nil
	}
	if schema.Type != fieldTypeText {
		return ErrMaxLengthOnNonText
	}
	if *schema.MaxLength <= 0 {
		return ErrMaxLengthInvalid
	}
	return nil
}
