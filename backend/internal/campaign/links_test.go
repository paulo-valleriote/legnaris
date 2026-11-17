package campaign_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"
)

func npcSheet(t *testing.T, owner, template string, kind plan.SheetKind) sheet.Sheet {
	t.Helper()
	sh := sheetOf(t, owner, template, kind, nil)
	sh.Name = "Brom the Ogre"
	sh.Data = map[string]any{"stats": map[string]any{"hp": 42.0}}
	sh.Layout = &sheet.Layout{SectionOrder: []string{"stats"}, Pinned: []string{"stats.hp"}}
	expr := "1d8"
	sh.Dice = &sheet.Dice{Section: map[string]*string{"stats": &expr}, Row: map[string]*string{"stats.hp": nil}}
	return sh
}

func linkFixture(id, campaignID, sheetID, ownerID string) campaign.SheetLink {
	return campaign.SheetLink{
		ID:         id,
		CampaignID: campaignID,
		SheetID:    sheetID,
		OwnerID:    ownerID,
		Revealed:   false,
		LinkedAt:   campaigntest.Now(),
	}
}

func TestLinkNpc(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	deleting := campaigntest.Copy(base)
	deleting.Status = campaign.StatusDeleting

	type setup func(h *harness, sh sheet.Sheet)
	tests := []struct {
		name         string
		user         string
		sheet        func(t *testing.T) sheet.Sheet
		missing      bool
		deleting     bool
		setup        setup
		wantCode     apperr.Code
		wantCreated  bool
		wantRevealed bool
		wantEvents   int
	}{
		{
			name:        "Happy path: the owner GM links an NPC",
			user:        owner,
			sheet:       func(t *testing.T) sheet.Sheet { return npcSheet(t, owner, base.TemplateID, plan.KindNPC) },
			wantCreated: true,
			wantEvents:  1,
		},
		{
			name:        "Happy path: a co-GM links their own Enemy",
			user:        coGM,
			sheet:       func(t *testing.T) sheet.Sheet { return npcSheet(t, coGM, base.TemplateID, plan.KindEnemy) },
			wantCreated: true,
			wantEvents:  1,
		},
		{
			name: "Edge: re-linking an existing pair returns the current link without an event",
			user: owner,
			sheet: func(t *testing.T) sheet.Sheet {
				return npcSheet(t, owner, base.TemplateID, plan.KindNPC)
			},
			setup: func(h *harness, sh sheet.Sheet) {
				existing := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
				existing.Revealed = true
				h.links.EXPECT().Upsert(mock.Anything, mock.Anything).Return(existing, false, nil).Once()
			},
			wantRevealed: true,
		},
		{
			name: "Edge: concurrent link that lost the upsert returns the winning link",
			user: owner,
			sheet: func(t *testing.T) sheet.Sheet {
				return npcSheet(t, owner, base.TemplateID, plan.KindNPC)
			},
			setup: func(h *harness, sh sheet.Sheet) {
				existing := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
				h.links.EXPECT().Upsert(mock.Anything, mock.Anything).Return(existing, false, nil).Once()
			},
		},
		{
			name:     "Error: a non-member gets not found",
			user:     stranger,
			sheet:    func(t *testing.T) sheet.Sheet { return npcSheet(t, stranger, base.TemplateID, plan.KindNPC) },
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:     "Error: a player cannot link",
			user:     player,
			sheet:    func(t *testing.T) sheet.Sheet { return npcSheet(t, player, base.TemplateID, plan.KindNPC) },
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Error: a GM cannot link a sheet they do not own",
			user:     coGM,
			sheet:    func(t *testing.T) sheet.Sheet { return npcSheet(t, owner, base.TemplateID, plan.KindNPC) },
			wantCode: apperr.CodeNotFound,
		},
		{
			name:     "Error: a missing sheet is not found",
			user:     owner,
			missing:  true,
			wantCode: apperr.CodeNotFound,
		},
		{
			name:     "Error: a character sheet is a kind mismatch",
			user:     owner,
			sheet:    func(t *testing.T) sheet.Sheet { return npcSheet(t, owner, base.TemplateID, plan.KindCharacter) },
			wantCode: apperr.CodeSheetKindMismatch,
		},
		{
			name:     "Error: a template mismatch is refused",
			user:     owner,
			sheet:    func(t *testing.T) sheet.Sheet { return npcSheet(t, owner, "other_template", plan.KindNPC) },
			wantCode: apperr.CodeTemplateMismatch,
		},
		{
			name:  "Error: a link store failure is internal",
			user:  owner,
			sheet: func(t *testing.T) sheet.Sheet { return npcSheet(t, owner, base.TemplateID, plan.KindEnemy) },
			setup: func(h *harness, sh sheet.Sheet) {
				h.links.EXPECT().Upsert(mock.Anything, mock.Anything).Return(campaign.SheetLink{}, false, errStoreDown).Once()
			},
			wantCode: apperr.CodeInternal,
		},
		{
			name:     "Edge: a deleting campaign refuses the link",
			user:     owner,
			deleting: true,
			wantCode: apperr.CodeCampaignNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("Owner Name", nil)
			switch {
			case tc.deleting:
				h.loadAlways(deleting)
			case tc.missing:
				h.loadAlways(base)
				h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).Return(nil, sheet.ErrSheetNotFound).Once()
			default:
				h.loadAlways(base)
			}
			sheetID := campaigntest.NewID(t)
			var chosen *sheet.Sheet
			if tc.sheet != nil {
				sh := tc.sheet(t)
				chosen = &sh
				sheetID = sh.ID
				h.sheetReturns(sh)
				if tc.wantCode == "" && tc.setup == nil {
					h.links.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(link *campaign.SheetLink) bool {
						return link.CampaignID == base.ID && link.SheetID == sh.ID && link.OwnerID == tc.user && !link.Revealed
					})).RunAndReturn(func(_ context.Context, link *campaign.SheetLink) (campaign.SheetLink, bool, error) {
						return *link, true, nil
					}).Once()
				}
			}
			if tc.setup != nil && chosen != nil {
				tc.setup(h, *chosen)
			}
			if tc.wantCode == "" {
				h.expectSave(base.Version, nil)
			}
			view, err := h.service().LinkNpc(context.Background(), campaign.LinkNpcCommand{
				CampaignID: base.ID,
				UserID:     tc.user,
				SheetID:    sheetID,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved, "a rejected link must not reach the save")
				assert.Empty(t, h.events)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			assert.Equal(t, base.Version+1, h.saved.Version)
			assert.Equal(t, sheetID, view.SheetID)
			assert.Equal(t, tc.wantRevealed, view.Revealed)
			assert.True(t, view.IsOwner)
			assert.Equal(t, "Owner Name", view.OwnerName)
			assert.Equal(t, tc.wantEvents, len(h.events))
			if tc.wantEvents == 1 {
				assert.Equal(t, campaign.EventSheetLinked, h.events[0].Type)
			}
		})
	}
}

