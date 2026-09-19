package app

import (
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
)

type Controllers struct {
	Template *TemplateController
	Sheet    *SheetController
}

func NewControllers(database *db.Database, dagRegistry map[string]*template.DAG) *Controllers {
	templateRepository := template.NewRepository(database.Client.Database("legnaris"))
	templateService := template.NewService(templateRepository)
	templateController := NewTemplateController(templateService)

	sheetRepository := sheet.NewRepository(database.Client.Database("legnaris"))
	sheetService := sheet.NewService(sheetRepository, *templateService, dagRegistry)
	sheetController := NewSheetController(sheetService)

	return &Controllers{
		Template: templateController,
		Sheet:    sheetController,
	}
}
