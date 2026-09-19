package app

import (
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
)

type Template struct {
	Controller *TemplateController
	Service    *template.Service
}

type Sheet struct {
	Controller *SheetController
	Service    *sheet.Service
}

type Modules struct {
	Template Template
	Sheet    Sheet
}

func NewModules(database *db.Database, dagRegistry map[string]*template.DAG) *Modules {
	templateRepository := template.NewRepository(database.Client.Database("legnaris"))
	templateService := template.NewService(templateRepository)
	templateController := NewTemplateController(templateService)

	sheetRepository := sheet.NewRepository(database.Client.Database("legnaris"))
	sheetService := sheet.NewService(sheetRepository, *templateService, dagRegistry)
	sheetController := NewSheetController(sheetService)

	return &Modules{
		Template: Template{
			Controller: templateController,
			Service:    templateService,
		},
		Sheet: Sheet{
			Controller: sheetController,
			Service:    sheetService,
		},
	}
}
