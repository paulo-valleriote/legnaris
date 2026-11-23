package app

import (
	"context"
	"slices"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/sheet/template"
)

type campaignTemplateCatalog interface {
	FetchByID(id string) (*template.Template, bool)
	ListByGameSystem(gameSystemID string) []template.Template
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
		return false, nil
	}
	view, err := g.service.Fetch(ctx, campaignID, userID)
	if err != nil || view.Status != campaign.StatusActive {
		return false, nil
	}
	return slices.Contains(view.TemplateIDs, templateID), nil
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
	draftsAllowed := r.catalog.DraftsAllowed()
	templates := r.catalog.ListByGameSystem(gameSystemID)
	creatable := false
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		if tmpl.Creatable(draftsAllowed) {
			creatable = true
		}
		if tmpl.Available(draftsAllowed) {
			ids = append(ids, tmpl.ID)
		}
	}
	if !creatable {
		return campaign.Scope{}, campaign.ErrScopeNoCreatable
	}
	return campaign.Scope{
		Name:        system.Name,
		TemplateIDs: ids,
		GameSystem:  &campaign.GameSystem{ID: system.ID, Name: system.Name},
	}, nil
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
	draftsAllowed := r.catalog.DraftsAllowed()
	templates := r.catalog.ListByGameSystem(gameSystemID)
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		if tmpl.Available(draftsAllowed) {
			ids = append(ids, tmpl.ID)
		}
	}
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
