package campaign_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

func scopeResolver(t *testing.T, resolve campaign.Scope) *mocks.MockScopeResolver {
	t.Helper()
	resolver := mocks.NewMockScopeResolver(t)
	resolver.EXPECT().ResolveScope(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string, string) campaign.Scope { return resolve },
	).Maybe()
	return resolver
}

func createResolver(t *testing.T, created campaign.Scope, resolve campaign.Scope, err error) *mocks.MockScopeResolver {
	t.Helper()
	resolver := scopeResolver(t, resolve)
	resolver.EXPECT().CreateScope(mock.Anything, mock.Anything, mock.Anything).Return(created, err).Once()
	return resolver
}

func TestCreate_Scope(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	gameSystemScope := campaign.Scope{
		Name:        "5E",
		TemplateIDs: []string{"char_v1", "statblock_v1"},
		GameSystem:  &campaign.GameSystem{ID: "fifth", Name: "5E"},
	}
	templateScope := campaign.Scope{Name: "Fantasy Template", TemplateIDs: []string{"fantasy_v1"}}

	tests := []struct {
		name        string
		cmd         campaign.CreateCommand
		created     campaign.Scope
		resolved    campaign.Scope
		scopeErr    error
		callScope   bool
		wantCode    apperr.Code
		wantGame    string
		wantTmpl    string
		wantEvent   map[string]any
		wantScope   string
		wantTmplIDs []string
	}{
		{
			name:        "Happy path: a standalone template scope is stored as template_id",
			cmd:         campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", TemplateID: "fantasy_v1"},
			created:     templateScope,
			resolved:    templateScope,
			callScope:   true,
			wantTmpl:    "fantasy_v1",
			wantEvent:   map[string]any{"template_id": "fantasy_v1"},
			wantScope:   "Fantasy Template",
			wantTmplIDs: []string{"fantasy_v1"},
		},
		{
			name:        "Happy path: a game system scope is stored as game_system",
			cmd:         campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", GameSystem: "fifth"},
			created:     gameSystemScope,
			resolved:    gameSystemScope,
			callScope:   true,
			wantGame:    "fifth",
			wantEvent:   map[string]any{"game_system": "fifth"},
			wantScope:   "5E",
			wantTmplIDs: []string{"char_v1", "statblock_v1"},
		},
		{
			name:     "Edge: both scope fields are rejected",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", GameSystem: "fifth", TemplateID: "fantasy_v1"},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: neither scope field is rejected",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "Curse"},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: an unknown scope is rejected",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", GameSystem: "sixth"},
			scopeErr:  campaign.ErrScopeUnknown,
			callScope: true,
			wantCode:  apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: a retired template is rejected",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", TemplateID: "retired_v1"},
			scopeErr:  campaign.ErrScopeRetired,
			callScope: true,
			wantCode:  apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: a draft template outside development is rejected",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", TemplateID: "draft_v1"},
			scopeErr:  campaign.ErrScopeDraft,
			callScope: true,
			wantCode:  apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: a template that belongs to a game system is rejected",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", TemplateID: "char_v1"},
			scopeErr:  campaign.ErrScopeGameSystem,
			callScope: true,
			wantCode:  apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: a game system without a creatable template is rejected",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", GameSystem: "fifth"},
			scopeErr:  campaign.ErrScopeNoCreatable,
			callScope: true,
			wantCode:  apperr.CodeInvalidRequest,
		},
		{
			name:      "Error: an unexpected resolver failure is internal",
			cmd:       campaign.CreateCommand{OwnerID: ownerID, Name: "Curse", TemplateID: "fantasy_v1"},
			scopeErr:  errors.New("boom"),
			callScope: true,
			wantCode:  apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			var resolver *mocks.MockScopeResolver
			if tc.callScope {
				resolver = createResolver(t, tc.created, tc.resolved, tc.scopeErr)
			} else {
				resolver = scopeResolver(t, tc.resolved)
			}
			inserted := false
			h.repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, c *campaign.Campaign) error {
					inserted = true
					h.inserted = c
					return nil
				},
			).Maybe()

			view, err := h.service(campaign.WithScopeResolver(resolver)).Create(context.Background(), tc.cmd)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.False(t, inserted, "a refused create must not insert")
				assert.Empty(t, h.events, "a refused create must not publish")
				return
			}
			require.NoError(t, err)
			require.True(t, inserted)
			assert.Equal(t, tc.wantGame, h.inserted.GameSystemID)
			assert.Equal(t, tc.wantTmpl, h.inserted.TemplateID)
			require.Len(t, h.events, 1)
			assert.Equal(t, tc.wantEvent, h.events[0].Data)
			assert.Equal(t, tc.wantScope, view.ScopeName)
			assert.Equal(t, tc.wantTmplIDs, view.TemplateIDs)
			assert.Equal(t, tc.wantTmpl, view.TemplateID)
		})
	}
}

