package sheet

import (
	"slices"
	"time"

	"legnaris/backend/internal/plan"
)

type Sheet struct {
	ID                 string         `bson:"_id,omitempty"`
	TemplateID         string         `bson:"template_id"`
	OwnerID            string         `bson:"owner_id"`
	Kind               plan.SheetKind `bson:"kind"`
	Name               string         `bson:"name"`
	Data               map[string]any `bson:"data"`
	Layout             *Layout        `bson:"layout,omitempty"`
	Dice               *Dice          `bson:"dice,omitempty"`
	Version            int64          `bson:"version"`
	LayoutVersion      int64          `bson:"layout_version,omitempty"`
	LastResetRequestID string         `bson:"last_reset_request_id,omitempty"`
	CampaignID         *string        `bson:"campaign_id,omitempty"`
	CreatedAt          time.Time      `bson:"created_at,omitempty"`
	UpdatedAt          time.Time      `bson:"updated_at,omitempty"`
}

type Layout struct {
	SectionOrder []string `bson:"section_order,omitempty" json:"section_order,omitempty"`
	Pinned       []string `bson:"pinned,omitempty" json:"pinned,omitempty"`
}

func (l *Layout) IsEmpty() bool {
	return l == nil || (len(l.SectionOrder) == 0 && len(l.Pinned) == 0)
}

func (l *Layout) Clone() *Layout {
	if l == nil {
		return nil
	}
	return &Layout{
		SectionOrder: slices.Clone(l.SectionOrder),
		Pinned:       slices.Clone(l.Pinned),
	}
}

type Dice struct {
	Section map[string]*string `bson:"section,omitempty" json:"section"`
	Row     map[string]*string `bson:"row,omitempty" json:"row"`
}

func (d *Dice) IsEmpty() bool {
	return d == nil || (len(d.Section) == 0 && len(d.Row) == 0)
}

func (d *Dice) Clone() *Dice {
	if d == nil {
		return nil
	}
	return &Dice{Section: cloneOverrides(d.Section), Row: cloneOverrides(d.Row)}
}

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
