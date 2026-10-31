package template

import "time"

type FieldGroup map[string]FieldNode

type FieldNode struct {
	Schema   *FieldSchema `bson:"schema"`
	Children FieldGroup   `bson:"children"`
	// Label is the display name of a group node (table row header or section).
	// Empty falls back to the humanized key, e.g. slot1 -> "Level 1".
	Label string `bson:"label,omitempty"`
	// Columns lists the child keys rendered as ONE table row, in column
	// order. Empty renders the children as stacked rows (the default).
	Columns []string `bson:"columns,omitempty"`
}

func (n *FieldNode) IsLeaf() bool {
	return n.Schema != nil
}

func (n *FieldNode) IsGroup() bool {
	return n.Schema == nil && n.Children != nil
}

type Template struct {
	ID      string     `bson:"_id,omitempty"`
	Name    string     `bson:"name"`
	Fields  FieldGroup `bson:"fields"`
	Default bool       `bson:"default"`
	// SectionOrder lists top-level field keys in the order the API must emit
	// them: the sheet renders sections in arrival order, while encoding/json
	// would sort a map's keys alphabetically. Keys not listed — and templates
	// with no order at all — keep the alphabetical fallback.
	SectionOrder []string `bson:"section_order,omitempty"`
	// SuggestedPins lists Row keys the template recommends as a player's
	// first Pins, in suggestion order. Presentation metadata only: the server
	// never writes it into a Sheet or its Layout, a client offers it. Keys use
	// the Layout grammar and are checked by ValidateSuggestedPins at boot.
	SuggestedPins []string  `bson:"suggested_pins,omitempty"`
	CreatedAt     time.Time `bson:"created_at,omitempty"`
	UpdatedAt     time.Time `bson:"updated_at,omitempty"`
}

type FieldSchema struct {
	Type       string     `bson:"type"`
	Label      string     `bson:"label"`
	Formula    string     `bson:"formula"`
	DependsOn  [][]string `bson:"depends_on"`
	IsEditable bool       `bson:"is_editable"`
	Default    float64    `bson:"default"`
	// ExclusiveGroup names the mutually exclusive set this leaf belongs to.
	// Leaves sharing a non-empty group under the same parent may have at most
	// one non-zero value. Empty means "independent".
	ExclusiveGroup string `bson:"exclusive_group,omitempty"`
	// OptionLabel is the short label used when this leaf is shown as an option
	// of its exclusive group (e.g. "Dexterity"). Empty falls back to Label.
	OptionLabel string `bson:"option_label,omitempty"`
	// ColumnLabel is the short label used when this leaf is rendered as a
	// table column. Empty falls back to Label, then to the humanized key.
	ColumnLabel string `bson:"column_label,omitempty"`
}

// SeedValue returns the value a new sheet starts with for this schema:
// text fields start empty, booleans start false, and every other type starts
// at the numeric Default. The split is type-driven because Default is a
// float64 in the API contract — the JSON `default` key never carries a
// string or bool (clients decode it as a number).
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
