package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet/template"
)

type campaignCatalogFixture struct {
	templates []template.Template
	drafts    bool
}

func (f campaignCatalogFixture) FetchByID(id string) (*template.Template, bool) {
	for i := range f.templates {
		if f.templates[i].ID == id {
			cp := f.templates[i]
			return &cp, true
		}
	}
	return nil, false
}

func (f campaignCatalogFixture) TemplateStatesByGameSystem(gameSystemID string) []template.TemplateStateRef {
	found := make([]template.TemplateStateRef, 0, len(f.templates))
	for _, tmpl := range f.templates {
		if tmpl.GameSystemID == gameSystemID {
			found = append(found, template.TemplateStateRef{ID: tmpl.ID, State: tmpl.State})
		}
	}
	return found
}

func (f campaignCatalogFixture) DraftsAllowed() bool {
	return f.drafts
}

var (
	scopeCharV1      = template.Template{ID: "char_v1", Name: "5E Character", State: template.StateActive, GameSystemID: template.GameSystemFifth}
	scopeStatBlockV1 = template.Template{ID: "statblock_v1", Name: "5E Stat Block", State: template.StateActive, GameSystemID: template.GameSystemFifth}
	scopeDraftV1     = template.Template{ID: "draft_v1", Name: "5E Draft", State: template.StateDraft, GameSystemID: template.GameSystemFifth}
	scopeRetiredV1   = template.Template{ID: "retired_v1", Name: "5E Retired", State: template.StateRetired, GameSystemID: template.GameSystemFifth}
	scopeFantasyV1   = template.Template{ID: "fantasy_v1", Name: "Fantasy Template", State: template.StateActive}
)

func mixedCatalog(drafts bool) campaignCatalogFixture {
	return campaignCatalogFixture{
		templates: []template.Template{scopeCharV1, scopeStatBlockV1, scopeDraftV1, scopeRetiredV1, scopeFantasyV1},
		drafts:    drafts,
	}
}

func TestCampaignScopeResolver_CreateScope(t *testing.T) {
	tests := []struct {
		name        string
		catalog     campaignCatalogFixture
		gameSystem  string
		templateID  string
		wantErr     error
		wantName    string
		wantTmplIDs []string
	}{
		{
			name:        "Happy path: an active standalone template",
			catalog:     mixedCatalog(false),
			templateID:  "fantasy_v1",
			wantName:    "Fantasy Template",
			wantTmplIDs: []string{"fantasy_v1"},
		},
		{
			name:        "Happy path: a game system lists its active and retired templates",
			catalog:     mixedCatalog(false),
			gameSystem:  "fifth",
			wantName:    "5E",
			wantTmplIDs: []string{"char_v1", "statblock_v1", "retired_v1"},
		},
		{
			name:        "Happy path: development adds the drafts of a game system",
			catalog:     mixedCatalog(true),
			gameSystem:  "fifth",
			wantName:    "5E",
			wantTmplIDs: []string{"char_v1", "statblock_v1", "draft_v1", "retired_v1"},
		},
		{
			name:       "Happy path: a draft standalone template is creatable in development",
			catalog:    campaignCatalogFixture{templates: []template.Template{{ID: "lonely_draft_v1", Name: "Draft", State: template.StateDraft}}, drafts: true},
			templateID: "lonely_draft_v1",
			wantName:   "Draft",
			wantTmplIDs: []string{
				"lonely_draft_v1",
			},
		},
		{
			name:        "Happy path: a game system whose only template is a draft is creatable in development",
			catalog:     campaignCatalogFixture{templates: []template.Template{scopeDraftV1}, drafts: true},
			gameSystem:  "fifth",
			wantName:    "5E",
			wantTmplIDs: []string{"draft_v1"},
		},
		{
			name:       "Edge: an unknown template is refused",
			catalog:    mixedCatalog(false),
			templateID: "missing_v1",
			wantErr:    campaign.ErrScopeUnknown,
		},
		{
			name:       "Edge: an unknown game system is refused",
			catalog:    mixedCatalog(false),
			gameSystem: "sixth",
			wantErr:    campaign.ErrScopeUnknown,
		},
		{
			name:       "Edge: an empty game system id is refused",
			catalog:    mixedCatalog(false),
			gameSystem: "",
			wantErr:    campaign.ErrScopeUnknown,
		},
		{
			name:       "Error: a template that belongs to a game system is refused",
			catalog:    mixedCatalog(true),
			templateID: "char_v1",
			wantErr:    campaign.ErrScopeGameSystem,
		},
		{
			name:       "Error: a retired standalone template is refused",
			catalog:    campaignCatalogFixture{templates: []template.Template{{ID: "old_v1", Name: "Old", State: template.StateRetired}}, drafts: true},
			templateID: "old_v1",
			wantErr:    campaign.ErrScopeRetired,
		},
		{
			name:       "Error: a draft standalone template is refused outside development",
			catalog:    campaignCatalogFixture{templates: []template.Template{{ID: "new_v1", Name: "New", State: template.StateDraft}}},
			templateID: "new_v1",
			wantErr:    campaign.ErrScopeDraft,
		},
		{
			name:       "Error: a game system whose templates are all retired is refused",
			catalog:    campaignCatalogFixture{templates: []template.Template{scopeRetiredV1}, drafts: true},
			gameSystem: "fifth",
			wantErr:    campaign.ErrScopeNoCreatable,
		},
		{
			name:       "Error: a game system whose templates are all drafts outside development is refused",
			catalog:    campaignCatalogFixture{templates: []template.Template{scopeDraftV1}},
			gameSystem: "fifth",
			wantErr:    campaign.ErrScopeNoCreatable,
		},
		{
			name:       "Error: a game system with no templates is refused",
			catalog:    campaignCatalogFixture{},
			gameSystem: "fifth",
			wantErr:    campaign.ErrScopeNoCreatable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newCampaignScopeResolver(tc.catalog)
			scope, err := resolver.CreateScope(context.Background(), tc.gameSystem, tc.templateID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, scope.Name)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, scope.Name)
			assert.Equal(t, tc.wantTmplIDs, scope.TemplateIDs)
		})
	}
}

