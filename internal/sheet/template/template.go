package template

import "time"

type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	Status      string    `json:"status"`
	Visibility  string    `json:"visibility"`
	VersionID   string    `json:"current_version_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TemplateVersion struct {
	ID            string    `json:"id"`
	TemplateID    string    `json:"template_id"`
	Version       int       `json:"version"`
	SchemaVersion int       `json:"schema_version"`
	Schema        string    `json:"schema"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
