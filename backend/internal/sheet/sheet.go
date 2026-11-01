package sheet

import (
	"slices"
	"time"
)

type Sheet struct {
	ID         string         `bson:"_id,omitempty"`
	TemplateID string         `bson:"template_id"`
	OwnerID    string         `bson:"owner_id"`
	Name       string         `bson:"name"`
	Data       map[string]any `bson:"data"`
	Layout     *Layout        `bson:"layout,omitempty"`
	// Version counts the data writes persisted on this Sheet. A document that
	// predates the field reads as version 0. PATCH uses it as a compare-and-swap
	// guard, so it is never exposed to clients.
	Version   int64     `bson:"version"`
	CreatedAt time.Time `bson:"created_at,omitempty"`
	UpdatedAt time.Time `bson:"updated_at,omitempty"`
}

// Layout is the player's chosen arrangement of a Sheet: the order of Sections
// and which Rows are pinned (in pin order). Rows are never reordered; a pinned
// Row rises to the top of its own Section. It is user data stored on the Sheet;
// the Template only supplies the default. Both lists may be partial — keys they
// omit fall back to Template order on read — and Rows are referenced by the
// keys template.SectionRowKeys derives. The same struct is the stored and the
// wire shape.
type Layout struct {
	SectionOrder []string `bson:"section_order,omitempty" json:"section_order,omitempty"`
	Pinned       []string `bson:"pinned,omitempty" json:"pinned,omitempty"`
}

// IsEmpty reports whether the Layout carries no ordering at all, which is what
// "Reset to default" looks like. A nil Layout is empty.
func (l *Layout) IsEmpty() bool {
	return l == nil || (len(l.SectionOrder) == 0 && len(l.Pinned) == 0)
}

// Clone returns a deep copy, so a Layout that crosses the service boundary
// never aliases the one it came from. A nil Layout clones to nil.
func (l *Layout) Clone() *Layout {
	if l == nil {
		return nil
	}
	return &Layout{
		SectionOrder: slices.Clone(l.SectionOrder),
		Pinned:       slices.Clone(l.Pinned),
	}
}
