package template

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
)

type TemplateDTO struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Fields       FieldGroupDTO `json:"fields"`
	Default      bool          `json:"default"`
	SectionOrder []string      `json:"-"`
	Dice         *DiceDTO      `json:"dice,omitempty"`
}

type DiceDTO struct {
	Section map[string]string `json:"section,omitempty"`
	Row     map[string]string `json:"row,omitempty"`
}

type FieldGroupDTO map[string]FieldNodeDTO

type FieldNodeDTO struct {
	Schema   *FieldSchemaDTO `json:"schema,omitempty"`
	Children FieldGroupDTO   `json:"children,omitempty"`
	Label    string          `json:"label,omitempty"`
	Columns  []string        `json:"columns,omitempty"`
	Order    []string        `json:"-"`
}

func (n FieldNodeDTO) MarshalJSON() ([]byte, error) {
	type payload struct {
		Schema   *FieldSchemaDTO `json:"schema,omitempty"`
		Children json.RawMessage `json:"children,omitempty"`
		Label    string          `json:"label,omitempty"`
		Columns  []string        `json:"columns,omitempty"`
	}
	out := payload{Schema: n.Schema, Label: n.Label, Columns: n.Columns}
	if len(n.Children) > 0 {
		children, err := marshalFieldGroup(n.Children, n.Order)
		if err != nil {
			return nil, err
		}
		out.Children = children
	}
	return json.Marshal(out)
}

type FieldSchemaDTO struct {
	Type           string     `json:"type"`
	Label          string     `json:"label"`
	Formula        string     `json:"formula,omitempty"`
	DependsOn      [][]string `json:"depends_on,omitempty"`
	IsEditable     bool       `json:"is_editable"`
	Default        float64    `json:"default"`
	ExclusiveGroup string     `json:"exclusive_group,omitempty"`
	OptionLabel    string     `json:"option_label,omitempty"`
	ColumnLabel    string     `json:"column_label,omitempty"`
	Generatable    bool       `json:"generatable,omitempty"`
	Min            *float64   `json:"min,omitempty"`
	Max            *float64   `json:"max,omitempty"`
	MaxLength      *int       `json:"max_length,omitempty"`
}

func (t Template) ToDTO() TemplateDTO {
	return TemplateDTO{
		Dice:         diceToDTO(t),
		ID:           t.ID,
		Name:         t.Name,
		Fields:       t.Fields.toDTO(),
		Default:      t.Default,
		SectionOrder: slices.Clone(t.SectionOrder),
	}
}

func diceToDTO(t Template) *DiceDTO {
	if len(t.SectionDice) == 0 && len(t.RowDice) == 0 {
		return nil
	}
	return &DiceDTO{Section: maps.Clone(t.SectionDice), Row: maps.Clone(t.RowDice)}
}

func (d TemplateDTO) MarshalJSON() ([]byte, error) {
	type payload struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Fields  json.RawMessage `json:"fields"`
		Default bool            `json:"default"`
		Dice    *DiceDTO        `json:"dice,omitempty"`
	}
	fields, err := marshalFieldGroup(d.Fields, d.SectionOrder)
	if err != nil {
		return nil, err
	}
	return json.Marshal(payload{ID: d.ID, Name: d.Name, Fields: fields, Default: d.Default, Dice: d.Dice})
}

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
		Label:    n.Label,
		Columns:  slices.Clone(n.Columns),
		Order:    slices.Clone(n.Order),
	}
}

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
		ColumnLabel:    s.ColumnLabel,
		Generatable:    s.Generatable,
		Min:            clonePtr(s.Min),
		Max:            clonePtr(s.Max),
		MaxLength:      clonePtr(s.MaxLength),
	}
}