func TestLinkNpc_RejectsMalformedIDs(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	owner := campaigntest.OwnerID(base)
	tests := []struct {
		name       string
		campaignID string
		userID     string
		sheetID    string
		wantCode   apperr.Code
	}{
		{name: "Error: missing user is unauthorized", campaignID: base.ID, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeUnauthorized},
		{name: "Edge: missing campaign id", campaignID: "", userID: owner, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed campaign id", campaignID: "nope", userID: owner, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: missing sheet id", campaignID: base.ID, userID: owner, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed sheet id", campaignID: base.ID, userID: owner, sheetID: "not-a-uuid", wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().LinkNpc(context.Background(), campaign.LinkNpcCommand{
				CampaignID: tc.campaignID,
				UserID:     tc.userID,
				SheetID:    tc.sheetID,
			})
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestUnlinkNpc(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)

	tests := []struct {
		name     string
		user     string
		sheetID  string
		setup    func(h *harness)
		wantCode apperr.Code
		wantLink bool
	}{
		{
			name:     "Error: a non-member gets not found",
			user:     stranger,
			setup:    func(h *harness) {},
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:     "Error: a player cannot unlink",
			user:     player,
			setup:    func(h *harness) {},
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name: "Error: a co-GM cannot unlink a sheet they do not own",
			user: coGM,
			setup: func(h *harness) {
				h.sheetReturns(sh)
			},
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name: "Edge: unlinking a pair with no link succeeds without an event",
			user: owner,
			setup: func(h *harness) {
				h.sheetReturns(sh)
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(nil, campaign.ErrLinkNotFound).Once()
			},
		},
		{
			name: "Edge: a concurrent unlink that deleted no row emits no event",
			user: owner,
			setup: func(h *harness) {
				h.sheetReturns(sh)
				link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Once()
				h.links.EXPECT().Delete(mock.Anything, base.ID, sh.ID).Return(false, nil).Once()
			},
		},
		{
			name: "Happy path: the owner unlinks their sheet",
			user: owner,
			setup: func(h *harness) {
				h.sheetReturns(sh)
				link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Once()
				h.links.EXPECT().Delete(mock.Anything, base.ID, sh.ID).Return(true, nil).Once()
			},
			wantLink: true,
		},
		{
			name: "Error: a link store failure is internal",
			user: owner,
			setup: func(h *harness) {
				h.sheetReturns(sh)
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(nil, errStoreDown).Once()
			},
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("Owner Name", nil)
			h.loadAlways(base)
			if tc.setup != nil {
				tc.setup(h)
			}
			if tc.wantCode == "" {
				h.expectSave(base.Version, nil)
			}
			sheetID := sh.ID
			if tc.user == stranger || tc.user == player {
				sheetID = campaigntest.NewID(t)
			}
			err := h.service().UnlinkNpc(context.Background(), campaign.UnlinkNpcCommand{
				CampaignID: base.ID,
				UserID:     tc.user,
				SheetID:    sheetID,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved)
				assert.Empty(t, h.events)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			assert.Equal(t, base.Version+1, h.saved.Version)
			if tc.wantLink {
				require.Len(t, h.events, 1)
				assert.Equal(t, campaign.EventSheetUnlinked, h.events[0].Type)
				assert.Equal(t, campaign.ReasonUnlinked, eventData(t, h.events[0])["reason"])
			} else {
				assert.Empty(t, h.events)
			}
		})
	}
}

