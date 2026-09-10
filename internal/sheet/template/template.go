package template

import "time"

type Template struct {
	ID          string
	Name        string
	Description string
	OwnerID     string
	Status      string
	Visibility  string
	VersionID   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TemplateVersion struct {
	ID            string
	TemplateID    string
	Version       int
	SchemaVersion int
	Schema        string
	Status        string
	CreatedAt     time.Time
}
