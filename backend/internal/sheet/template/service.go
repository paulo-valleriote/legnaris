package template

import (
	"context"
	"regexp"

	"legnaris/backend/shared/apperr"
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

var templateIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (s *ServiceImpl) List(ctx context.Context) ([]Template, error) {
	return s.registry.List(), nil
}

func (s *ServiceImpl) FetchTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error) {
	if cmd == nil || cmd.ID == "" {
		return nil, ErrTemplateIDNotProvided
	}
	if !templateIDRe.MatchString(cmd.ID) {
		return nil, ErrTemplateIDInvalid
	}

	tmpl, ok := s.registry.FetchByID(cmd.ID)
	if !ok {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", nil)
	}

	return tmpl, nil
}