func TestUnlinkNpc_MissingSheetIsNotFound(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(base)
	h := newHarness(t)
	h.loadAlways(base)
	h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).Return(nil, sheet.ErrSheetNotFound).Once()

	err := h.service().UnlinkNpc(context.Background(), campaign.UnlinkNpcCommand{
		CampaignID: base.ID,
		UserID:     owner,
		SheetID:    campaigntest.NewID(t),
	})
	requireAppErr(t, err, apperr.CodeNotFound)
}

func TestUnlinkNpc_ConcurrentDuplicatesPublishOnce(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(base)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)

	h := newHarness(t)
	h.withUsernames("Owner Name", nil)
	h.loadAlways(base)
	h.sheetReturns(sh)
	link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
	h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Twice()
	h.links.EXPECT().Delete(mock.Anything, base.ID, sh.ID).Return(true, nil).Once()
	h.links.EXPECT().Delete(mock.Anything, base.ID, sh.ID).Return(false, nil).Once()
	h.expectSave(base.Version, nil)
	h.expectSave(base.Version, nil)

	cmd := campaign.UnlinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID}
	require.NoError(t, h.service().UnlinkNpc(context.Background(), cmd))
	require.NoError(t, h.service().UnlinkNpc(context.Background(), cmd))

	require.Len(t, h.events, 1, "only the request that deleted the row publishes")
	assert.Equal(t, campaign.EventSheetUnlinked, h.events[0].Type)
}

func TestLinkNpc_WithoutALinkStoreFailsClosed(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(base)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)

	h := newHarness(t)
	h.loadAlways(base)
	h.sheetReturns(sh)
	svc := campaign.NewService(h.repo, h.tx, h.pub,
		campaign.WithCharacterStore(h.sheets),
		campaign.WithNameLookup(h.names),
		campaign.WithClock(campaigntest.Now),
	)

	_, err := svc.LinkNpc(context.Background(), campaign.LinkNpcCommand{
		CampaignID: base.ID,
		UserID:     owner,
		SheetID:    sh.ID,
	})
	requireAppErr(t, err, apperr.CodeInternal)
	assert.Nil(t, h.saved, "a missing link store must not reach the save")
	assert.Empty(t, h.events)
}

