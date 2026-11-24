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

	ErrGenerationEnumOnNonGeneratable = errors.New("generation enum is only allowed on generatable leaves")

	ErrGenerationEnumOnNonText = errors.New("generation enum is only allowed on text leaves")

	ErrGenerationEnumDuplicate = errors.New("generation enum values must be unique")

	ErrClassSubclassInvalid = errors.New("class and subclass rule is invalid")
)

type ClassSubclassRule struct {
	Class         []string `bson:"class"`
	Subclass      []string `bson:"subclass"`
	Level         []string `bson:"level"`
	SubclassLevel int      `bson:"subclass_level"`
}

func ValidateGenerationMetadata(tmpl Template) error {
	if err := validateClassSubclass(tmpl); err != nil {
		return err
	}
	return validateGenerationGroup(tmpl.Fields, Path{})
}

func validateClassSubclass(tmpl Template) error {
	rule := tmpl.ClassSubclass
	if rule == nil {
		return nil
	}
	class := generatedLeafAt(tmpl.Fields, rule.Class)
	subclass := generatedLeafAt(tmpl.Fields, rule.Subclass)
	level := generatedLeafAt(tmpl.Fields, rule.Level)
	if class == nil || subclass == nil || level == nil {
		return fmt.Errorf("%w: paths must resolve to leaves", ErrClassSubclassInvalid)
	}
	if class.Type != fieldTypeText || subclass.Type != fieldTypeText || level.Type != fieldTypeNumber {
		return fmt.Errorf("%w: class and subclass must be text and level must be a number", ErrClassSubclassInvalid)
	}
	if !class.Generatable || !subclass.Generatable || !level.Generatable {
		return fmt.Errorf("%w: class, subclass and level must be generatable", ErrClassSubclassInvalid)
	}
	if len(class.GenerationEnum) == 0 || len(subclass.GenerationEnum) == 0 {
		return fmt.Errorf("%w: class and subclass need a generation enum", ErrClassSubclassInvalid)
	}
	if rule.SubclassLevel < 1 {
		return fmt.Errorf("%w: subclass level must be positive", ErrClassSubclassInvalid)
	}
	return nil
}

func generatedLeafAt(group FieldGroup, path []string) *FieldSchema {
	if len(path) == 0 {
		return nil
	}
	current := group
	for index, name := range path {
		node, ok := current[name]
		if !ok {
			return nil
		}
		if index == len(path)-1 {
			if !node.IsLeaf() {
				return nil
			}
			return node.Schema
		}
		if !node.IsGroup() {
			return nil
		}
		current = node.Children
	}
	return nil
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
	if err := validateGenerationEnum(schema); err != nil {
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

func validateGenerationEnum(schema *FieldSchema) error {
	if len(schema.GenerationEnum) == 0 {
		return nil
	}
	if !schema.Generatable {
		return ErrGenerationEnumOnNonGeneratable
	}
	if schema.Type != fieldTypeText {
		return ErrGenerationEnumOnNonText
	}
	seen := make(map[string]bool, len(schema.GenerationEnum))
	for _, value := range schema.GenerationEnum {
		if seen[value] {
			return fmt.Errorf("%w: %q", ErrGenerationEnumDuplicate, value)
		}
		seen[value] = true
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
