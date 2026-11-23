package campaign

import (
	"context"
	"errors"
	"slices"

	"legnaris/backend/shared/apperr"
)

type GameSystem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Scope struct {
	Name        string
	TemplateIDs []string
	GameSystem  *GameSystem
}

type ScopeResolver interface {
	CreateScope(ctx context.Context, gameSystemID, templateID string) (Scope, error)
	ResolveScope(ctx context.Context, gameSystemID, templateID string) Scope
}

var (
	ErrScopeUnknown     = errors.New("campaign scope is unknown")
	ErrScopeRetired     = errors.New("campaign scope template is retired")
	ErrScopeDraft       = errors.New("campaign scope template is a draft")
	ErrScopeGameSystem  = errors.New("campaign scope template belongs to a game system")
	ErrScopeNoCreatable = errors.New("campaign game system has no creatable template")
)

var errNoScopeResolver = errors.New("campaign scope resolver is not configured")

const (
	MsgScopeRequired    = "exactly one of game_system or template_id is required"
	MsgScopeUnknown     = "game system or template not found"
	MsgScopeRetired     = "template is retired and cannot start a campaign"
	MsgScopeDraft       = "template is a draft and is not available outside development"
	MsgScopeGameSystem  = "template belongs to a game system; choose the game system instead"
	MsgScopeNoCreatable = "game system has no template that can start a campaign"
)

type noScopeResolver struct{}

func (noScopeResolver) CreateScope(_ context.Context, _, _ string) (Scope, error) {
	return Scope{}, errNoScopeResolver
}

func (noScopeResolver) ResolveScope(_ context.Context, gameSystemID, templateID string) Scope {
	return storedScope(gameSystemID, templateID)
}

func storedScope(gameSystemID, templateID string) Scope {
	if templateID != "" {
		return Scope{Name: templateID, TemplateIDs: []string{templateID}}
	}
	if gameSystemID == "" {
		return Scope{TemplateIDs: []string{}}
	}
	return Scope{
		Name:        gameSystemID,
		TemplateIDs: []string{},
		GameSystem:  &GameSystem{ID: gameSystemID, Name: gameSystemID},
	}
}

func (s *ServiceImpl) scopeOf(ctx context.Context, c *Campaign) Scope {
	return s.scopes.ResolveScope(ctx, c.GameSystemID, c.TemplateID)
}

func templateIDsOf(scope Scope) []string {
	if scope.TemplateIDs == nil {
		return []string{}
	}
	return scope.TemplateIDs
}

func (s *ServiceImpl) allowsTemplate(ctx context.Context, c *Campaign, templateID string) bool {
	return slices.Contains(s.scopeOf(ctx, c).TemplateIDs, templateID)
}

func scopeEventData(c Campaign) map[string]any {
	if c.GameSystemID != "" {
		return map[string]any{"game_system": c.GameSystemID}
	}
	return map[string]any{"template_id": c.TemplateID}
}

func requireOneScope(gameSystemID, templateID string) error {
	if (gameSystemID != "") == (templateID != "") {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeRequired, nil)
	}
	return nil
}

func createScopeError(err error) error {
	switch {
	case errors.Is(err, ErrScopeUnknown):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeUnknown, err)
	case errors.Is(err, ErrScopeRetired):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeRetired, err)
	case errors.Is(err, ErrScopeDraft):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeDraft, err)
	case errors.Is(err, ErrScopeGameSystem):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeGameSystem, err)
	case errors.Is(err, ErrScopeNoCreatable):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgScopeNoCreatable, err)
	default:
		return internalError(MsgCreateFailed, err)
	}
}
