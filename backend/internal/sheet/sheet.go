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
	Dice       *Dice          `bson:"dice,omitempty"`
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

// Dice is the player's Dice expression choices on a Sheet. Section maps a
// Section key and Row maps a Row key to an expression that replaces the
// Template's default. A nil value is an explicit "no dice" and is different
// from an absent key, which means "use the Template". It is user data stored on
// the Sheet; the Template only supplies defaults. The same struct is the stored
// and the wire shape.
type Dice struct {
	Section map[string]*string `bson:"section,omitempty" json:"section"`
	Row     map[string]*string `bson:"row,omitempty" json:"row"`
}

// IsEmpty reports whether the block holds no entry at all, which is what
// "clear everything" looks like. A null entry is a choice, so it is not empty.
// A nil Dice is empty.
func (d *Dice) IsEmpty() bool {
	return d == nil || (len(d.Section) == 0 && len(d.Row) == 0)
}

// Clone returns a deep copy, so a block that crosses the service boundary
// never aliases the one it came from. A nil Dice clones to nil.
func (d *Dice) Clone() *Dice {
	if d == nil {
		return nil
	}
	return &Dice{Section: cloneOverrides(d.Section), Row: cloneOverrides(d.Row)}
}

// Block returns a deep copy with both maps non-nil, the shape every response
// carries: a Sheet that never had dice reads as an empty block, not as a
// missing one. It is safe on a nil Dice.
func (d *Dice) Block() Dice {
	if d == nil {
		return Dice{Section: map[string]*string{}, Row: map[string]*string{}}
	}
	return Dice{Section: nonNilOverrides(d.Section), Row: nonNilOverrides(d.Row)}
}

func nonNilOverrides(overrides map[string]*string) map[string]*string {
	if cloned := cloneOverrides(overrides); cloned != nil {
		return cloned
	}
	return map[string]*string{}
}

func cloneOverrides(overrides map[string]*string) map[string]*string {
	if overrides == nil {
		return nil
	}
	cloned := make(map[string]*string, len(overrides))
	for key, expr := range overrides {
		if expr == nil {
			cloned[key] = nil
			continue
		}
		copied := *expr
		cloned[key] = &copied
	}
	return cloned
}
