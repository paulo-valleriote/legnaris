package template

import "time"

type Template struct {
	ID        string                 `bson:"_id,omitempty"`
	Name      string                 `bson:"name"`
	Fields    map[string]FieldSchema `bson:"fields"`
	CreatedAt time.Time              `bson:"created_at,omitempty"`
	UpdatedAt time.Time              `bson:"updated_at,omitempty"`
}

type FieldSchema struct {
	Type       string   `bson:"type"`
	Label      string   `bson:"label"`
	Formula    string   `bson:"formula"`
	DependsOn  []string `bson:"depends_on"`
	IsEditable bool     `bson:"is_editable"`
	Default    float64  `bson:"default"`
}
