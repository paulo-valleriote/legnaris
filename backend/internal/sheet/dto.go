package sheet

import (
	"time"

	"legnaris/backend/shared/maputils"
)

type SheetDTO struct {
	ID         string         `json:"id"`
	TemplateID string         `json:"template_id"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Data       map[string]any `json:"data"`
	Layout     *Layout        `json:"layout,omitempty"`
	Dice       Dice           `json:"dice"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func (s Sheet) ToDTO() SheetDTO {
	return SheetDTO{
		ID:         s.ID,
		TemplateID: s.TemplateID,
		Kind:       string(s.Kind),
		Name:       s.Name,
		Data:       maputils.DeepCopy(s.Data),
		Layout:     s.Layout.Clone(),
		Dice:       s.Dice.Block(),
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}
