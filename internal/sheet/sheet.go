package sheet

import "time"

type Sheet struct {
	ID                  string    `json:"id"`
	OwnerID             string    `json:"owner_id"`
	TemplateID          string    `json:"template_id"`
	TemplateVersionID   string    `json:"template_version_id"`
	Name                string    `json:"name"`
	Data                string    `json:"data"`
	LifecycleStatus     string    `json:"lifecycle_status"`
	DeletionScheduledAt time.Time `json:"deletion_scheduled_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
