package campaign_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/shared/apperr"
)

const testInviteCode = "Abcdefghijklmnopqrstuvwx"

func withInvite(c campaign.Campaign, code string) campaign.Campaign {
	out := campaigntest.Copy(c)
	out.InviteCode = code
	return out
}

func withPassword(t *testing.T, c campaign.Campaign, password string) campaign.Campaign {
	t.Helper()
	hash, err := testHasher.Hash(password)
	require.NoError(t, err)
	out := campaigntest.Copy(c)
	out.PasswordHash = hash
	return out
}

func (h *harness) allowJoins(allowed bool, retryAfter time.Duration) {
	h.limiter.EXPECT().Allow(mock.Anything, mock.Anything).Return(allowed, retryAfter).Once()
}

func (h *harness) resolveCode(code string, stored campaign.Campaign) {
	h.repo.EXPECT().FetchByInviteCode(mock.Anything, code).RunAndReturn(
		func(context.Context, string) (*campaign.Campaign, error) {
			cp := campaigntest.Copy(stored)
			return &cp, nil
		},
	).Once()
}

func TestJoinByCode(t *testing.T) {
	open := withInvite(campaigntest.Build(t, 0, 2), testInviteCode)
	protected := withPassword(t, open, "correct horse")
	full := withInvite(campaigntest.Build(t, 0, campaign.MaxPlayers), testInviteCode)
	existing := campaigntest.MembersWithRole(open, campaign.RolePlayer)[0]
	deleting := campaigntest.Copy(open)
	deleting.Status = campaign.StatusDeleting
	rotated := withInvite(open, "ZZZZZZZZZZZZZZZZZZZZZZZZ")

	tests := []struct {
		name         string
		stored       campaign.Campaign
		cmd          campaign.JoinCommand
		asUser       string
		resolve      bool
		limiterAllow *bool
		setup        func(h *harness, stored campaign.Campaign)
		wantCode     apperr.Code
		wantJoin     bool
	}{
		{
			name:     "Happy path: a valid code joins the campaign as a player when no password is set",
			stored:   open,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored); h.expectSave(1, nil) },
			wantJoin: true,
		},
		{
			name:     "Happy path: a password-protected campaign joins with the correct password",
			stored:   protected,
			cmd:      campaign.JoinCommand{Code: testInviteCode, Password: "correct horse"},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored); h.expectSave(1, nil) },
			wantJoin: true,
		},
		{
			name:     "Edge: a password sent to a campaign without one is ignored",
			stored:   open,
			cmd:      campaign.JoinCommand{Code: testInviteCode, Password: "anything at all"},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored); h.expectSave(1, nil) },
			wantJoin: true,
		},
		{
			name:     "Edge: the last free player slot is granted",
			stored:   withInvite(campaigntest.Build(t, 0, campaign.MaxPlayers-1), testInviteCode),
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored); h.expectSave(1, nil) },
			wantJoin: true,
		},
		{
			name:     "Edge: an empty code is an invalid request before any lookup",
			cmd:      campaign.JoinCommand{Code: ""},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: a code with the wrong length is treated as an unknown invite",
			cmd:      campaign.JoinCommand{Code: "short"},
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: a code with characters outside the alphabet is treated as an unknown invite",
			cmd:      campaign.JoinCommand{Code: strings.Repeat("!", 24)},
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: an unknown code is an invalid invite",
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			wantCode: apperr.CodeInvalidInvite,
			setup: func(h *harness, _ campaign.Campaign) {
				h.repo.EXPECT().FetchByInviteCode(mock.Anything, testInviteCode).Return(nil, campaign.ErrNotFound).Once()
			},
		},
		{
			name:     "Edge: a rotated code no longer resolves and is an invalid invite",
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			wantCode: apperr.CodeInvalidInvite,
			setup: func(h *harness, _ campaign.Campaign) {
				h.repo.EXPECT().FetchByInviteCode(mock.Anything, testInviteCode).Return(nil, campaign.ErrNotFound).Once()
			},
		},
		{
			name:     "Edge: a code of a deleting campaign is an invalid invite",
			stored:   deleting,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: a password-protected campaign without a password asks for one",
			stored:   protected,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			wantCode: apperr.CodePasswordRequired,
		},
		{
			name:     "Edge: an oversized password is refused before hashing",
			cmd:      campaign.JoinCommand{Code: testInviteCode, Password: strings.Repeat("p", 513)},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: a wrong password is indistinguishable from an unknown code",
			stored:   protected,
			cmd:      campaign.JoinCommand{Code: testInviteCode, Password: "wrong horse"},
			resolve:  true,
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:         "Error: a resolved code over the per-campaign limit is rate limited",
			stored:       open,
			cmd:          campaign.JoinCommand{Code: testInviteCode},
			resolve:      true,
			limiterAllow: boolPtr(false),
			wantCode:     apperr.CodeRateLimited,
		},
		{
			name:         "Error: a wrong password still counts against the per-campaign limit",
			stored:       protected,
			cmd:          campaign.JoinCommand{Code: testInviteCode, Password: "wrong horse"},
			resolve:      true,
			limiterAllow: boolPtr(true),
			wantCode:     apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: a campaign whose code changed between lookup and transaction is an invalid invite",
			stored:   open,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			setup:    func(h *harness, _ campaign.Campaign) { h.loadOnce(rotated) },
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: a password changed between lookup and transaction is an invalid invite",
			stored:   protected,
			cmd:      campaign.JoinCommand{Code: testInviteCode, Password: "correct horse"},
			resolve:  true,
			setup:    func(h *harness, _ campaign.Campaign) { h.loadOnce(open) },
			wantCode: apperr.CodeInvalidInvite,
		},
		{
			name:     "Edge: an existing member cannot join again with the code",
			stored:   open,
			asUser:   existing,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "Edge: a full campaign refuses the join",
			stored:   full,
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			resolve:  true,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignFull,
		},
		{
			name:    "Error: a concurrent write during the join surfaces as conflict",
			stored:  open,
			cmd:     campaign.JoinCommand{Code: testInviteCode},
			resolve: true,
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.expectSave(1, campaign.ErrVersionConflict)
			},
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "Error: a failed code lookup is an internal error",
			cmd:      campaign.JoinCommand{Code: testInviteCode},
			wantCode: apperr.CodeInternal,
			setup: func(h *harness, _ campaign.Campaign) {
				h.repo.EXPECT().FetchByInviteCode(mock.Anything, testInviteCode).Return(nil, errors.New("db down")).Once()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("", nil)
			if tc.resolve {
				h.resolveCode(tc.cmd.Code, tc.stored)
			}
			if tc.resolve && tc.stored.Active() {
				allowed := true
				if tc.limiterAllow != nil {
					allowed = *tc.limiterAllow
				}
				h.allowJoins(allowed, time.Minute)
			}
			if tc.setup != nil {
				tc.setup(h, tc.stored)
			}
			cmd := tc.cmd
			cmd.UserID = tc.asUser
			if cmd.UserID == "" {
				cmd.UserID = campaigntest.NewID(t)
			}
			view, err := h.service().JoinByCode(context.Background(), cmd)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved, "a rejected join must not persist")
				assert.Empty(t, h.events, "a rejected join must not publish an event")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, campaign.RolePlayer, view.ViewerRole)
			assert.Empty(t, view.InviteCode, "a joining player must not see the invite code")
			assert.Nil(t, view.PasswordSet)
			require.Len(t, h.events, 1)
			assert.Equal(t, campaign.EventMemberJoined, h.events[0].Type)
		})
	}
}

