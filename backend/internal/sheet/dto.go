package sheet

import (
	"time"

	"legnaris/backend/shared/maputils"
)

// SheetDTO is the API representation of a Sheet, serialized in snake_case.
type SheetDTO struct {
	ID         string         `json:"id"`
	TemplateID string         `json:"template_id"`
	Name       string         `json:"name"`
	Data       map[string]any `json:"data"`
	Layout     *Layout        `json:"layout,omitempty"`
	Dice       Dice           `json:"dice"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ToDTO converts a Sheet to its API representation.
//
// Data is deep-copied so the DTO never aliases the Sheet it came from: the DTO
// escapes the service boundary, where a caller can mutate its map (LGN-60).
// Every other field is copied by value and strings are immutable, so nothing
// else needs a copy, except Layout and Dice, which are cloned. A nil Data and a
// nil Layout stay nil; Dice is always an empty block rather than absent.
func (s Sheet) ToDTO() SheetDTO {
	return SheetDTO{
		ID:         s.ID,
		TemplateID: s.TemplateID,
		Name:       s.Name,
		Data:       maputils.DeepCopy(s.Data),
		Layout:     s.Layout.Clone(),
		Dice:       s.Dice.Block(),
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}