func TestCreate_WithoutScopeResolverFailsClosed(t *testing.T) {
	h := newHarness(t)
	svc := campaign.NewService(h.repo, h.tx, h.pub, campaign.WithClock(campaigntest.Now))

	_, err := svc.Create(context.Background(), campaign.CreateCommand{
		OwnerID: campaigntest.NewID(t), Name: "Curse", TemplateID: "fantasy_v1",
	})
	requireAppErr(t, err, apperr.CodeInternal)
}

func TestFetch_Scope(t *testing.T) {
	templateCampaign := campaigntest.Build(t, 0, 0)
	templateCampaign.TemplateID = "fantasy_v1"
	unknownCampaign := campaigntest.Build(t, 0, 0)
	unknownCampaign.TemplateID = "removed_v1"
	systemCampaign := campaigntest.Build(t, 0, 0)
	systemCampaign.TemplateID = ""
	systemCampaign.GameSystemID = "fifth"

	tests := []struct {
		name        string
		stored      campaign.Campaign
		resolved    campaign.Scope
		wantScope   string
		wantTmplIDs []string
		wantGame    *campaign.GameSystem
		wantTmpl    string
	}{
		{
			name:        "Happy path: a stored template resolves its name",
			stored:      templateCampaign,
			resolved:    campaign.Scope{Name: "Fantasy Template", TemplateIDs: []string{"fantasy_v1"}},
			wantScope:   "Fantasy Template",
			wantTmplIDs: []string{"fantasy_v1"},
			wantTmpl:    "fantasy_v1",
		},
		{
			name:        "Edge: a stored unknown template reads as its id",
			stored:      unknownCampaign,
			resolved:    campaign.Scope{Name: "removed_v1", TemplateIDs: []string{"removed_v1"}},
			wantScope:   "removed_v1",
			wantTmplIDs: []string{"removed_v1"},
			wantTmpl:    "removed_v1",
		},
		{
			name:        "Happy path: a game system scope carries the system and its templates",
			stored:      systemCampaign,
			resolved:    campaign.Scope{Name: "5E", TemplateIDs: []string{"char_v1", "retired_v1"}, GameSystem: &campaign.GameSystem{ID: "fifth", Name: "5E"}},
			wantScope:   "5E",
			wantTmplIDs: []string{"char_v1", "retired_v1"},
			wantGame:    &campaign.GameSystem{ID: "fifth", Name: "5E"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadOnce(tc.stored)

			view, err := h.service(campaign.WithScopeResolver(scopeResolver(t, tc.resolved))).
				Fetch(context.Background(), tc.stored.ID, campaigntest.OwnerID(tc.stored))
			require.NoError(t, err)
			assert.Equal(t, tc.wantScope, view.ScopeName)
			assert.Equal(t, tc.wantTmplIDs, view.TemplateIDs)
			assert.Equal(t, tc.wantGame, view.GameSystem)
			assert.Equal(t, tc.wantTmpl, view.TemplateID)
		})
	}
}