func TestJoinByCode_EventNeverCarriesCodeOrPassword(t *testing.T) {
	stored := withPassword(t, withInvite(campaigntest.Build(t, 0, 0), testInviteCode), "correct horse")
	h := newHarness(t)
	h.withUsernames("", nil)
	h.resolveCode(testInviteCode, stored)
	h.allowJoins(true, time.Minute)
	h.loadOnce(stored)
	h.expectSave(1, nil)
	_, err := h.service().JoinByCode(context.Background(), campaign.JoinCommand{
		UserID:   campaigntest.NewID(t),
		Code:     testInviteCode,
		Password: "correct horse",
	})
	require.NoError(t, err)
	require.Len(t, h.events, 1)
	payload, err := json.Marshal(h.events[0])
	require.NoError(t, err)
	assert.NotContains(t, string(payload), testInviteCode)
	assert.NotContains(t, string(payload), "correct horse")
	assert.NotContains(t, string(payload), stored.PasswordHash)
}

func TestJoinByCode_UnknownCodeDoesNotConsumeCampaignLimit(t *testing.T) {
	h := newHarness(t)
	h.withUsernames("", nil)
	h.repo.EXPECT().FetchByInviteCode(mock.Anything, testInviteCode).Return(nil, campaign.ErrNotFound).Once()
	_, err := h.service().JoinByCode(context.Background(), campaign.JoinCommand{UserID: campaigntest.NewID(t), Code: testInviteCode})
	requireAppErr(t, err, apperr.CodeInvalidInvite)
	h.limiter.AssertNotCalled(t, "Allow", mock.Anything, mock.Anything)
}

