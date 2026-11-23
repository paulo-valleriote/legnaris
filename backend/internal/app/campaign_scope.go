package app

import (
	"context"
	"errors"
	"slices"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
)

type campaignTemplateCatalog interface {
	FetchByID(id string) (*template.Template, bool)
	TemplateStatesByGameSystem(gameSystemID string) []template.TemplateStateRef
	DraftsAllowed() bool
}

type campaignScopeResolver struct {
	catalog campaignTemplateCatalog
	systems template.GameSystems
}

type campaignScopeGate struct {
	service campaign.Service
}

func newCampaignScopeGate() *campaignScopeGate {
	return &campaignScopeGate{}
}

func (g *campaignScopeGate) bind(service campaign.Service) {
	g.service = service
}

func (g *campaignScopeGate) AllowsTemplate(ctx context.Context, userID, campaignID, templateID string) (bool, error) {
	if g.service == nil {
		return false, apperr.NewError(apperr.CodeInternal, "campaign scope gate is not bound", nil)
	}
	view, err := g.service.Fetch(ctx, campaignID, userID)
	if err != nil {
		if isCampaignAccessError(err) {
			return false, nil
		}
		return false, err
	}
	if view.Status != campaign.StatusActive {
		return false, nil
	}
	return slices.Contains(view.TemplateIDs, templateID), nil
}

func isCampaignAccessError(err error) bool {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		return false
	}
	switch appErr.Code {
	case apperr.CodeCampaignNotFound, apperr.CodeCampaignForbidden, apperr.CodeNotFound, apperr.CodeForbidden:
		return true
	default:
		return false
	}
}

func newCampaignScopeResolver(catalog campaignTemplateCatalog) campaignScopeResolver {
	return campaignScopeResolver{catalog: catalog, systems: template.DefaultGameSystems()}
}

func (r campaignScopeResolver) CreateScope(_ context.Context, gameSystemID, templateID string) (campaign.Scope, error) {
	if templateID != "" {
		return r.createTemplateScope(templateID)
	}
	return r.createGameSystemScope(gameSystemID)
}

func (r campaignScopeResolver) createTemplateScope(templateID string) (campaign.Scope, error) {
	tmpl, ok := r.catalog.FetchByID(templateID)
	if !ok {
		return campaign.Scope{}, campaign.ErrScopeUnknown
	}
	if tmpl.GameSystemID != "" {
		return campaign.Scope{}, campaign.ErrScopeGameSystem
	}
	switch {
	case tmpl.State == template.StateRetired:
		return campaign.Scope{}, campaign.ErrScopeRetired
	case !tmpl.Creatable(r.catalog.DraftsAllowed()):
		return campaign.Scope{}, campaign.ErrScopeDraft
	}
	return campaign.Scope{Name: tmpl.Name, TemplateIDs: []string{tmpl.ID}}, nil
}

func (r campaignScopeResolver) createGameSystemScope(gameSystemID string) (campaign.Scope, error) {
	system, ok := r.systems.Fetch(gameSystemID)
	if !ok {
		return campaign.Scope{}, campaign.ErrScopeUnknown
	}
	ids, creatable := gameSystemTemplateIDs(r.catalog.TemplateStatesByGameSystem(gameSystemID), r.catalog.DraftsAllowed())
	if !creatable {
		return campaign.Scope{}, campaign.ErrScopeNoCreatable
	}
	return campaign.Scope{
		Name:        system.Name,
		TemplateIDs: ids,
		GameSystem:  &campaign.GameSystem{ID: system.ID, Name: system.Name},
	}, nil
}

func gameSystemTemplateIDs(refs []template.TemplateStateRef, draftsAllowed bool) ([]string, bool) {
	ids := make([]string, 0, len(refs))
	creatable := false
	for _, ref := range refs {
		if ref.State.Creatable(draftsAllowed) {
			creatable = true
		}
		if ref.State.Available(draftsAllowed) {
			ids = append(ids, ref.ID)
		}
	}
	return ids, creatable
}

func (r campaignScopeResolver) ResolveScope(_ context.Context, gameSystemID, templateID string) campaign.Scope {
	if templateID != "" {
		return r.storedTemplateScope(templateID)
	}
	return r.storedGameSystemScope(gameSystemID)
}

func (r campaignScopeResolver) storedTemplateScope(templateID string) campaign.Scope {
	if tmpl, ok := r.catalog.FetchByID(templateID); ok {
		return campaign.Scope{Name: tmpl.Name, TemplateIDs: []string{templateID}}
	}
	return campaign.Scope{Name: templateID, TemplateIDs: []string{templateID}}
}

func (r campaignScopeResolver) storedGameSystemScope(gameSystemID string) campaign.Scope {
	if gameSystemID == "" {
		return campaign.Scope{TemplateIDs: []string{}}
	}
	ids, _ := gameSystemTemplateIDs(r.catalog.TemplateStatesByGameSystem(gameSystemID), r.catalog.DraftsAllowed())
	scope := campaign.Scope{
		Name:        gameSystemID,
		TemplateIDs: ids,
		GameSystem:  &campaign.GameSystem{ID: gameSystemID, Name: gameSystemID},
	}
	if system, ok := r.systems.Fetch(gameSystemID); ok {
		scope.Name = system.Name
		scope.GameSystem = &campaign.GameSystem{ID: system.ID, Name: system.Name}
	}
	return scope
}
