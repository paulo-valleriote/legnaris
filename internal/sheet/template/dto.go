package template

import (
	"bytes"
	"encoding/json"
	"slices"
)

// TemplateDTO is the API representation of a Template, serialized in snake_case.
type TemplateDTO struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Fields  FieldGroupDTO `json:"fields"`
	Default bool          `json:"default"`
	// SectionOrder is the template's declared display order for top-level
	// fields. It drives MarshalJSON and never appears in the payload.
	SectionOrder []string `json:"-"`
}

// FieldGroupDTO mirrors FieldGroup (a map of field name → node) for the API.
type FieldGroupDTO map[string]FieldNodeDTO

// FieldNodeDTO mirrors FieldNode. A leaf node exposes only "schema"; a group
// node exposes only "children".
type FieldNodeDTO struct {
	Schema   *FieldSchemaDTO `json:"schema,omitempty"`
	Children FieldGroupDTO   `json:"children,omitempty"`
}

// FieldSchemaDTO mirrors FieldSchema for the API.
type FieldSchemaDTO struct {
	Type           string     `json:"type"`
	Label          string     `json:"label"`
	Formula        string     `json:"formula,omitempty"`
	DependsOn      [][]string `json:"depends_on,omitempty"`
	IsEditable     bool       `json:"is_editable"`
	Default        float64    `json:"default"`
	ExclusiveGroup string     `json:"exclusive_group,omitempty"`
	OptionLabel    string     `json:"option_label,omitempty"`
}

// ToDTO converts a Template to its API representation. The result shares no
// map or slice with the template: the field tree is rebuilt level by level,
// DependsOn is cloned (LGN-60) and SectionOrder is copied for the same reason.
func (t Template) ToDTO() TemplateDTO {
	return TemplateDTO{
		ID:           t.ID,
		Name:         t.Name,
		Fields:       t.Fields.toDTO(),
		Default:      t.Default,
		SectionOrder: slices.Clone(t.SectionOrder),
	}
}

// MarshalJSON emits the template with its top-level fields in SectionOrder
// instead of the alphabetical order encoding/json forces on maps: the sheet
// renders sections exactly as they arrive (identity and stats first, the
// tuning machinery last). A listed key missing from the map is skipped, and
// keys not listed — or a DTO with no order at all — fall back to alphabetical.
func (d TemplateDTO) MarshalJSON() ([]byte, error) {
	type payload struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Fields  json.RawMessage `json:"fields"`
		Default bool            `json:"default"`
	}
	fields, err := marshalFieldGroup(d.Fields, d.SectionOrder)
	if err != nil {
		return nil, err
	}
	return json.Marshal(payload{ID: d.ID, Name: d.Name, Fields: fields, Default: d.Default})
}

// marshalFieldGroup writes one fields object with the keys of order first (in
// that sequence, deduplicated) and every remaining key alphabetically after
// them. Nested objects marshal as plain maps, so they stay alphabetical.
func marshalFieldGroup(group FieldGroupDTO, order []string) (json.RawMessage, error) {
	keys := make([]string, 0, len(group))
	emitted := make(map[string]bool, len(group))
	for _, key := range order {
		if _, ok := group[key]; ok && !emitted[key] {
			keys = append(keys, key)
			emitted[key] = true
		}
	}
	rest := make([]string, 0, len(group)-len(keys))
	for key := range group {
		if !emitted[key] {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)
	keys = append(keys, rest...)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(encodedKey)
		buf.WriteByte(':')
		encodedValue, err := json.Marshal(group[key])
		if err != nil {
			return nil, err
		}
		buf.Write(encodedValue)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (g FieldGroup) toDTO() FieldGroupDTO {
	if len(g) == 0 {
		return FieldGroupDTO{}
	}

	dto := make(FieldGroupDTO, len(g))
	for name, node := range g {
		dto[name] = node.toDTO()
	}
	return dto
}

func (n FieldNode) toDTO() FieldNodeDTO {
	return FieldNodeDTO{
		Schema:   n.Schema.toDTO(),
		Children: n.Children.toDTO(),
	}
}

// toDTO converts a FieldSchema to its API representation, cloning DependsOn:
// the DTO escapes the package while the schema behind it belongs to a
// registered template (LGN-60). Every other field is an immutable scalar or a
// string, and the struct itself is a fresh value.
func (s *FieldSchema) toDTO() *FieldSchemaDTO {
	if s == nil {
		return nil
	}

	return &FieldSchemaDTO{
		Type:           s.Type,
		Label:          s.Label,
		Formula:        s.Formula,
		DependsOn:      cloneDependsOn(s.DependsOn),
		IsEditable:     s.IsEditable,
		Default:        s.Default,
		ExclusiveGroup: s.ExclusiveGroup,
		OptionLabel:    s.OptionLabel,
	}
}