func TestJoinByCode_RequiresAuthenticatedUser(t *testing.T) {
	h := newHarness(t)
	h.withUsernames("", nil)
	_, err := h.service().JoinByCode(context.Background(), campaign.JoinCommand{Code: testInviteCode})
	requireAppErr(t, err, apperr.CodeUnauthorized)
}

func TestCreate_IssuesInviteCodeVisibleToOwnerOnly(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	h := newHarness(t)
	h.withUsernames("", nil)
	h.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
	h.withUsernames("", nil)
	view, err := h.service().Create(context.Background(), campaign.CreateCommand{OwnerID: ownerID, Name: "Ok", TemplateID: "dnd5e"})
	require.NoError(t, err)
	require.Len(t, view.InviteCode, 24)
	require.NotNil(t, view.PasswordSet)
	assert.False(t, *view.PasswordSet)
}

func TestRotateInviteCode(t *testing.T) {
	base := withInvite(campaigntest.Build(t, 1, 1), testInviteCode)
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	tests := []struct {
		name     string
		actor    string
		stored   campaign.Campaign
		setup    func(h *harness)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: the owner rotates the code and the old one stops resolving",
			actor:  campaigntest.OwnerID(base),
			stored: base,
		},
		{
			name:   "Happy path: any GM rotates the code",
			actor:  gm,
			stored: base,
		},
		{
			name:     "Error: a player cannot rotate the code",
			actor:    player,
			stored:   base,
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Edge: a non-member gets not found",
			actor:    campaigntest.NewID(t),
			stored:   base,
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:     "Edge: a deleting campaign refuses rotation",
			actor:    campaigntest.OwnerID(base),
			stored:   deletingCopy(base),
			wantCode: apperr.CodeCampaignNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("", nil)
			h.loadOnce(tc.stored)
			if tc.wantCode == "" {
				h.expectSave(tc.stored.Version, nil)
			}
			view, err := h.service().RotateInviteCode(context.Background(), campaign.InviteCommand{CampaignID: base.ID, ActorID: tc.actor})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			assert.NotEqual(t, testInviteCode, h.saved.InviteCode)
			assert.Len(t, h.saved.InviteCode, 24)
			assert.Equal(t, tc.stored.Version+1, h.saved.Version)
			assert.Equal(t, h.saved.InviteCode, view.InviteCode, "a GM sees the new code")
			assert.Empty(t, h.events, "rotation is not a catalogued event")
		})
	}
}

func TestRotateInviteCode_ProducesDistinctCodes(t *testing.T) {
	base := withInvite(campaigntest.Build(t, 0, 0), testInviteCode)
	seen := map[string]struct{}{}
	for range 25 {
		h := newHarness(t)
		h.withUsernames("", nil)
		h.loadOnce(base)
		h.expectSave(1, nil)
		_, err := h.service().RotateInviteCode(context.Background(), campaign.InviteCommand{CampaignID: base.ID, ActorID: campaigntest.OwnerID(base)})
		require.NoError(t, err)
		_, dup := seen[h.saved.InviteCode]
		require.False(t, dup, "rotated invite codes must not repeat")
		seen[h.saved.InviteCode] = struct{}{}
	}
}

