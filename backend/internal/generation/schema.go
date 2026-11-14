package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"legnaris/backend/internal/sheet/template"
)

// Field type names of a FieldSchema, as the Template declares them.
const (
	fieldTypeNumber = "number"
	fieldTypeText   = "text"
	fieldTypeBool   = "bool"

	// schemaType is the JSON Schema keyword that names a value kind.
	schemaType = "type"
)

// Schema and output errors. They are plain sentinels: a schema error is a
// Template defect and never reaches a client.
var (
	// ErrNoTemplate reports a nil Template handed to the schema or prompt builders.
	ErrNoTemplate = errors.New("generation: template is nil")
	// ErrNothingToGenerate reports a Template with no generatable leaf.
	ErrNothingToGenerate = errors.New("generation: template has no generatable field")
	// ErrUnsupportedType reports a generatable leaf of a type the schema cannot express.
	ErrUnsupportedType = errors.New("generation: generatable field has an unsupported type")
	// ErrPropertyCollision reports an exclusive group whose name equals a sibling key.
	ErrPropertyCollision = errors.New("generation: exclusive group name collides with a field")
)

// level is the part of one FieldGroup the generation flow reads: its generatable
// leaves, its child groups, and each exclusive group with its members.
type level struct {
	leaves    map[string]*template.FieldSchema
	groups    map[string]template.FieldGroup
	exclusive map[string][]string
	members   map[string][]string
}

// levelOf splits a FieldGroup into the parts the schema, the prompt and the output
// check handle. Exclusive members are listed in members (every member of the
// group) and in exclusive (the generatable ones only); only the generatable
// members are offered to the model.
func levelOf(group template.FieldGroup) level {
	lv := level{
		leaves:    map[string]*template.FieldSchema{},
		groups:    map[string]template.FieldGroup{},
		exclusive: map[string][]string{},
		members:   map[string][]string{},
	}
	for _, name := range slices.Sorted(maps.Keys(group)) {
		node := group[name]
		switch {
		case node.IsLeaf():
			schema := node.Schema
			if schema.ExclusiveGroup != "" {
				lv.members[schema.ExclusiveGroup] = append(lv.members[schema.ExclusiveGroup], name)
				if schema.Generatable {
					lv.exclusive[schema.ExclusiveGroup] = append(lv.exclusive[schema.ExclusiveGroup], name)
				}
				continue
			}
			if schema.Generatable {
				lv.leaves[name] = schema
			}
		case node.IsGroup():
			if levelOf(node.Children).hasGenerated() {
				lv.groups[name] = node.Children
			}
		}
	}
	return lv
}

// hasGenerated reports whether the level offers the model at least one field. Child
// groups are kept only when they offer one, so the schema and the output check see
// the same tree.
func (lv level) hasGenerated() bool {
	return len(lv.leaves) > 0 || len(lv.exclusive) > 0 || len(lv.groups) > 0
}

// OutputSchema returns the strict JSON Schema the model output must satisfy. It is
// nested like the Template's groups, sets additionalProperties to false, lists
// only the generatable leaves, and turns each exclusive group into one enum
// property holding the generatable member the model picked (ADR 0007).
func OutputSchema(tmpl *template.Template) (json.RawMessage, error) {
	if tmpl == nil {
		return nil, ErrNoTemplate
	}
	obj, err := objectSchema(tmpl.Fields)
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

// objectSchema builds the schema of one group, or nil when the group offers the
// model nothing to fill.
func objectSchema(group template.FieldGroup) (map[string]any, error) {
	lv := levelOf(group)
	props := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(lv.leaves)) {
		prop, err := leafSchema(lv.leaves[name])
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		props[name] = prop
	}
	for _, name := range slices.Sorted(maps.Keys(lv.groups)) {
		child, err := objectSchema(lv.groups[name])
		if err != nil {
			return nil, err
		}
		if child != nil {
			props[name] = child
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

// leafSchema maps one generatable leaf to its JSON Schema property. Bounds are
// carried over only when the Template sets them.
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
		return prop, nil
	case fieldTypeText:
		prop := map[string]any{schemaType: "string"}
		if schema.MaxLength != nil {
			prop["maxLength"] = *schema.MaxLength
		}
		return prop, nil
	case fieldTypeBool:
		return map[string]any{schemaType: "boolean"}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedType, schema.Type)
	}
}
