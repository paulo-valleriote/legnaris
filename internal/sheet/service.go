package sheet

import (
	"context"
	"errors"
	"legnaris/internal/sheet/template"
	"maps"
	"time"
)

type Service interface {
	Create(ctx context.Context, cmd CreateSheetCommand) (*Sheet, error)
}

type CreateSheetCommand struct {
	TemplateID string `json:"template_id"`
	SheetName  string `json:"sheet_name"`
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