func TestSetPassword(t *testing.T) {
	base := withInvite(campaigntest.Build(t, 1, 1), testInviteCode)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	tests := []struct {
		name     string
		actor    string
		password string
		wantCode apperr.Code
	}{
		{
			name:     "Happy path: the owner sets a password",
			actor:    campaigntest.OwnerID(base),
			password: "correct horse",
		},
		{
			name:     "Happy path: a GM sets a password",
			actor:    campaigntest.MembersWithRole(base, campaign.RoleGM)[0],
			password: "correct horse",
		},
		{
			name:     "Edge: a password of exactly 8 characters is accepted",
			actor:    campaigntest.OwnerID(base),
			password: strings.Repeat("a", 8),
		},
		{
			name:     "Edge: a password of exactly 128 characters is accepted",
			actor:    campaigntest.OwnerID(base),
			password: strings.Repeat("a", 128),
		},
		{
			name:     "Edge: 8 multi-byte characters are accepted",
			actor:    campaigntest.OwnerID(base),
			password: strings.Repeat("á", 8),
		},
		{
			name:     "Edge: a password of 7 characters is refused",
			actor:    campaigntest.OwnerID(base),
			password: strings.Repeat("a", 7),
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: a password of 129 characters is refused",
			actor:    campaigntest.OwnerID(base),
			password: strings.Repeat("a", 129),
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: an empty password is refused",
			actor:    campaigntest.OwnerID(base),
			password: "",
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Error: a player cannot set a password",
			actor:    player,
			password: "correct horse",
			wantCode: apperr.CodeCampaignForbidden,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("", nil)
			if tc.wantCode == "" || tc.wantCode == apperr.CodeCampaignForbidden {
				h.loadOnce(base)
			}
			if tc.wantCode == "" {
				h.expectSave(1, nil)
			}
			view, err := h.service().SetPassword(context.Background(), campaign.PasswordCommand{CampaignID: base.ID, ActorID: tc.actor, Password: tc.password})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			assert.True(t, testHasher.Verify(h.saved.PasswordHash, tc.password), "the stored hash must verify the password")
			assert.NotContains(t, h.saved.PasswordHash, tc.password)
			require.NotNil(t, view.PasswordSet)
			assert.True(t, *view.PasswordSet)
			assert.Equal(t, h.saved.InviteCode, view.InviteCode, "the code is unchanged by a password change")
			assert.Equal(t, base.Members, h.saved.Members[:len(base.Members)], "a password change removes no member")
		})
	}
}

func TestClearPassword(t *testing.T) {
	protected := withPassword(t, withInvite(campaigntest.Build(t, 1, 1), testInviteCode), "correct horse")
	player := campaigntest.MembersWithRole(protected, campaign.RolePlayer)[0]
	tests := []struct {
		name     string
		actor    string
		wantCode apperr.Code
	}{
		{name: "Happy path: a GM clears the password", actor: campaigntest.OwnerID(protected)},
		{name: "Error: a player cannot clear the password", actor: player, wantCode: apperr.CodeCampaignForbidden},
		{name: "Edge: a non-member gets not found", actor: campaigntest.NewID(t), wantCode: apperr.CodeCampaignNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("", nil)
			h.loadOnce(protected)
			if tc.wantCode == "" {
				h.expectSave(1, nil)
			}
			view, err := h.service().ClearPassword(context.Background(), campaign.InviteCommand{CampaignID: protected.ID, ActorID: tc.actor})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Nil(t, h.saved)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, h.saved.PasswordHash)
			require.NotNil(t, view.PasswordSet)
			assert.False(t, *view.PasswordSet)
		})
	}
}

func TestClearPassword_IsIdempotentOnUnprotectedCampaign(t *testing.T) {
	open := withInvite(campaigntest.Build(t, 0, 0), testInviteCode)
	h := newHarness(t)
	h.withUsernames("", nil)
	h.loadOnce(open)
	h.expectSave(1, nil)
	_, err := h.service().ClearPassword(context.Background(), campaign.InviteCommand{CampaignID: open.ID, ActorID: campaigntest.OwnerID(open)})
	require.NoError(t, err)
	assert.Empty(t, h.saved.PasswordHash)
}

func TestFetch_InviteCodeExposedOnlyToGMs(t *testing.T) {
	base := withPassword(t, withInvite(campaigntest.Build(t, 1, 1), testInviteCode), "correct horse")
	tests := []struct {
		name         string
		viewer       string
		wantCode     string
		wantPassword bool
	}{
		{name: "Happy path: the owner sees the code and that a password is set", viewer: campaigntest.OwnerID(base), wantCode: testInviteCode, wantPassword: true},
		{name: "Happy path: a GM sees the code", viewer: campaigntest.MembersWithRole(base, campaign.RoleGM)[0], wantCode: testInviteCode, wantPassword: true},
		{name: "Edge: a player never sees the code or the password state", viewer: campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("", nil)
			h.loadOnce(base)
			h.withUsernames("someone", nil)
			view, err := h.service().Fetch(context.Background(), base.ID, tc.viewer)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCode, view.InviteCode)
			if tc.wantCode == "" {
				assert.Nil(t, view.PasswordSet)
				return
			}
			require.NotNil(t, view.PasswordSet)
			assert.Equal(t, tc.wantPassword, *view.PasswordSet)
		})
	}
}

func deletingCopy(c campaign.Campaign) campaign.Campaign {
	out := campaigntest.Copy(c)
	out.Status = campaign.StatusDeleting
	return out
}

func boolPtr(b bool) *bool {
	return &b
}