func TestCampaignScopeResolver_ResolveScope(t *testing.T) {
	tests := []struct {
		name        string
		catalog     campaignCatalogFixture
		gameSystem  string
		templateID  string
		wantName    string
		wantTmplIDs []string
		wantSystem  *campaign.GameSystem
	}{
		{
			name:        "Happy path: a known template resolves its name",
			catalog:     mixedCatalog(false),
			templateID:  "fantasy_v1",
			wantName:    "Fantasy Template",
			wantTmplIDs: []string{"fantasy_v1"},
		},
		{
			name:        "Edge: an unknown template resolves to its id",
			catalog:     mixedCatalog(false),
			templateID:  "removed_v1",
			wantName:    "removed_v1",
			wantTmplIDs: []string{"removed_v1"},
		},
		{
			name:        "Happy path: a game system resolves its name and available templates",
			catalog:     mixedCatalog(false),
			gameSystem:  "fifth",
			wantName:    "5E",
			wantTmplIDs: []string{"char_v1", "statblock_v1", "retired_v1"},
			wantSystem:  &campaign.GameSystem{ID: "fifth", Name: "5E"},
		},
		{
			name:        "Happy path: development adds the drafts to a stored game system",
			catalog:     mixedCatalog(true),
			gameSystem:  "fifth",
			wantName:    "5E",
			wantTmplIDs: []string{"char_v1", "statblock_v1", "draft_v1", "retired_v1"},
			wantSystem:  &campaign.GameSystem{ID: "fifth", Name: "5E"},
		},
		{
			name:        "Edge: an unknown game system resolves to its id and no templates",
			catalog:     mixedCatalog(false),
			gameSystem:  "sixth",
			wantName:    "sixth",
			wantTmplIDs: []string{},
			wantSystem:  &campaign.GameSystem{ID: "sixth", Name: "sixth"},
		},
		{
			name:        "Edge: an empty stored scope has no name and no templates",
			catalog:     mixedCatalog(false),
			wantTmplIDs: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newCampaignScopeResolver(tc.catalog)
			scope := resolver.ResolveScope(context.Background(), tc.gameSystem, tc.templateID)
			assert.Equal(t, tc.wantName, scope.Name)
			assert.Equal(t, tc.wantTmplIDs, scope.TemplateIDs)
			assert.Equal(t, tc.wantSystem, scope.GameSystem)
		})
	}
}

func TestCampaignScopeResolver_CreateScopeThroughRealRegistry(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	resolver := newCampaignScopeResolver(registry)

	t.Run("Error: the retired fantasy Template cannot start a Campaign", func(t *testing.T) {
		scope, err := resolver.CreateScope(context.Background(), "", "fantasy_v1")

		require.ErrorIs(t, err, campaign.ErrScopeRetired)
		assert.Empty(t, scope.Name)
	})

	t.Run("Happy path: the fifth edition Game system offers only the 5E Templates", func(t *testing.T) {
		scope, err := resolver.CreateScope(context.Background(), template.GameSystemFifth, "")

		require.NoError(t, err)
		assert.Equal(t, "5E", scope.Name)
		assert.Equal(t, []string{"fifth_character_v1", "fifth_stat_block_v1"}, scope.TemplateIDs)
		require.NotNil(t, scope.GameSystem)
		assert.Equal(t, template.GameSystemFifth, scope.GameSystem.ID)
	})
}

func TestCampaignScopeResolver_ResolveScopeThroughRealRegistry(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	resolver := newCampaignScopeResolver(registry)

	tests := []struct {
		name        string
		templateID  string
		wantName    string
		wantTmplIDs []string
	}{
		{
			name:        "Happy path: a legacy campaign document resolves its stored template",
			templateID:  "fantasy_v1",
			wantName:    "Fantasy Template",
			wantTmplIDs: []string{"fantasy_v1"},
		},
		{
			name:        "Edge: an unknown stored template resolves to its id",
			templateID:  "removed_v1",
			wantName:    "removed_v1",
			wantTmplIDs: []string{"removed_v1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope := resolver.ResolveScope(context.Background(), "", tc.templateID)
			assert.Equal(t, tc.wantName, scope.Name)
			assert.Equal(t, tc.wantTmplIDs, scope.TemplateIDs)
			assert.Nil(t, scope.GameSystem)
		})
	}
}
