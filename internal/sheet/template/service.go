package template

import (
	"context"

	"legnaris/shared/apperr"
)

type Service interface {
	List(ctx context.Context) ([]Template, error)
	FetchTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error)
}

type ServiceImpl struct {
	registry *Registry
}

func NewService(registry *Registry) Service {
	return &ServiceImpl{
		registry: registry,
	}
}

type CreateTemplateCommand struct {
	Name string `json:"name"`
}

type FetchTemplateByIDCommand struct {
	ID string `json:"id"`
}

func (s *ServiceImpl) List(ctx context.Context) ([]Template, error) {
	return s.registry.List(), nil
}

func (s *ServiceImpl) FetchTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error) {
	tmpl, ok := s.registry.FetchByID(cmd.ID)
	if !ok {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	return tmpl, nil
}
