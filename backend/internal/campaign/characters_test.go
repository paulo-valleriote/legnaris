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
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"
)

func sheetOf(t *testing.T, owner string, template string, kind plan.SheetKind, linkedTo *string) sheet.Sheet {
	t.Helper()
	return sheet.Sheet{
		ID:         campaigntest.NewID(t),
		OwnerID:    owner,
		TemplateID: template,
		Kind:       kind,
		Name:       "Aria",
		Version:    3,
		CampaignID: linkedTo,
		UpdatedAt:  campaigntest.Now(),
	}
}

func strPtr(s string) *string {
	return &s
}

func (h *harness) loadAlways(stored campaign.Campaign) {
	h.repo.EXPECT().FetchByID(mock.Anything, stored.ID).RunAndReturn(
		func(context.Context, string) (*campaign.Campaign, error) {
			cp := campaigntest.Copy(stored)
			return &cp, nil
		},
	).Maybe()
}

func (h *harness) loadOther(other campaign.Campaign) {
	h.repo.EXPECT().FetchByID(mock.Anything, other.ID).RunAndReturn(
		func(context.Context, string) (*campaign.Campaign, error) {
			cp := campaigntest.Copy(other)
			return &cp, nil
		},
	).Maybe()
}

func (h *harness) sheetReturns(sh sheet.Sheet) {
	h.sheets.EXPECT().FetchByID(mock.Anything, sh.ID).RunAndReturn(
		func(context.Context, string) (*sheet.Sheet, error) {
			cp := sh
			return &cp, nil
		},
	).Maybe()
}

func TestLink(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	stranger := campaigntest.NewID(t)
	other := campaigntest.Build(t, 0, 0)
	other.Status = campaign.StatusActive

	type setup func(h *harness)
	tests := []struct {
		name     string
		user     string
		sheet    func(t *testing.T) sheet.Sheet
		extra    setup
		wantCode apperr.Code
	}{
		{
			name:     "Error: non-member cannot link",
			user:     stranger,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, stranger, base.TemplateID, plan.KindCharacter, nil) },
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:     "Error: GM cannot link a character",
			user:     gm,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, gm, base.TemplateID, plan.KindCharacter, nil) },
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Error: sheet does not exist",
			user:     player,
			sheet:    nil,
			wantCode: apperr.CodeNotFound,
		},
		{
			name:     "Error: sheet owned by another user reads as not found",
			user:     player,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, stranger, base.TemplateID, plan.KindCharacter, nil) },
			wantCode: apperr.CodeNotFound,
		},
		{
			name:     "Error: NPC sheet cannot be linked",
			user:     player,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, player, base.TemplateID, plan.KindNPC, nil) },
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Error: monster sheet cannot be linked",
			user:     player,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, player, base.TemplateID, plan.KindMonster, nil) },
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Error: template mismatch is 422",
			user:     player,
			sheet:    func(t *testing.T) sheet.Sheet { return sheetOf(t, player, "other_template", plan.KindCharacter, nil) },
			wantCode: apperr.CodeTemplateMismatch,
		},
		{
			name: "Error: sheet already linked to this active campaign",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(base.ID))
			},
			wantCode: apperr.CodeSheetAlreadyLinked,
		},
		{
			name: "Error: sheet already linked to another active campaign",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(other.ID))
			},
			extra:    func(h *harness) { h.loadOther(other) },
			wantCode: apperr.CodeSheetAlreadyLinked,
		},
		{
			name: "Error: concurrent link lost the compare-and-set",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)
			},
			extra: func(h *harness) {
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, mock.Anything, mock.Anything, base.ID).Return(false, nil).Once()
			},
			wantCode: apperr.CodeSheetAlreadyLinked,
		},
		{
			name: "Happy path: free sheet links and replaces nothing",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)
			},
			extra: func(h *harness) {
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, mock.Anything, mock.Anything, base.ID).Return(true, nil).Once()
			},
		},
		{
			name: "Happy path: dangling campaign_id reads as not linked",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(campaigntest.NewID(t)))
			},
			extra: func(h *harness) {
				h.repo.EXPECT().FetchByID(mock.Anything, mock.Anything).Return(nil, campaign.ErrNotFound).Maybe()
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, mock.Anything, mock.Anything, base.ID).Return(true, nil).Once()
			},
		},
		{
			name: "Happy path: sheet whose campaign is deleting reads as not linked",
			user: player,
			sheet: func(t *testing.T) sheet.Sheet {
				return sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(other.ID))
			},
			extra: func(h *harness) {
				deleting := campaigntest.Copy(other)
				deleting.Status = campaign.StatusDeleting
				h.loadOther(deleting)
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(1), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, mock.Anything, mock.Anything, base.ID).Return(true, nil).Once()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			h.withUsernames("someone", nil)
			sheetID := campaigntest.NewID(t)
			if tc.sheet != nil {
				sh := tc.sheet(t)
				sheetID = sh.ID
				h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).RunAndReturn(
					func(context.Context, string) (*sheet.Sheet, error) {
						cp := sh
						return &cp, nil
					},
				).Maybe()
			} else {
				h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).Return(nil, sheet.ErrSheetNotFound).Maybe()
			}
			if tc.extra != nil {
				tc.extra(h)
			}
			if tc.wantCode == "" {
				h.expectSave(base.Version, nil)
			}
			_, err := h.service().Link(context.Background(), campaign.LinkCommand{
				CampaignID: base.ID,
				UserID:     tc.user,
				SheetID:    sheetID,
			})
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			requireAppErr(t, err, tc.wantCode)
			if tc.wantCode == apperr.CodeTemplateMismatch || tc.wantCode == apperr.CodeSheetAlreadyLinked {
				assert.Nil(t, h.saved, "a rejected link must not reach the save")
			}
		})
	}
}

