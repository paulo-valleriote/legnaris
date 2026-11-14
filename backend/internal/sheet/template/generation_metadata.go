package template

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var (
	// ErrGeneratableNotEditable reports a leaf marked Generatable that the
	// player cannot edit: derived leaves are computed, never generated.
	ErrGeneratableNotEditable = errors.New("generatable leaf must be editable")

	// ErrRangeInverted reports a Min greater than its Max.
	ErrRangeInverted = errors.New("min must not exceed max")

	// ErrRangeOnNonNumber reports Min or Max on a leaf that is not a number.
	ErrRangeOnNonNumber = errors.New("min and max are only allowed on number leaves")

	// ErrMaxLengthInvalid reports a MaxLength that is set but not positive.
	ErrMaxLengthInvalid = errors.New("max length must be greater than zero")

	// ErrMaxLengthOnNonText reports a MaxLength on a leaf that is not text.
	ErrMaxLengthOnNonText = errors.New("max length is only allowed on text leaves")
)

// ValidateGenerationMetadata checks the generation metadata of every leaf
// against the rules the generation validator relies on: only editable leaves
// may be generatable, Min must not exceed Max, ranges apply to number leaves
// only and a max length to text leaves only, and a set max length is positive.
// Leaves with no metadata are valid. Groups are walked in sorted key order so
// the first reported violation is the same on every boot.
func ValidateGenerationMetadata(tmpl Template) error {
	return validateGenerationGroup(tmpl.Fields, Path{})
}

// validateGenerationGroup walks one level of the field tree in sorted key
// order, recursing into groups and checking each leaf's metadata.
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

// validateLeafGeneration applies the per-leaf rules in a fixed order, so a
// leaf with several violations always reports the same one.
func validateLeafGeneration(schema *FieldSchema) error {
	if schema.Generatable && !schema.IsEditable {
		return ErrGeneratableNotEditable
	}
	if err := validateRange(schema); err != nil {
		return err
	}
	return validateMaxLength(schema)
}

// validateRange enforces that Min and Max only appear on number leaves and
// that a set pair is ordered.
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

// validateMaxLength enforces that MaxLength only appears on text leaves and,
// when set, is positive. A nil MaxLength means no bound and is always valid.
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
