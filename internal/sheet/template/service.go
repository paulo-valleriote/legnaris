package template

import (
	"context"
	"time"
)

type Service struct {
	repository Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: *repository,
	}
}

type CreateTemplateCommand struct {
	Name string `json:"name"`
}

func (s *Service) List(ctx context.Context) ([]Template, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(ctx context.Context, cmd CreateTemplateCommand) error {
	return s.repository.Create(ctx, Template{
		ID:        "1",
		Name:      cmd.Name,
		Fields:    FantasyV1Seed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}
