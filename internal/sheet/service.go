package sheet

import (
	"context"
	"errors"
	"legnaris/internal/sheet/template"
	"legnaris/shared/apperr"
	"maps"
	"time"

	"github.com/google/uuid"
)

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
}

type PartialUpdateSheetCommand struct {
	SheetID string         `json:"-"`
	Data    map[string]any `json:"data"`
}

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
	FetchByID(ctx context.Context, id string) (*Sheet, error)
	PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error)
}

type ServiceImpl struct {
	repository      Repository
	templateService template.Service
	dagCache        map[string]*template.DAG
}

func NewService(repository Repository, templateService template.Service, dags map[string]*template.DAG) Service {
	return &ServiceImpl{
		repository:      repository,
		templateService: templateService,
		dagCache:        dags,
	}
}

func (s *ServiceImpl) Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error) {
	dag, exists := s.dagCache[cmd.TemplateID]
	if !exists {
		return nil, errors.New("template not founded")
	}

	tmpl, err := s.templateService.FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: cmd.TemplateID})
	if err != nil {
		return nil, err
	}

	initialData := make(map[string]any)
	for key, field := range tmpl.Fields {
		if field.Formula == "" {
			initialData[key] = field.Default
		}
	}

	derivedEffects, err := dag.Recalculate(make(map[string]any), initialData)
	if err != nil {
		return nil, err
	}

	maps.Copy(initialData, derivedEffects)
	newSheet := Sheet{
		ID:         uuid.NewString(),
		TemplateID: cmd.TemplateID,
		Name:       cmd.SheetName,
		Data:       initialData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repository.Create(ctx, &newSheet); err != nil {
		return nil, err
	}
	return &newSheet, nil
}

func (s *ServiceImpl) FetchByID(ctx context.Context, id string) (*Sheet, error) {
	if id == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, "sheet id not informed", errors.New("sheet id is empty"))
	}

	sheet, err := s.repository.FetchByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return sheet, nil
}

// TODO: Fix partial update peristing changes as map instead of "x.y.z": "val"
func (s *ServiceImpl) PartialUpdate(ctx context.Context, cmd PartialUpdateSheetCommand) (map[string]any, error) {
	storedSheet, err := s.repository.FetchByID(ctx, cmd.SheetID)
	if err != nil {
		return nil, err
	}

	dag, exists := s.dagCache[storedSheet.TemplateID]
	if !exists {
		return nil, errors.New("DAG not found for template")
	}

	for key := range cmd.Data {
		node, exists := dag.Nodes[key]
		if !exists {
			return nil, errors.New("field not exists")
		}
		if !node.IsEditable {
			return nil, errors.New("field is not editable")
		}
	}

	cascadingEffects, err := dag.Recalculate(storedSheet.Data, cmd.Data)
	if err != nil {
		return nil, err
	}

	dbUpdates := make(map[string]any)
	for k, v := range cmd.Data {
		dbUpdates["data."+k] = v
	}
	for k, v := range cascadingEffects {
		dbUpdates["data."+k] = v
	}
	dbUpdates["updated_at"] = time.Now()

	if err := s.repository.PartialUpdate(ctx, cmd.SheetID, dbUpdates); err != nil {
		return nil, err
	}
	return cascadingEffects, nil
}