func TestLink_RejectsMalformedIDs(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	tests := []struct {
		name       string
		campaignID string
		userID     string
		sheetID    string
		wantCode   apperr.Code
	}{
		{name: "Edge: empty sheet id", campaignID: base.ID, userID: player, sheetID: "", wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: non UUID sheet id", campaignID: base.ID, userID: player, sheetID: "not-a-uuid", wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: empty campaign id", campaignID: "", userID: player, sheetID: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
		{name: "Error: missing user is unauthorized", campaignID: base.ID, userID: "", sheetID: campaigntest.NewID(t), wantCode: apperr.CodeUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().Link(context.Background(), campaign.LinkCommand{
				CampaignID: tc.campaignID,
				UserID:     tc.userID,
				SheetID:    tc.sheetID,
			})
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestLink_HappyPathPublishesLinkAndSwapEvents(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	sh := sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)

	h := newHarness(t)
	h.loadOnce(base)
	h.withUsernames("someone", nil)
	h.sheetReturns(sh)
	h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(1), nil).Once()
	h.sheets.EXPECT().LinkCampaign(mock.Anything, sh.ID, (*string)(nil), base.ID).Return(true, nil).Once()
	h.expectSave(base.Version, nil)

	view, err := h.service().Link(context.Background(), campaign.LinkCommand{CampaignID: base.ID, UserID: player, SheetID: sh.ID})
	require.NoError(t, err)
	assert.Equal(t, base.Version+1, view.Version)
	require.Len(t, h.events, 2)
	assert.Equal(t, campaign.EventSheetLinked, h.events[0].Type)
	assert.Equal(t, map[string]any{"sheet_id": sh.ID}, h.events[0].Data)
	assert.Equal(t, campaign.EventSheetUnlinked, h.events[1].Type)
	assert.Equal(t, map[string]any{"user_id": player}, h.events[1].Data)
}

func TestUnlink(t *testing.T) {
	base := campaigntest.Build(t, 1, 1)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	stranger := campaigntest.NewID(t)

	tests := []struct {
		name       string
		user       string
		removed    int64
		repoErr    bool
		wantCode   apperr.Code
		wantEvents int
	}{
		{name: "Error: non-member cannot unlink", user: stranger, wantCode: apperr.CodeCampaignNotFound},
		{name: "Error: unlink store failure is internal", user: player, repoErr: true, wantCode: apperr.CodeInternal},
		{name: "Edge: unlinking with no character still succeeds without an event", user: gm, removed: 0, wantEvents: 0},
		{name: "Happy path: unlink publishes sheet_unlinked", user: player, removed: 1, wantEvents: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadOnce(base)
			h.withUsernames("someone", nil)
			if tc.user == player || tc.user == gm {
				if tc.repoErr {
					h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, tc.user, base.ID).Return(int64(0), errStoreDown).Once()
				} else {
					h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, tc.user, base.ID).Return(tc.removed, nil).Once()
					h.expectSave(base.Version, nil)
				}
			}
			view, err := h.service().Unlink(context.Background(), campaign.UnlinkCommand{CampaignID: base.ID, UserID: tc.user})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, base.Version+1, view.Version)
			assert.Len(t, h.events, tc.wantEvents)
		})
	}
}

