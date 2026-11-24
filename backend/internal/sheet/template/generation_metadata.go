package template

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"legnaris/backend/internal/plan"
)

var (
	ErrGeneratableNotEditable = errors.New("generatable leaf must be editable")

	ErrGeneratableKindsOnNonGeneratable = errors.New("generatable kinds are only allowed on generatable leaves")

	ErrGeneratableKindInvalid = errors.New("generatable kind is not a valid sheet kind")

	ErrPreferDefaultOnNonGeneratable = errors.New("prefer default is only allowed on generatable leaves")

	ErrPreferDefaultOutOfBounds = errors.New("prefer default is outside the declared bounds")

	ErrRangeInverted = errors.New("min must not exceed max")

	ErrRangeOnNonNumber = errors.New("min and max are only allowed on number leaves")

	ErrMaxLengthInvalid = errors.New("max length must be greater than zero")

	ErrMaxLengthOnNonText = errors.New("max length is only allowed on text leaves")

	ErrMaxRecordsInvalid = errors.New("max records must be between one and capacity")
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
		if node.IsList() {
			if node.List != nil {
				if err := validateMaxRecords(node.List); err != nil {
					return fmt.Errorf("list %q: %w", current.String(), err)
				}
				if err := validateGenerationGroup(node.List.Shape, current); err != nil {
					return err
				}
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
	if err := validateGeneratableKinds(schema); err != nil {
		return err
	}
	if err := validatePreferDefault(schema); err != nil {
		return err
	}
	if err := validateRange(schema); err != nil {
		return err
	}
	return validateMaxLength(schema)
}

func validateGeneratableKinds(schema *FieldSchema) error {
	if len(schema.GeneratableKinds) == 0 {
		return nil
	}
	if !schema.Generatable {
		return ErrGeneratableKindsOnNonGeneratable
	}
	for _, kind := range schema.GeneratableKinds {
		if _, ok := plan.ParseSheetKind(string(kind)); !ok {
			return fmt.Errorf("%w: %q", ErrGeneratableKindInvalid, kind)
		}
	}
	return nil
}

func validatePreferDefault(schema *FieldSchema) error {
	if !schema.PreferDefault {
		return nil
	}
	if !schema.Generatable {
		return ErrPreferDefaultOnNonGeneratable
	}
	if schema.Min != nil && schema.Default < *schema.Min {
		return ErrPreferDefaultOutOfBounds
	}
	if schema.Max != nil && schema.Default > *schema.Max {
		return ErrPreferDefaultOutOfBounds
	}
	return nil
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

func validateMaxRecords(list *ListSchema) error {
	if list.MaxRecords == 0 {
		return nil
	}
	if list.MaxRecords < 0 || list.MaxRecords > list.Capacity {
		return fmt.Errorf("%w: %d (capacity %d)", ErrMaxRecordsInvalid, list.MaxRecords, list.Capacity)
	}
	return nil
}
