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

type FetchTemplateByIDCommand struct {
	ID string `json:"id"`
}

func (s *ServiceImpl) List(ctx context.Context) ([]Template, error) {
	return s.registry.List(), nil
}

// FetchTemplate returns the template for cmd.ID, the single place that decides
// whether an ID was given at all (LGN-66): controllers forward the path param
// and translate ErrTemplateIDNotProvided at the HTTP boundary.
func (s *ServiceImpl) FetchTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error) {
	if cmd == nil || cmd.ID == "" {
		return nil, ErrTemplateIDNotProvided
	}

	tmpl, ok := s.registry.FetchByID(cmd.ID)
	if !ok {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	return tmpl, nil
}