var errStoreDown = errors.New("store down")

func TestListLinkedSheets(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	owner := campaigntest.OwnerID(base)

	linked := sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(base.ID))
	tests := []struct {
		name     string
		viewer   string
		stored   bool
		wantCode apperr.Code
		wantLen  int
	}{
		{name: "Error: stranger gets not found", viewer: stranger, stored: true, wantCode: apperr.CodeCampaignNotFound},
		{name: "Error: player is forbidden", viewer: player, stored: true, wantCode: apperr.CodeCampaignForbidden},
		{name: "Edge: GM with no linked sheets gets an empty list", viewer: gm, stored: false, wantLen: 0},
		{name: "Happy path: GM lists linked sheets", viewer: gm, stored: true, wantLen: 1},
		{name: "Happy path: owner (a GM) lists linked sheets", viewer: owner, stored: true, wantLen: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			if tc.stored {
				h.linked = []sheet.Sheet{linked}
			}
			out, err := h.service().ListLinkedSheets(context.Background(), base.ID, tc.viewer)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			require.Len(t, out, tc.wantLen)
			if tc.wantLen == 1 {
				assert.Equal(t, linked.ID, out[0].ID)
				assert.Equal(t, linked.OwnerID, out[0].OwnerID)
			}
		})
	}
}

func TestListLinkedSheets_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		campaignID string
		viewer     string
		wantCode   apperr.Code
	}{
		{name: "Edge: missing viewer", campaignID: campaigntest.NewID(t), viewer: "", wantCode: apperr.CodeUnauthorized},
		{name: "Edge: malformed campaign id", campaignID: "x", viewer: campaigntest.NewID(t), wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().ListLinkedSheets(context.Background(), tc.campaignID, tc.viewer)
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestReadLinkedSheet(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)
	player := players[0]
	otherPlayer := players[1]
	stranger := campaigntest.NewID(t)

	linked := sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(base.ID))
	linked.Version = 4
	linked.LayoutVersion = 2
	unlinked := sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)
	linkedToOther := sheetOf(t, player, base.TemplateID, plan.KindCharacter, strPtr(campaigntest.NewID(t)))
	wantETag := campaign.SheetETag(4, 2, campaign.RoleGM)

	tests := []struct {
		name        string
		viewer      string
		sheet       sheet.Sheet
		ifNoneMatch string
		missingRow  bool
		wantCode    apperr.Code
		wantNotMod  bool
		wantETag    string
	}{
		{name: "Happy path: GM reads a linked sheet with its ETag", viewer: gm, sheet: linked, wantETag: wantETag},
		{name: "Happy path: GM revalidates and gets 304 on matching ETag", viewer: gm, sheet: linked, ifNoneMatch: wantETag, wantNotMod: true, wantETag: wantETag},
		{name: "Happy path: GM gets a full read on stale ETag", viewer: gm, sheet: linked, ifNoneMatch: `"3.2.gm"`, wantETag: wantETag},
		{name: "Error: lost access (stranger) is 404 even with matching ETag", viewer: stranger, sheet: linked, ifNoneMatch: wantETag, wantCode: apperr.CodeCampaignNotFound},
		{name: "Error: other player is forbidden even with matching ETag", viewer: otherPlayer, sheet: linked, ifNoneMatch: wantETag, wantCode: apperr.CodeCampaignForbidden},
		{name: "Error: unlinked sheet reads as not found for GM", viewer: gm, sheet: unlinked, wantCode: apperr.CodeNotFound},
		{name: "Error: sheet linked to another campaign reads as not found", viewer: gm, sheet: linkedToOther, wantCode: apperr.CodeNotFound},
		{name: "Error: missing sheet row is not found", viewer: gm, sheet: linked, missingRow: true, wantCode: apperr.CodeNotFound},
		{name: "Edge: owner player reads their own linked sheet", viewer: player, sheet: linked, wantETag: campaign.SheetETag(4, 2, campaign.RolePlayer)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadAlways(base)
			if tc.missingRow {
				h.sheets.EXPECT().FetchByID(mock.Anything, linked.ID).Return(nil, sheet.ErrSheetNotFound).Maybe()
			} else {
				h.sheetReturns(tc.sheet)
			}
			res, err := h.service().ReadLinkedSheet(context.Background(), campaign.ReadLinkedSheetQuery{
				CampaignID:  base.ID,
				ViewerID:    tc.viewer,
				SheetID:     tc.sheet.ID,
				IfNoneMatch: tc.ifNoneMatch,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantNotMod, res.NotModified)
			assert.Equal(t, tc.wantETag, res.ETag)
			if tc.wantNotMod {
				assert.Nil(t, res.Sheet, "a 304 must not carry the sheet body")
				return
			}
			require.NotNil(t, res.Sheet)
			assert.Equal(t, tc.sheet.ID, res.Sheet.ID)
		})
	}
}

