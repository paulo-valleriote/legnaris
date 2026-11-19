package campaign_test

import (
	"context"
	"fmt"
	"testing"

	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSetNpcRevealed(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	deleting := campaigntest.Copy(base)
	deleting.Status = campaign.StatusDeleting
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)

	tests := []struct {
		name         string
		user         string
		revealed     bool
		current      bool
		deleting     bool
		missingLink  bool
		missingSheet bool
		sheetErr     error
		setErr       error
		wantCode     apperr.Code
		wantEvent    string
	}{
		{
			name:      "Happy path: a co-GM reveals a hidden NPC",
			user:      coGM,
			revealed:  true,
			current:   false,
			wantEvent: campaign.EventSheetRevealed,
		},
		{
			name:      "Happy path: the owner reveals their own hidden NPC",
			user:      owner,
			revealed:  true,
			current:   false,
			wantEvent: campaign.EventSheetRevealed,
		},
		{
			name:      "Happy path: a GM hides a revealed NPC",
			user:      coGM,
			revealed:  false,
			current:   true,
			wantEvent: campaign.EventSheetHidden,
		},
		{
			name:     "Edge: revealing an already revealed link returns the current link without an event",
			user:     coGM,
			revealed: true,
			current:  true,
		},
		{
			name:     "Edge: hiding an already hidden link returns the current link without an event",
			user:     owner,
			revealed: false,
			current:  false,
		},
		{
			name:     "Error: a player cannot reveal",
			user:     player,
			revealed: true,
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Error: a player cannot hide",
			user:     player,
			revealed: false,
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Error: a non-member gets campaign not found",
			user:     stranger,
			revealed: true,
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:        "Error: a missing link is not found",
			user:        owner,
			revealed:    true,
			missingLink: true,
			wantCode:    apperr.CodeNotFound,
		},
		{
			name:         "Error: a link pointing to a missing sheet is not found",
			user:         owner,
			revealed:     true,
			missingSheet: true,
			wantCode:     apperr.CodeNotFound,
		},
		{
			name:     "Error: a sheet read failure is internal",
			user:     owner,
			revealed: true,
			sheetErr: errStoreDown,
			wantCode: apperr.CodeInternal,
		},
		{
			name:     "Error: a reveal write failure is internal",
			user:     owner,
			revealed: true,
			setErr:   errStoreDown,
			wantCode: apperr.CodeInternal,
		},
		{
			name:     "Edge: a deleting campaign is not found",
			user:     owner,
			revealed: true,
			deleting: true,
			wantCode: apperr.CodeCampaignNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("Owner Name", nil)
			if tc.deleting {
				h.loadAlways(deleting)
			} else {
				h.loadAlways(base)
			}
			memberRejected := tc.wantCode == apperr.CodeCampaignNotFound || tc.wantCode == apperr.CodeCampaignForbidden
			if !memberRejected && !tc.deleting {
				switch {
				case tc.missingSheet:
					h.sheets.EXPECT().FetchByID(mock.Anything, sh.ID).Return(nil, sheeterr.ErrSheetNotFound).Once()
				case tc.sheetErr != nil:
					h.sheets.EXPECT().FetchByID(mock.Anything, sh.ID).Return(nil, tc.sheetErr).Once()
				default:
					h.sheetReturns(sh)
					link := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
					link.Revealed = tc.current
					switch {
					case tc.missingLink:
						h.links.EXPECT().SetRevealed(mock.Anything, base.ID, sh.ID, tc.revealed).
							Return(campaign.SheetLink{}, false, campaign.ErrLinkNotFound).Once()
					case tc.setErr != nil:
						h.links.EXPECT().SetRevealed(mock.Anything, base.ID, sh.ID, tc.revealed).
							Return(campaign.SheetLink{}, false, tc.setErr).Once()
					default:
						h.links.EXPECT().SetRevealed(mock.Anything, base.ID, sh.ID, tc.revealed).RunAndReturn(
							func(_ context.Context, _, _ string, revealed bool) (campaign.SheetLink, bool, error) {
								out := link
								out.Revealed = revealed
								return out, tc.current != revealed, nil
							},
						).Once()
					}
				}
			}
			if tc.wantCode == "" {
				h.expectSave(base.Version, nil)
			}
			view, err := h.service().SetNpcRevealed(context.Background(), campaign.RevealNpcCommand{
				CampaignID: base.ID,
				UserID:     tc.user,
				SheetID:    sh.ID,
				Revealed:   tc.revealed,
			})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved, "a rejected reveal must not reach the save")
				assert.Empty(t, h.events)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved, "reveal and hide run inside the campaign transaction")
			assert.Equal(t, base.Version+1, h.saved.Version)
			assert.Equal(t, sh.ID, view.SheetID)
			assert.Equal(t, tc.revealed, view.Revealed)
			assert.Equal(t, tc.user == owner, view.IsOwner)
			assert.Equal(t, "Owner Name", view.OwnerName)
			if tc.wantEvent == "" {
				assert.Empty(t, h.events)
				return
			}
			require.Len(t, h.events, 1)
			assert.Equal(t, tc.wantEvent, h.events[0].Type)
			assert.Equal(t, base.ID, h.events[0].AggregateID)
			assert.Equal(t, int64(base.Version+1), h.events[0].AggregateVersion)
			assert.Equal(t, map[string]any{
				"sheet_id": sh.ID,
				"kind":     "npc",
				"owner_id": owner,
			}, h.events[0].Data)
		})
	}
}

func TestSetNpcRevealed_RejectsMalformedIDs(t *testing.T) {
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
			_, err := h.service().SetNpcRevealed(context.Background(), campaign.RevealNpcCommand{
				CampaignID: tc.campaignID,
				UserID:     tc.userID,
				SheetID:    tc.sheetID,
				Revealed:   true,
			})
			requireAppErr(t, err, tc.wantCode)
		})
	}
}

func TestRevealEvents_CarryNoSheetNameOrValues(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
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
	hidden := linkFixture(campaigntest.NewID(t), base.ID, sh.ID, owner)
	revealed := hidden
	revealed.Revealed = true
	h.links.EXPECT().SetRevealed(mock.Anything, base.ID, sh.ID, true).Return(revealed, true, nil).Once()
	h.links.EXPECT().SetRevealed(mock.Anything, base.ID, sh.ID, false).Return(hidden, true, nil).Once()
	h.expectSave(base.Version, nil)
	h.expectSave(base.Version+1, nil)

	_, err := h.service().SetNpcRevealed(context.Background(), campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: owner, SheetID: sh.ID, Revealed: true,
	})
	require.NoError(t, err)
	_, err = h.service().SetNpcRevealed(context.Background(), campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: owner, SheetID: sh.ID, Revealed: false,
	})
	require.NoError(t, err)
	require.Len(t, h.events, 2)

	assert.Equal(t, campaign.EventSheetRevealed, h.events[0].Type)
	assert.Equal(t, campaign.EventSheetHidden, h.events[1].Type)
	for _, ev := range h.events {
		assert.Equal(t, map[string]any{
			"sheet_id": sh.ID,
			"kind":     "npc",
			"owner_id": owner,
		}, ev.Data)
		rendered := fmt.Sprint(ev.Data)
		assert.NotContains(t, rendered, sh.Name)
		assert.NotContains(t, rendered, "999")
		assert.NotContains(t, ev.Data, "name")
		assert.NotContains(t, ev.Data, "data")
	}
}