func TestListMine_Scope(t *testing.T) {
	stored := campaigntest.Build(t, 0, 0)
	stored.TemplateID = ""
	stored.GameSystemID = "fifth"
	owner := campaigntest.OwnerID(stored)
	h := newHarness(t)
	resolved := campaign.Scope{Name: "5E", TemplateIDs: []string{"char_v1"}, GameSystem: &campaign.GameSystem{ID: "fifth", Name: "5E"}}
	h.repo.EXPECT().ListActiveByMember(mock.Anything, owner).Return([]campaign.Campaign{stored}, nil).Once()

	summaries, err := h.service(campaign.WithScopeResolver(scopeResolver(t, resolved))).ListMine(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, "5E", summaries[0].ScopeName)
	assert.Equal(t, []string{"char_v1"}, summaries[0].TemplateIDs)
	assert.Equal(t, &campaign.GameSystem{ID: "fifth", Name: "5E"}, summaries[0].GameSystem)
	assert.Empty(t, summaries[0].TemplateID)
}

func TestLink_ScopeMembership(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	base.TemplateID = ""
	base.GameSystemID = "fifth"
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	resolved := campaign.Scope{Name: "5E", TemplateIDs: []string{"char_v1", "retired_v1"}, GameSystem: &campaign.GameSystem{ID: "fifth", Name: "5E"}}

	tests := []struct {
		name      string
		template  string
		wantCode  apperr.Code
		linkCalls bool
	}{
		{name: "Happy path: the first template of the game system links", template: "char_v1", linkCalls: true},
		{name: "Happy path: a retired template of the game system still links", template: "retired_v1", linkCalls: true},
		{name: "Error: a template of another game system is a mismatch", template: "other_v1", wantCode: apperr.CodeTemplateMismatch},
		{name: "Error: a standalone template is a mismatch", template: "fantasy_v1", wantCode: apperr.CodeTemplateMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadAlways(base)
			sh := sheetOf(t, player, tc.template, plan.KindCharacter, nil)
			h.sheetReturns(sh)
			if tc.linkCalls {
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, sh.ID, mock.Anything, base.ID).Return(true, nil).Once()
				h.expectSave(base.Version, nil)
			}

			_, err := h.service(campaign.WithScopeResolver(scopeResolver(t, resolved))).Link(context.Background(), campaign.LinkCommand{
				CampaignID: base.ID,
				UserID:     player,
				SheetID:    sh.ID,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
		})
	}
}

func TestLink_TwoTemplatesOfOneGameSystemLinkToOneCampaign(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	base.TemplateID = ""
	base.GameSystemID = "fifth"
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	resolved := campaign.Scope{Name: "5E", TemplateIDs: []string{"char_v1", "statblock_v1", "retired_v1"}, GameSystem: &campaign.GameSystem{ID: "fifth", Name: "5E"}}
	h := newHarness(t)
	h.withUsernames("someone", nil)
	h.loadAlways(base)
	resolver := scopeResolver(t, resolved)

	for _, templateID := range []string{"char_v1", "statblock_v1"} {
		sh := sheetOf(t, player, templateID, plan.KindCharacter, nil)
		h.sheetReturns(sh)
		h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
		h.sheets.EXPECT().LinkCampaign(mock.Anything, sh.ID, mock.Anything, base.ID).Return(true, nil).Once()
		h.expectSave(base.Version, nil)

		_, err := h.service(campaign.WithScopeResolver(resolver)).Link(context.Background(), campaign.LinkCommand{
			CampaignID: base.ID,
			UserID:     player,
			SheetID:    sh.ID,
		})
		require.NoError(t, err)
	}
}

