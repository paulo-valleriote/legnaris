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

func TestGmExit_DeletesOnlyThatOwnersLinks(t *testing.T) {
	base := campaigntest.Build(t, 1, 1)
	owner := campaigntest.OwnerID(base)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	sh := npcSheet(t, coGM, base.TemplateID, plan.KindEnemy)

	tests := []struct {
		name      string
		target    string
		actor     string
		run       func(ctx context.Context, h *harness) error
		removed   []campaign.SheetLink
		storeErr  error
		character bool
		wantCode  apperr.Code
		wantType  string
		wantLink  bool
	}{
		{
			name:   "Happy path: a GM who leaves loses their links in this campaign",
			target: coGM,
			actor:  coGM,
			run: func(ctx context.Context, h *harness) error {
				return h.service().Leave(ctx, campaign.LeaveCommand{CampaignID: base.ID, UserID: coGM})
			},
			removed:   []campaign.SheetLink{linkFixture(campaigntest.NewID(t), base.ID, sh.ID, coGM)},
			character: true,
			wantType:  campaign.EventMemberLeft,
			wantLink:  true,
		},
		{
			name:   "Happy path: a GM demoted to player loses their links in this campaign",
			target: coGM,
			actor:  owner,
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Demote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: owner, TargetID: coGM})
				return err
			},
			removed:  []campaign.SheetLink{linkFixture(campaigntest.NewID(t), base.ID, sh.ID, coGM)},
			wantType: campaign.EventRoleChanged,
			wantLink: true,
		},
		{
			name:   "Happy path: a removed GM loses their links in this campaign",
			target: coGM,
			actor:  owner,
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Remove(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: owner, TargetID: coGM})
				return err
			},
			removed:   []campaign.SheetLink{linkFixture(campaigntest.NewID(t), base.ID, sh.ID, coGM)},
			character: true,
			wantType:  campaign.EventMemberLeft,
			wantLink:  true,
		},
		{
			name:   "Edge: a player who leaves has no links to remove and emits no unlink event",
			target: player,
			actor:  player,
			run: func(ctx context.Context, h *harness) error {
				return h.service().Leave(ctx, campaign.LeaveCommand{CampaignID: base.ID, UserID: player})
			},
			character: true,
			wantType:  campaign.EventMemberLeft,
		},
		{
			name:   "Error: a failed link deletion aborts the demotion without saving",
			target: coGM,
			actor:  owner,
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Demote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: owner, TargetID: coGM})
				return err
			},
			storeErr: errStoreDown,
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadOnce(base)
			h.links.EXPECT().DeleteByOwner(mock.Anything, base.ID, tc.target).Return(tc.removed, tc.storeErr).Once()
			if tc.storeErr == nil {
				if tc.character {
					h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, tc.target).Return(nil).Once()
				}
				h.expectSave(base.Version, nil)
			}
			if tc.wantLink {
				h.sheets.EXPECT().FetchByID(mock.Anything, sh.ID).Return(&sh, nil).Once()
			}

			err := tc.run(context.Background(), h)

			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved, "a rejected exit must not reach the save")
				assert.Empty(t, h.events)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			wantEvents := 1
			if tc.wantLink {
				wantEvents = 2
			}
			require.Len(t, h.events, wantEvents)
			assert.Equal(t, tc.wantType, h.events[0].Type)
			if !tc.wantLink {
				return
			}
			unlinked := h.events[1]
			require.Equal(t, campaign.EventSheetUnlinked, unlinked.Type)
			require.NotNil(t, unlinked.ActorID)
			assert.Equal(t, tc.actor, *unlinked.ActorID)
			assert.Equal(t, base.ID, unlinked.AggregateID)
			assert.Equal(t, base.Version+1, unlinked.AggregateVersion)
			data := eventData(t, unlinked)
			assert.Equal(t, map[string]any{
				"sheet_id": sh.ID,
				"kind":     string(plan.KindEnemy),
				"owner_id": coGM,
				"reason":   campaign.ReasonOwnerLeft,
			}, data)
			rendered := fmt.Sprint(data)
			assert.NotContains(t, rendered, sh.Name)
			assert.NotContains(t, data, "name")
			assert.NotContains(t, data, "data")
		})
	}
}

func TestGmExit_MissingSheetSkipsTheUnlinkEvent(t *testing.T) {
	base := campaigntest.Build(t, 1, 0)
	coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	owner := campaigntest.OwnerID(base)
	removed := []campaign.SheetLink{linkFixture(campaigntest.NewID(t), base.ID, campaigntest.NewID(t), coGM)}

	tests := []struct {
		name     string
		sheetErr error
	}{
		{name: "Edge: a link whose sheet is already gone emits nothing", sheetErr: sheeterr.ErrSheetNotFound},
		{name: "Error: a sheet read failure does not fail the demotion and emits nothing", sheetErr: errStoreDown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadOnce(base)
			h.links.EXPECT().DeleteByOwner(mock.Anything, base.ID, coGM).Return(removed, nil).Once()
			h.sheets.EXPECT().FetchByID(mock.Anything, removed[0].SheetID).Return(nil, tc.sheetErr).Once()
			h.expectSave(base.Version, nil)

			_, err := h.service().Demote(context.Background(), campaign.RoleCommand{CampaignID: base.ID, ActorID: owner, TargetID: coGM})
			require.NoError(t, err)
			require.Len(t, h.events, 1)
			assert.Equal(t, campaign.EventRoleChanged, h.events[0].Type)
		})
	}
}

func TestNpcReads_TolerateMissingCampaign(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	owner := campaigntest.OwnerID(base)
	sh := npcSheet(t, owner, base.TemplateID, plan.KindNPC)

	tests := []struct {
		name string
		run  func(ctx context.Context, h *harness) error
	}{
		{
			name: "Edge: listing links of a missing campaign is not found without reading links",
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().ListNpcLinks(ctx, base.ID, owner)
				return err
			},
		},
		{
			name: "Edge: reading a linked NPC of a missing campaign is not found without reading links",
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().ReadNpcSheet(ctx, campaign.ReadNpcSheetQuery{CampaignID: base.ID, ViewerID: owner, SheetID: sh.ID})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadMissing(base.ID)

			requireAppErr(t, tc.run(context.Background(), h), apperr.CodeCampaignNotFound)
		})
	}
}
