package app

import (
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
)

type Controllers struct {
	Template *TemplateController
}

func NewControllers(database *db.Database) *Controllers {
	repository := template.NewRepository(database.Client.Database("legnaris"))
	service := template.NewService(repository)
	controller := NewTemplateController(service)

	return &Controllers{
		Template: controller,
	}
}
