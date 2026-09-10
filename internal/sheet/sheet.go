package sheet

import "time"

type Sheet struct {
	ID                  string
	OwnerID             string
	TemplateID          string
	TemplateVersionID   string
	Name                string
	Data                string
	LifecycleStatus     string
	DeletionScheduledAt time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