func TestListNpcLinks(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)

	hidden := npcSheet(t, owner, base.TemplateID, plan.KindNPC)
	hidden.Name = "Brom"
	revealed := npcSheet(t, coGM, base.TemplateID, plan.KindEnemy)
	revealed.Name = "Wyrm"
	hiddenLink := linkFixture(campaigntest.NewID(t), base.ID, hidden.ID, owner)
	revealedLink := linkFixture(campaigntest.NewID(t), base.ID, revealed.ID, coGM)
	revealedLink.Revealed = true
	revealedLink.LinkedAt = campaigntest.Now().Add(time.Minute)
	dangling := linkFixture(campaigntest.NewID(t), base.ID, campaigntest.NewID(t), owner)

	tests := []struct {
		name        string
		viewer      string
		links       []campaign.SheetLink
		sheets      map[string]sheet.Sheet
		wantCode    apperr.Code
		wantLen     int
		wantSheetID string
	}{
		{
			name:     "Error: a non-member gets not found",
			viewer:   stranger,
			links:    []campaign.SheetLink{hiddenLink},
			sheets:   map[string]sheet.Sheet{hidden.ID: hidden},
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:        "Happy path: a player lists only revealed links",
			viewer:      player,
			links:       []campaign.SheetLink{hiddenLink, revealedLink},
			sheets:      map[string]sheet.Sheet{hidden.ID: hidden, revealed.ID: revealed},
			wantLen:     1,
			wantSheetID: revealed.ID,
		},
		{
			name:    "Edge: a player with only hidden links gets an empty list",
			viewer:  player,
			links:   []campaign.SheetLink{hiddenLink},
			sheets:  map[string]sheet.Sheet{hidden.ID: hidden},
			wantLen: 0,
		},
		{
			name:    "Edge: a GM with no links gets an empty list",
			viewer:  owner,
			wantLen: 0,
		},
		{
			name:    "Happy path: a GM lists hidden and revealed links with owners",
			viewer:  coGM,
			links:   []campaign.SheetLink{hiddenLink, revealedLink},
			sheets:  map[string]sheet.Sheet{hidden.ID: hidden, revealed.ID: revealed},
			wantLen: 2,
		},
		{
			name:    "Edge: links pointing to a missing sheet are ignored",
			viewer:  owner,
			links:   []campaign.SheetLink{hiddenLink, dangling},
			sheets:  map[string]sheet.Sheet{hidden.ID: hidden},
			wantLen: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			h.withUsernames("A Name", nil)
			h.links.EXPECT().ListByCampaignID(mock.Anything, base.ID).Return(tc.links, nil).Maybe()
			h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, id string) (*sheet.Sheet, error) {
					found, ok := tc.sheets[id]
					if !ok {
						return nil, sheet.ErrSheetNotFound
					}
					cp := found
					return &cp, nil
				},
			).Maybe()
			out, err := h.service().ListNpcLinks(context.Background(), base.ID, tc.viewer)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			require.Len(t, out, tc.wantLen)
			if tc.wantSheetID != "" {
				assert.Equal(t, tc.wantSheetID, out[0].SheetID)
				assert.True(t, out[0].Revealed)
			}
			if tc.wantLen == 2 {
				assert.Equal(t, hidden.ID, out[0].SheetID)
				assert.Equal(t, "Brom", out[0].Name)
				assert.Equal(t, plan.KindNPC, out[0].Kind)
				assert.False(t, out[0].Revealed)
				assert.Equal(t, owner, out[0].OwnerID)
				assert.False(t, out[0].IsOwner)
				assert.Equal(t, revealed.ID, out[1].SheetID)
				assert.True(t, out[1].Revealed)
				assert.True(t, out[1].IsOwner, "the co-GM owns the revealed enemy")
			}
		})
	}
}

