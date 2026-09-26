package app

import (
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
)

type TemplateModule struct {
	Controller *TemplateController
	Service    template.Service
}

type SheetModule struct {
	Controller *SheetController
	Service    sheet.Service
}

type Modules struct {
	Template TemplateModule
	Sheet    SheetModule
}

func NewModules(database *db.Database, registry *template.Registry) *Modules {
	templateService := template.NewService(registry)
	templateController := NewTemplateController(templateService)

	sheetRepository := sheet.NewRepository(database.Client.Database("legnaris"))
	sheetService := sheet.NewService(sheetRepository, templateService, registry)
	sheetController := NewSheetController(sheetService)

	return &Modules{
		Template: TemplateModule{
			Controller: templateController,
			Service:    templateService,
		},
		Sheet: SheetModule{
			Controller: sheetController,
			Service:    sheetService,
		},
	}
}
