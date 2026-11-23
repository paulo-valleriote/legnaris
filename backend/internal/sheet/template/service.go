package template

import (
	"context"
	"regexp"
)

type Service interface {
	List(ctx context.Context) ([]Template, error)
	FetchTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error)
	FetchCatalogTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error)
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
	return s.fetch(cmd)
}

func (s *ServiceImpl) FetchCatalogTemplate(ctx context.Context, cmd *FetchTemplateByIDCommand) (*Template, error) {
	tmpl, err := s.fetch(cmd)
	if err != nil {
		return nil, err
	}
	if !tmpl.Available(s.registry.DraftsAllowed()) {
		return nil, ErrTemplateNotFound
	}
	return tmpl, nil
}

func (s *ServiceImpl) fetch(cmd *FetchTemplateByIDCommand) (*Template, error) {
	if cmd == nil || cmd.ID == "" {
		return nil, ErrTemplateIDNotProvided
	}
	if !templateIDRe.MatchString(cmd.ID) {
		return nil, ErrTemplateIDInvalid
	}

	tmpl, ok := s.registry.FetchByID(cmd.ID)
	if !ok {
		return nil, ErrTemplateNotFound
	}

	return tmpl, nil
}