func TestUnlinkNpc_RejectsMalformedIDs(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(base)
	tests := []struct {
		name       string
		campaignID string
		userID     string
		sheetID    string
		wantCode   apperr.Code
	}{
		{name: "Error: missing user is unauthorized", campaignID: base.ID, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeUnauthorized},
		{name: "Edge: missing campaign id", campaignID: "", userID: owner, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed campaign id", campaignID: "nope", userID: owner, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: missing sheet id", campaignID: base.ID, userID: owner, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed sheet id", campaignID: base.ID, userID: owner, sheetID: "not-a-uuid", wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			err := h.service().UnlinkNpc(context.Background(), campaign.UnlinkNpcCommand{
				CampaignID: tc.campaignID,
				UserID:     tc.userID,
				SheetID:    tc.sheetID,
			})
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestListNpcLinks_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		campaignID string
		viewer     string
		wantCode   apperr.Code
	}{
		{name: "Edge: missing viewer", campaignID: campaigntest.NewID(t), wantCode: apperr.CodeUnauthorized},
		{name: "Edge: malformed campaign id", campaignID: "nope", viewer: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().ListNpcLinks(context.Background(), tc.campaignID, tc.viewer)
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestReadNpcSheet(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)
	sh.Version = 5
	sh.LayoutVersion = 3
	gmETag := campaign.SheetETag(5, 3, campaign.RoleGM)
	playerETag := campaign.SheetETag(5, 3, campaign.RolePlayer)

	tests := []struct {
		name        string
		viewer      string
		revealed    bool
		missingLink bool
		missingRow  bool
		ifNoneMatch string
		wantCode    apperr.Code
		wantNotMod  bool
		wantETag    string
	}{
		{name: "Happy path: a GM reads a hidden linked NPC with its ETag", viewer: coGM, wantETag: gmETag},
		{name: "Happy path: the owner reads their own linked NPC", viewer: owner, wantETag: gmETag},
		{name: "Happy path: a player reads a revealed NPC read-only", viewer: player, revealed: true, wantETag: playerETag},
		{name: "Happy path: a matching GM ETag answers 304", viewer: coGM, ifNoneMatch: gmETag, wantNotMod: true, wantETag: gmETag},
		{name: "Happy path: a matching player ETag answers 304", viewer: player, revealed: true, ifNoneMatch: playerETag, wantNotMod: true, wantETag: playerETag},
		{name: "Edge: a stale ETag answers the full sheet", viewer: coGM, ifNoneMatch: `"4.3.gm"`, wantETag: gmETag},
		{name: "Edge: a GM ETag does not match a player read", viewer: player, revealed: true, ifNoneMatch: gmETag, wantETag: playerETag},
		{name: "Error: a player cannot read a hidden NPC", viewer: player, wantCode: apperr.CodeNotFound},
		{name: "Error: lost access with a matching ETag is 404 before the tag is evaluated", viewer: player, ifNoneMatch: playerETag, wantCode: apperr.CodeNotFound},
		{name: "Error: a non-member gets not found", viewer: stranger, wantCode: apperr.CodeCampaignNotFound},
		{name: "Error: a missing link is not found", viewer: coGM, missingLink: true, wantCode: apperr.CodeNotFound},
		{name: "Error: a missing sheet is not found", viewer: coGM, missingRow: true, wantCode: apperr.CodeNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			switch {
			case tc.missingLink:
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(nil, campaign.ErrLinkNotFound).Once()
			case tc.wantCode == apperr.CodeCampaignNotFound:
			default:
				link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
				link.Revealed = tc.revealed
				h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Once()
				hiddenFromPlayer := tc.wantCode == apperr.CodeNotFound && !tc.missingRow
				if tc.missingRow {
					h.sheets.EXPECT().FetchByID(mock.Anything, sh.ID).Return(nil, sheet.ErrSheetNotFound).Once()
				} else if !hiddenFromPlayer {
					h.sheetReturns(sh)
				}
			}
			res, err := h.service().ReadNpcSheet(context.Background(), campaign.ReadNpcSheetQuery{
				CampaignID:  base.ID,
				ViewerID:    tc.viewer,
				SheetID:     sh.ID,
				IfNoneMatch: tc.ifNoneMatch,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Empty(t, res.ETag)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantETag, res.ETag)
			assert.Equal(t, tc.wantNotMod, res.NotModified)
			if tc.wantNotMod {
				assert.Nil(t, res.Sheet)
				return
			}
			require.NotNil(t, res.Sheet)
			assert.Equal(t, sh.ID, res.Sheet.ID)
		})
	}
}

func TestReadNpcSheet_NonLinkableKindIsNotFound(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	owner := campaigntest.OwnerID(base)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindCharacter)

	tests := []struct {
		name   string
		viewer string
	}{
		{name: "Edge: a GM reading a link to a character sheet gets not found", viewer: owner},
		{name: "Edge: a player reading a link to a character sheet gets not found", viewer: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
			link.Revealed = true
			h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Once()
			h.sheetReturns(sh)

			res, err := h.service().ReadNpcSheet(context.Background(), campaign.ReadNpcSheetQuery{
				CampaignID: base.ID,
				ViewerID:   tc.viewer,
				SheetID:    sh.ID,
			})
			requireAppErr(t, err, apperr.CodeNotFound)
			assert.Empty(t, res.ETag)
			assert.Nil(t, res.Sheet)
		})
	}
}

