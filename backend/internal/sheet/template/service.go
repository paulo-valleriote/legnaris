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

// templateIDRe is the slug shape code-defined template IDs use (fantasy_v1).
// Anything else dies with a 400 before the registry lookup instead of
// surfacing as a 404, mirroring the sheet-ID check (LGN-86).
var templateIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (s *ServiceImpl) List(ctx context.Context) ([]Template, error) {
	return s.registry.List(), nil
}

// FetchTemplate returns the template for cmd.ID, the single place that decides
// whether an ID was given at all (LGN-66) and whether it has the shape a
// template ID must have (LGN-86): controllers forward the path param and
// translate the typed errors at the HTTP boundary.
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
