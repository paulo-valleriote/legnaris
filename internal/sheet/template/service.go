package template

import (
	"context"
	"errors"
	"legnaris/shared/apperr"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
}

type CreateTemplateCommand struct {
	Name string `json:"name"`
}

type FetchTemplateByIDCommand struct {
	ID string `json:"id"`
}

func (s *Service) List(ctx context.Context) ([]Template, error) {
	return s.repository.List(ctx)
}

func (s *Service) FetchTemplate(ctx context.Context, cmd FetchTemplateByIDCommand) (*Template, error) {
	template, err := s.repository.FetchByID(ctx, cmd.ID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeNotFound, "template not found", err)
	}

	return template, nil
}

// TODO: Current App verison will only use default templates
//func (s *Service) Create(ctx context.Context, cmd CreateTemplateCommand) error {
//	_, err := s.repository.Create(ctx, FantasyV1Seed)
//	return err
//}

func (s *Service) SeedDefaultTemplates(ctx context.Context, templates []Template) ([]string, error) {
	ids, err := s.repository.CreateMany(ctx, templates)
	if err != nil {
		return []string{}, errors.New("seeders werent executed correctly")
	}
	return ids, nil
}