func TestReadNpcSheet_RejectsInvalidInput(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	tests := []struct {
		name     string
		query    campaign.ReadNpcSheetQuery
		wantCode apperr.Code
	}{
		{name: "Edge: missing viewer", query: campaign.ReadNpcSheetQuery{CampaignID: base.ID, SheetID: campaigntest.NewID(t)}, wantCode: apperr.CodeUnauthorized},
		{name: "Edge: malformed sheet id", query: campaign.ReadNpcSheetQuery{CampaignID: base.ID, ViewerID: campaigntest.OwnerID(base), SheetID: "nope"}, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed campaign id", query: campaign.ReadNpcSheetQuery{CampaignID: "nope", ViewerID: campaigntest.OwnerID(base), SheetID: campaigntest.NewID(t)}, wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().ReadNpcSheet(context.Background(), tc.query)
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestLinkEvents_CarryNoSheetNameOrValues(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(base)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)
	sh.Name = "Secret Wyrm"
	sh.Data = map[string]any{"stats": map[string]any{"hp": 999.0}}

	h := newHarness(t)
	version := base.Version
	h.repo.EXPECT().FetchByID(mock.Anything, base.ID).RunAndReturn(
		func(context.Context, string) (*campaign.Campaign, error) {
			cp := campaigntest.Copy(base)
			cp.Version = version
			version++
			return &cp, nil
		},
	).Maybe()
	h.withUsernames("Owner Name", nil)
	h.sheetReturns(sh)
	h.links.EXPECT().Upsert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, link *campaign.SheetLink) (campaign.SheetLink, bool, error) {
			return *link, true, nil
		},
	).Once()
	link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
	h.links.EXPECT().Fetch(mock.Anything, base.ID, sh.ID).Return(&link, nil).Once()
	h.links.EXPECT().Delete(mock.Anything, base.ID, sh.ID).Return(true, nil).Once()
	h.expectSave(base.Version, nil)
	h.expectSave(base.Version+1, nil)

	_, err := h.service().LinkNpc(context.Background(), campaign.LinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID})
	require.NoError(t, err)
	require.NoError(t, h.service().UnlinkNpc(context.Background(), campaign.UnlinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID}))
	require.Len(t, h.events, 2)

	assert.Equal(t, map[string]any{
		"sheet_id": sh.ID,
		"kind":     "npc",
		"owner_id": owner,
	}, h.events[0].Data)
	assert.Equal(t, map[string]any{
		"sheet_id": sh.ID,
		"kind":     "npc",
		"owner_id": owner,
		"reason":   campaign.ReasonUnlinked,
	}, h.events[1].Data)
	for _, ev := range h.events {
		rendered := fmt.Sprint(ev.Data)
		assert.NotContains(t, rendered, sh.Name)
		assert.NotContains(t, rendered, "999")
		assert.NotContains(t, ev.Data, "name")
		assert.NotContains(t, ev.Data, "data")
	}
}

