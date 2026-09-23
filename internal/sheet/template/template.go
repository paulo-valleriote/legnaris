package template

import "time"

type FieldGroup map[string]FieldNode

type FieldNode struct {
	Schema   *FieldSchema `bson:"schema"`
	Children FieldGroup   `bson:"children"`
}

type Template struct {
	ID        string     `bson:"_id,omitempty"`
	Name      string     `bson:"name"`
	Fields    FieldGroup `bson:"fields"`
	Default   bool       `bson:"default"`
	CreatedAt time.Time  `bson:"created_at,omitempty"`
	UpdatedAt time.Time  `bson:"updated_at,omitempty"`
}

type FieldSchema struct {
	Type       string     `bson:"type"`
	Label      string     `bson:"label"`
	Formula    string     `bson:"formula"`
	DependsOn  [][]string `bson:"depends_on"`
	IsEditable bool       `bson:"is_editable"`
	Default    float64    `bson:"default"`
}

func (n *FieldNode) IsLeaf() bool {
	return n.Schema != nil
}

func (n *FieldNode) IsGroup() bool {
	return n.Schema == nil && n.Children != nil
}
