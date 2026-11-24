package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/template"
)

const (
	fieldTypeNumber = "number"
	fieldTypeText   = "text"
	fieldTypeBool   = "bool"

	schemaType = "type"
)

var (
	ErrNoTemplate        = errors.New("generation: template is nil")
	ErrNothingToGenerate = errors.New("generation: template has no generatable field")
	ErrUnsupportedType   = errors.New("generation: generatable field has an unsupported type")
	ErrPropertyCollision = errors.New("generation: exclusive group name collides with a field")
)

type level struct {
	leaves    map[string]*template.FieldSchema
	groups    map[string]template.FieldGroup
	exclusive map[string][]string
	lists     map[string]*template.ListSchema
}

func newLevel() level {
	return level{
		leaves:    map[string]*template.FieldSchema{},
		groups:    map[string]template.FieldGroup{},
		exclusive: map[string][]string{},
		lists:     map[string]*template.ListSchema{},
	}
}

func levelOf(group template.FieldGroup, kind plan.SheetKind) level {
	lv := newLevel()
	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		switch {
		case node.IsLeaf():
			schema := node.Schema
			if schema.ExclusiveGroup != "" {
				if schema.GeneratableFor(kind) {
					lv.exclusive[schema.ExclusiveGroup] = append(lv.exclusive[schema.ExclusiveGroup], name)
				}
				continue
			}
			if schema.GeneratableFor(kind) {
				lv.leaves[name] = schema
			}
		case node.IsGroup():
			if levelOf(node.Children, kind).hasGenerated() {
				lv.groups[name] = node.Children
			}
		case node.IsList():
			if node.List != nil && node.List.MaxRecords > 0 && shapeLevel(node.List.Shape).hasGenerated() {
				lv.lists[name] = node.List
			}
		}
	}
	return lv
}

func shapeLevel(shape template.FieldGroup) level {
	lv := newLevel()
	for _, name := range slices.Sorted(maps.Keys(shape)) {
		node := shape[name]
		if !node.IsLeaf() || node.Schema == nil || !node.Schema.IsEditable || node.Schema.Formula != "" {
			continue
		}
		if node.Schema.ExclusiveGroup != "" {
			lv.exclusive[node.Schema.ExclusiveGroup] = append(lv.exclusive[node.Schema.ExclusiveGroup], name)
			continue
		}
		lv.leaves[name] = node.Schema
	}
	return lv
}

func (lv level) hasGenerated() bool {
	return len(lv.leaves) > 0 || len(lv.exclusive) > 0 || len(lv.groups) > 0 || len(lv.lists) > 0
}

func OutputSchema(tmpl *template.Template, kind plan.SheetKind) (json.RawMessage, error) {
	if tmpl == nil {
		return nil, ErrNoTemplate
	}
	obj, err := objectSchema(tmpl.Fields, kind)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrNothingToGenerate
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode output schema: %w", err)
	}
	return raw, nil
}

func objectSchema(group template.FieldGroup, kind plan.SheetKind) (map[string]any, error) {
	return schemaFromLevel(levelOf(group, kind), kind)
}

func schemaFromLevel(lv level, kind plan.SheetKind) (map[string]any, error) {
	props := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(lv.leaves)) {
		prop, err := leafSchema(lv.leaves[name])
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		props[name] = prop
	}
	for _, name := range slices.Sorted(maps.Keys(lv.groups)) {
		child, err := objectSchema(lv.groups[name], kind)
		if err != nil {
			return nil, err
		}
		if child != nil {
			props[name] = child
		}
	}
	for _, name := range slices.Sorted(maps.Keys(lv.lists)) {
		items, err := recordSchema(lv.lists[name].Shape)
		if err != nil {
			return nil, fmt.Errorf("list %q: %w", name, err)
		}
		if items == nil {
			continue
		}
		props[name] = map[string]any{
			schemaType: "array",
			"maxItems": lv.lists[name].MaxRecords,
			"items":    items,
		}
	}
	for _, group := range slices.Sorted(maps.Keys(lv.exclusive)) {
		if _, taken := props[group]; taken {
			return nil, fmt.Errorf("%w: %q", ErrPropertyCollision, group)
		}
		props[group] = map[string]any{schemaType: "string", "enum": lv.exclusive[group]}
	}
	if len(props) == 0 {
		return nil, nil
	}
	return map[string]any{
		schemaType:             "object",
		"properties":           props,
		"required":             slices.Sorted(maps.Keys(props)),
		"additionalProperties": false,
	}, nil
}

func recordSchema(shape template.FieldGroup) (map[string]any, error) {
	lv := shapeLevel(shape)
	if !lv.hasGenerated() {
		return nil, nil
	}
	return schemaFromLevel(lv, "")
}

func leafSchema(schema *template.FieldSchema) (map[string]any, error) {
	switch schema.Type {
	case fieldTypeNumber:
		prop := map[string]any{schemaType: "number"}
		if schema.Min != nil {
			prop["minimum"] = *schema.Min
		}
		if schema.Max != nil {
			prop["maximum"] = *schema.Max
		}
		if schema.Flag {
			prop["enum"] = []any{0, 1}
		}
		return prop, nil
	case fieldTypeText:
		prop := map[string]any{schemaType: "string"}
		if schema.MaxLength != nil {
			prop["maxLength"] = *schema.MaxLength
		}
		if len(schema.GenerationEnum) > 0 {
			prop["enum"] = schema.GenerationEnum
		}
		return prop, nil
	case fieldTypeBool:
		return map[string]any{schemaType: "boolean"}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedType, schema.Type)
	}
}
