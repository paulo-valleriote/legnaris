package sheet

type CreateSheetDTO struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
}

type PatchSheetDTO struct {
	SheetData map[string]any `json:"sheet_data"`
}
