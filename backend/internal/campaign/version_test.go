package campaign_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/plan"
)

type campaignWrite struct {
	name    string
	base    campaign.Campaign
	prepare func(h *harness, base campaign.Campaign)
	run     func(ctx context.Context, h *harness, base campaign.Campaign) error
}

func successfulCampaignWrites(t *testing.T) []campaignWrite {
	t.Helper()
	return []campaignWrite{
		{
			name: "join",
			base: campaigntest.Build(t, 0, 2),
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := campaign.JoinPlayer(ctx, h.tx, h.repo, base.ID, campaigntest.NewID(t), campaigntest.Now())
				return err
			},
		},
		{
			name: "leave",
			base: campaigntest.Build(t, 0, 2),
			prepare: func(h *harness, base campaign.Campaign) {
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]).Return(nil).Once()
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				return h.service().Leave(ctx, campaign.LeaveCommand{CampaignID: base.ID, UserID: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]})
			},
		},
		{
			name: "link",
			base: campaigntest.Build(t, 0, 2),
			prepare: func(h *harness, base campaign.Campaign) {
				player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
				sh := sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)
				h.sheets.EXPECT().FetchByID(mock.Anything, mock.Anything).Return(&sh, nil).Once()
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(0), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, mock.Anything, (*string)(nil), base.ID).Return(true, nil).Once()
				h.withUsernames("someone", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
				_, err := h.service().Link(ctx, campaign.LinkCommand{CampaignID: base.ID, UserID: player, SheetID: campaigntest.NewID(t)})
				return err
			},
		},
		{
			name: "unlink",
			base: campaigntest.Build(t, 0, 2),
			prepare: func(h *harness, base campaign.Campaign) {
				player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(1), nil).Once()
				h.withUsernames("someone", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().Unlink(ctx, campaign.UnlinkCommand{CampaignID: base.ID, UserID: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]})
				return err
			},
		},
		{
			name: "promote",
			base: campaigntest.Build(t, 0, 2),
			prepare: func(h *harness, base campaign.Campaign) {
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]).Return(nil).Once()
				h.withUsernames("someone", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().Promote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base), TargetID: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]})
				return err
			},
		},
		{
			name: "demote",
			base: campaigntest.Build(t, 1, 0),
			prepare: func(h *harness, _ campaign.Campaign) {
				h.withUsernames("someone", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().Demote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base), TargetID: campaigntest.MembersWithRole(base, campaign.RoleGM)[0]})
				return err
			},
		},
		{
			name: "join by code",
			base: withInvite(campaigntest.Build(t, 0, 2), testInviteCode),
			prepare: func(h *harness, base campaign.Campaign) {
				h.resolveCode(testInviteCode, base)
				h.allowJoins(true, time.Minute)
				h.withUsernames("", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().JoinByCode(ctx, campaign.JoinCommand{UserID: campaigntest.NewID(t), Code: testInviteCode})
				return err
			},
		},
		{
			name: "rotate invite code",
			base: withInvite(campaigntest.Build(t, 0, 2), testInviteCode),
			prepare: func(h *harness, _ campaign.Campaign) {
				h.withUsernames("", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().RotateInviteCode(ctx, campaign.InviteCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base)})
				return err
			},
		},
		{
			name: "set password",
			base: withInvite(campaigntest.Build(t, 0, 2), testInviteCode),
			prepare: func(h *harness, _ campaign.Campaign) {
				h.withUsernames("", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().SetPassword(ctx, campaign.PasswordCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base), Password: "correct horse"})
				return err
			},
		},
		{
			name: "clear password",
			base: withPassword(t, withInvite(campaigntest.Build(t, 0, 2), testInviteCode), "correct horse"),
			prepare: func(h *harness, _ campaign.Campaign) {
				h.withUsernames("", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().ClearPassword(ctx, campaign.InviteCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base)})
				return err
			},
		},
		{
			name: "remove",
			base: campaigntest.Build(t, 0, 2),
			prepare: func(h *harness, base campaign.Campaign) {
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]).Return(nil).Once()
				h.withUsernames("someone", nil)
			},
			run: func(ctx context.Context, h *harness, base campaign.Campaign) error {
				_, err := h.service().Remove(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base), TargetID: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]})
				return err
			},
		},
	}
}

func TestCampaignWrites_EverySuccessfulWriteIncrementsVersionByExactlyOne(t *testing.T) {
	for _, tc := range successfulCampaignWrites(t) {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadOnce(tc.base)
			h.expectSave(tc.base.Version, nil)
			if tc.prepare != nil {
				tc.prepare(h, tc.base)
			}
			require.NoError(t, tc.run(context.Background(), h, tc.base))
			require.NotNil(t, h.saved, "a successful Campaign write must persist through the version-checked save")
			require.Equal(t, tc.base.Version+1, h.saved.Version, "campaign.version must advance by exactly one per transaction")
		})
	}
}

func TestCampaignWrites_FailedWritesPersistNothing(t *testing.T) {
	base := campaigntest.Build(t, 0, 2)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	owner := campaigntest.OwnerID(base)
	tests := []struct {
		name string
		run  func(ctx context.Context, h *harness) error
	}{
		{
			name: "join by an existing member",
			run: func(ctx context.Context, h *harness) error {
				_, err := campaign.JoinPlayer(ctx, h.tx, h.repo, base.ID, player, campaigntest.Now())
				return err
			},
		},
		{
			name: "owner leave",
			run: func(ctx context.Context, h *harness) error {
				return h.service().Leave(ctx, campaign.LeaveCommand{CampaignID: base.ID, UserID: owner})
			},
		},
		{
			name: "player promote",
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Promote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: player, TargetID: owner})
				return err
			},
		},
		{
			name: "player demote",
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Demote(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: player, TargetID: owner})
				return err
			},
		},
		{
			name: "player remove",
			run: func(ctx context.Context, h *harness) error {
				_, err := h.service().Remove(ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: player, TargetID: owner})
				return err
			},
		},
		{
			name: "link with a template mismatch",
			run: func(ctx context.Context, h *harness) error {
				sh := sheetOf(t, player, "other_template", plan.KindCharacter, nil)
				h.sheetReturns(sh)
				_, err := h.service().Link(ctx, campaign.LinkCommand{CampaignID: base.ID, UserID: player, SheetID: sh.ID})
				return err
			},
		},
		{
			name: "link lost the compare-and-set after unlinking the previous character",
			run: func(ctx context.Context, h *harness) error {
				sh := sheetOf(t, player, base.TemplateID, plan.KindCharacter, nil)
				h.sheetReturns(sh)
				h.sheets.EXPECT().UnlinkOwnerCampaign(mock.Anything, player, base.ID).Return(int64(1), nil).Once()
				h.sheets.EXPECT().LinkCampaign(mock.Anything, sh.ID, (*string)(nil), base.ID).Return(false, nil).Once()
				_, err := h.service().Link(ctx, campaign.LinkCommand{CampaignID: base.ID, UserID: player, SheetID: sh.ID})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.loadOnce(base)
			require.Error(t, tc.run(context.Background(), h))
			require.Nil(t, h.saved, "a rejected write must not reach the save")
		})
	}
}
