package template

// TemplateDTO is the API representation of a Template, serialized in snake_case.
type TemplateDTO struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Fields  FieldGroupDTO `json:"fields"`
	Default bool          `json:"default"`
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
	Type       string     `json:"type"`
	Label      string     `json:"label"`
	Formula    string     `json:"formula,omitempty"`
	DependsOn  [][]string `json:"depends_on,omitempty"`
	IsEditable bool       `json:"is_editable"`
	Default    float64    `json:"default"`
}

// ToDTO converts a Template to its API representation.
func (t Template) ToDTO() TemplateDTO {
	return TemplateDTO{
		ID:      t.ID,
		Name:    t.Name,
		Fields:  t.Fields.toDTO(),
		Default: t.Default,
	}
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

func (s *FieldSchema) toDTO() *FieldSchemaDTO {
	if s == nil {
		return nil
	}

	return &FieldSchemaDTO{
		Type:       s.Type,
		Label:      s.Label,
		Formula:    s.Formula,
		DependsOn:  s.DependsOn,
		IsEditable: s.IsEditable,
		Default:    s.Default,
	}
}