func TestSheetLinkCleaner_DeletesLinksAndReturnsThemWithoutPublishing(t *testing.T) {
	sheetID := "01900000-0000-7000-8000-0000000000ff"
	linkStore := mocks.NewMockLinkStore(t)
	removed := []campaign.SheetLink{
		linkFixture(campaigntest.NewID(t), campaigntest.NewID(t), sheetID, campaigntest.NewID(t)),
		linkFixture(campaigntest.NewID(t), campaigntest.NewID(t), sheetID, campaigntest.NewID(t)),
	}
	linkStore.EXPECT().DeleteBySheetID(mock.Anything, sheetID).Return(removed, nil).Once()
	h := newHarness(t)
	cleaner := campaign.NewSheetLinkCleaner(linkStore, h.pub)

	got, err := cleaner.DeleteForSheet(context.Background(), sheetID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, sheet.RemovedLink{
		CampaignID: removed[0].CampaignID,
		SheetID:    removed[0].SheetID,
		OwnerID:    removed[0].OwnerID,
	}, got[0])
	assert.Equal(t, removed[1].CampaignID, got[1].CampaignID)
	assert.Empty(t, h.events, "the cleaner never publishes inside the deleting transaction")
}

func TestSheetLinkCleaner_PublishSheetUnlinkedEmitsOneEventPerLink(t *testing.T) {
	sheetID := "01900000-0000-7000-8000-0000000000ff"
	h := newHarness(t)
	cleaner := campaign.NewSheetLinkCleaner(mocks.NewMockLinkStore(t), h.pub)
	removed := []sheet.RemovedLink{
		{CampaignID: campaigntest.NewID(t), SheetID: sheetID, OwnerID: campaigntest.NewID(t)},
		{CampaignID: campaigntest.NewID(t), SheetID: sheetID, OwnerID: campaigntest.NewID(t)},
	}

	cleaner.PublishSheetUnlinked(plan.KindNPC, removed)

	require.Len(t, h.events, 2)
	for i, ev := range h.events {
		assert.Equal(t, campaign.EventSheetUnlinked, ev.Type)
		assert.Equal(t, removed[i].CampaignID, ev.AggregateID)
		assert.Equal(t, removed[i].OwnerID, *ev.ActorID)
		assert.Equal(t, campaign.ReasonSheetDeleted, eventData(t, ev)["reason"])
		assert.Equal(t, "npc", eventData(t, ev)["kind"])
		assert.NotContains(t, eventData(t, ev), "name")
	}
}

func TestSheetLinkCleaner_StoreFailureReturnsItWithoutPublishing(t *testing.T) {
	linkStore := mocks.NewMockLinkStore(t)
	linkStore.EXPECT().DeleteBySheetID(mock.Anything, "sheet").Return(nil, errStoreDown).Once()
	h := newHarness(t)
	cleaner := campaign.NewSheetLinkCleaner(linkStore, h.pub)

	removed, err := cleaner.DeleteForSheet(context.Background(), "sheet")
	require.ErrorIs(t, err, errStoreDown)
	assert.Empty(t, removed)
	assert.Empty(t, h.events)
}

func eventData(t *testing.T, ev events.Event) map[string]any {
	t.Helper()
	data, ok := ev.Data.(map[string]any)
	require.True(t, ok, "event data must be a map, got %T", ev.Data)
	return data
}

func TestSheetLinkCleaner_StoreFailurePublishesNothing(t *testing.T) {
	linkStore := mocks.NewMockLinkStore(t)
	linkStore.EXPECT().DeleteBySheetID(mock.Anything, "sheet").Return(nil, errStoreDown).Once()
	h := newHarness(t)
	cleaner := campaign.NewSheetLinkCleaner(linkStore, h.pub)

	removed, err := cleaner.DeleteForSheet(context.Background(), "sheet")
	require.ErrorIs(t, err, errStoreDown)
	assert.Empty(t, removed)
	assert.Empty(t, h.events)
}
