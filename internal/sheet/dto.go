package sheet

import "time"

// SheetDTO is the API representation of a Sheet, serialized in snake_case.
type SheetDTO struct {
	ID         string         `json:"id"`
	TemplateID string         `json:"template_id"`
	Name       string         `json:"name"`
	Data       map[string]any `json:"data"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ToDTO converts a Sheet to its API representation.
func (s Sheet) ToDTO() SheetDTO {
	return SheetDTO{
		ID:         s.ID,
		TemplateID: s.TemplateID,
		Name:       s.Name,
		Data:       s.Data,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}