func TestReadLinkedSheet_RejectsInvalidInput(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	tests := []struct {
		name     string
		query    campaign.ReadLinkedSheetQuery
		wantCode apperr.Code
	}{
		{name: "Edge: missing viewer", query: campaign.ReadLinkedSheetQuery{CampaignID: base.ID, SheetID: campaigntest.NewID(t)}, wantCode: apperr.CodeUnauthorized},
		{name: "Edge: malformed sheet id", query: campaign.ReadLinkedSheetQuery{CampaignID: base.ID, ViewerID: campaigntest.OwnerID(base), SheetID: "nope"}, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: malformed campaign id", query: campaign.ReadLinkedSheetQuery{CampaignID: "nope", ViewerID: campaigntest.OwnerID(base), SheetID: campaigntest.NewID(t)}, wantCode: apperr.CodeInvalidRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.service().ReadLinkedSheet(context.Background(), tc.query)
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestFetch_MembersCarryCharacterNames(t *testing.T) {
	base := campaigntest.Build(t, 0, 2)
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)
	linked := sheetOf(t, players[0], base.TemplateID, plan.KindCharacter, strPtr(base.ID))
	linked.Name = "Aria the Bold"

	h := newHarness(t)
	h.loadAlways(base)
	h.withUsernames("someone", nil)
	h.linked = []sheet.Sheet{linked}

	view, err := h.service().Fetch(context.Background(), base.ID, campaigntest.OwnerID(base))
	require.NoError(t, err)
	names := map[string]string{}
	for _, m := range view.Members {
		names[m.UserID] = m.CharacterName
	}
	assert.Equal(t, "Aria the Bold", names[players[0]])
	assert.Empty(t, names[players[1]], "a member without a linked character has an empty character name")
	assert.Empty(t, names[campaigntest.OwnerID(base)])
}

func TestFetch_CharacterLookupFailureDegradesToEmptyNames(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	h := newHarness(t)
	h.loadAlways(base)
	h.withUsernames("someone", nil)
	h.linkedErr = errStoreDown

	view, err := h.service().Fetch(context.Background(), base.ID, campaigntest.OwnerID(base))
	require.NoError(t, err)
	for _, m := range view.Members {
		assert.Empty(t, m.CharacterName)
	}
}
