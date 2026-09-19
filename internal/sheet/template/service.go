package template

import (
	"context"
	"legnaris/shared/apperr"
	"time"
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

func (s *Service) Create(ctx context.Context, cmd CreateTemplateCommand) error {
	return s.repository.Create(ctx, Template{
		ID:        "fantasy_v1",
		Name:      cmd.Name,
		Fields:    FantasyV1Seed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}
