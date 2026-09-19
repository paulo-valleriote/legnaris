package sheet

import (
	"time"
)

type Sheet struct {
	ID string `bson:"_id,omitempty"`
	//OwnerID             string
	TemplateID string         `bson:"template_id"`
	Name       string         `bson:"name"`
	Data       map[string]any `bson:"data"`
	CreatedAt  time.Time      `bson:"created_at,omitempty"`
	UpdatedAt  time.Time      `bson:"updated_at,omitempty"`
}