func TestLinkNpc_ScopeMembership(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	base.GameSystemID = "fifth"
	owner := campaigntest.OwnerID(base)
	resolved := campaign.Scope{Name: "5E", TemplateIDs: []string{"statblock_v1", "retired_v1"}, GameSystem: &campaign.GameSystem{ID: "fifth", Name: "5E"}}

	tests := []struct {
		name     string
		template string
		wantCode apperr.Code
	}{
		{name: "Happy path: a stat block of the game system links", template: "statblock_v1"},
		{name: "Happy path: a retired stat block of the game system links", template: "retired_v1"},
		{name: "Error: a stat block of another game system is a mismatch", template: "other_v1", wantCode: apperr.CodeTemplateMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadAlways(base)
			sh := npcSheet(t, owner, tc.template, plan.KindNPC)
			h.sheetReturns(sh)
			if tc.wantCode == "" {
				h.links.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(
					func(_ context.Context, candidate *campaign.SheetLink) (campaign.SheetLink, bool, error) {
						return *candidate, true, nil
					},
				).Once()
				h.expectSave(base.Version, nil)
			}

			_, err := h.service(campaign.WithScopeResolver(scopeResolver(t, resolved))).LinkNpc(context.Background(), campaign.LinkNpcCommand{
				CampaignID: base.ID,
				UserID:     owner,
				SheetID:    sh.ID,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDeletionEvent_CarriesTheStoredScope(t *testing.T) {
	tests := []struct {
		name     string
		shape    func(c *campaign.Campaign)
		wantData map[string]any
	}{
		{
			name:     "Happy path: a template campaign announces template_id",
			shape:    func(c *campaign.Campaign) { c.TemplateID = "fantasy_v1"; c.GameSystemID = "" },
			wantData: map[string]any{"template_id": "fantasy_v1"},
		},
		{
			name:     "Happy path: a game system campaign announces game_system",
			shape:    func(c *campaign.Campaign) { c.TemplateID = ""; c.GameSystemID = "fifth" },
			wantData: map[string]any{"game_system": "fifth"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newDeletionHarness(t)
			c := deletionRemovalFixture(t)
			tc.shape(&c)
			h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(3), nil).Once()
			h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
			h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
			h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(true, nil).Once()

			require.NoError(t, h.deps().Handle(context.Background(), c))
			require.Len(t, h.sent, 1)
			assert.Equal(t, tc.wantData, h.sent[0].Data)
		})
	}
}

func TestCreateAndLink_FantasyV1Regression(t *testing.T) {
	h := newHarness(t)
	h.withUsernames("someone", nil)
	ownerID := campaigntest.NewID(t)
	resolved := campaign.Scope{Name: "Fantasy Template", TemplateIDs: []string{"fantasy_v1"}}
	resolver := createResolver(t, resolved, resolved, nil)
	h.repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, c *campaign.Campaign) error {
			h.inserted = c
			return nil
		},
	).Once()

	view, err := h.service(campaign.WithScopeResolver(resolver)).Create(context.Background(), campaign.CreateCommand{
		OwnerID: ownerID, Name: "Curse", TemplateID: "fantasy_v1",
	})
	require.NoError(t, err)
	assert.Equal(t, "fantasy_v1", h.inserted.TemplateID)
	assert.Empty(t, h.inserted.GameSystemID)
	assert.Equal(t, "Fantasy Template", view.ScopeName)
	assert.Equal(t, []string{"fantasy_v1"}, view.TemplateIDs)

	base := campaigntest.Build(t, 0, 1)
	base.Members[0].UserID = ownerID
	base.TemplateID = "fantasy_v1"
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	h.loadAlways(base)
	sh := sheetOf(t, player, "fantasy_v1", plan.KindCharacter, nil)
	h.sheetReturns(sh)
	h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
	h.sheets.EXPECT().LinkCampaign(mock.Anything, sh.ID, mock.Anything, base.ID).Return(true, nil).Once()
	h.expectSave(base.Version, nil)

	_, err = h.service(campaign.WithScopeResolver(resolver)).Link(context.Background(), campaign.LinkCommand{
		CampaignID: base.ID, UserID: player, SheetID: sh.ID,
	})
	require.NoError(t, err)
}
