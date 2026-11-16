package template

import (
	"slices"
	"time"

	"legnaris/backend/internal/plan"
)

func (t *Template) AcceptsKind(kind plan.SheetKind) bool {
	return kind != "" && slices.Contains(t.Kinds, kind)
}

type FieldGroup map[string]FieldNode

type FieldNode struct {
	Schema   *FieldSchema `bson:"schema"`
	Children FieldGroup   `bson:"children"`
	Label    string       `bson:"label,omitempty"`
	Columns  []string     `bson:"columns,omitempty"`
	Order    []string     `bson:"order,omitempty"`
}

func (n *FieldNode) IsLeaf() bool {
	return n.Schema != nil
}

func (n *FieldNode) IsGroup() bool {
	return n.Schema == nil && n.Children != nil
}

type Template struct {
	ID            string            `bson:"_id,omitempty"`
	Name          string            `bson:"name"`
	Fields        FieldGroup        `bson:"fields"`
	Default       bool              `bson:"default"`
	SectionOrder  []string          `bson:"section_order,omitempty"`
	SuggestedPins []string          `bson:"suggested_pins,omitempty"`
	NameLeaf      string            `bson:"name_leaf,omitempty"`
	SectionDice   map[string]string `bson:"section_dice,omitempty"`
	RowDice       map[string]string `bson:"row_dice,omitempty"`
	Kinds         []plan.SheetKind  `bson:"kinds,omitempty"`
	CreatedAt     time.Time         `bson:"created_at,omitempty"`
	UpdatedAt     time.Time         `bson:"updated_at,omitempty"`
}

type FieldSchema struct {
	Type           string     `bson:"type"`
	Label          string     `bson:"label"`
	Formula        string     `bson:"formula"`
	DependsOn      [][]string `bson:"depends_on"`
	IsEditable     bool       `bson:"is_editable"`
	Default        float64    `bson:"default"`
	ExclusiveGroup string     `bson:"exclusive_group,omitempty"`
	OptionLabel    string     `bson:"option_label,omitempty"`
	ColumnLabel    string     `bson:"column_label,omitempty"`
	Generatable    bool       `bson:"generatable,omitempty"`
	Min            *float64   `bson:"min,omitempty"`
	Max            *float64   `bson:"max,omitempty"`
	MaxLength      *int       `bson:"max_length,omitempty"`
}

func (s *FieldSchema) SeedValue() any {
	switch s.Type {
	case fieldTypeText:
		return ""
	case fieldTypeBool:
		return false
	default:
		return s.Default
	}
}
