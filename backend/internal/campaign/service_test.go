package campaign_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"
)

var testHasher = mustNewHasher()

func mustNewHasher() campaign.PasswordHasher {
	h, err := campaign.NewArgon2Hasher()
	if err != nil {
		panic(err)
	}
	return h
}

type harness struct {
	t         *testing.T
	repo      *mocks.MockRepository
	tx        *mocks.MockTransactionRunner
	pub       *eventsmocks.MockPublisher
	unlinker  *mocks.MockCharacterUnlinker
	names     *mocks.MockNameLookup
	guard     *mocks.MockCreateGuard
	sheets    *mocks.MockCharacterStore
	limiter   *mocks.MockJoinLimiter
	linked    []sheet.Sheet
	linkedErr error
	events    []events.Event
	saved     *campaign.Campaign
	inserted  *campaign.Campaign
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		t:        t,
		repo:     mocks.NewMockRepository(t),
		tx:       mocks.NewMockTransactionRunner(t),
		pub:      eventsmocks.NewMockPublisher(t),
		unlinker: mocks.NewMockCharacterUnlinker(t),
		names:    mocks.NewMockNameLookup(t),
		guard:    mocks.NewMockCreateGuard(t),
		sheets:   mocks.NewMockCharacterStore(t),
		limiter:  mocks.NewMockJoinLimiter(t),
	}
	h.sheets.EXPECT().ListByCampaignID(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) ([]sheet.Sheet, error) { return h.linked, h.linkedErr },
	).Maybe()
	h.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	).Maybe()
	h.pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
		h.events = append(h.events, ev)
	}).Return().Maybe()
	return h
}

func (h *harness) withUsernames(name string, err error) {
	h.names.EXPECT().Username(mock.Anything, mock.Anything).Return(name, err).Maybe()
}

func (h *harness) service(opts ...campaign.ServiceOption) campaign.Service {
	base := []campaign.ServiceOption{
		campaign.WithCharacterUnlinker(h.unlinker),
		campaign.WithNameLookup(h.names),
		campaign.WithCharacterStore(h.sheets),
		campaign.WithClock(campaigntest.Now),
		campaign.WithPasswordHasher(testHasher),
		campaign.WithJoinLimiter(h.limiter),
	}
	return campaign.NewService(h.repo, h.tx, h.pub, append(base, opts...)...)
}

func (h *harness) loadOnce(stored campaign.Campaign) {
	h.repo.EXPECT().FetchByID(mock.Anything, stored.ID).RunAndReturn(
		func(context.Context, string) (*campaign.Campaign, error) {
			cp := campaigntest.Copy(stored)
			return &cp, nil
		},
	).Once()
}

func (h *harness) loadMissing(id string) {
	h.repo.EXPECT().FetchByID(mock.Anything, id).Return(nil, campaign.ErrNotFound).Once()
}

func (h *harness) expectSave(expectedVersion int64, result error) {
	h.repo.EXPECT().Save(mock.Anything, mock.Anything, expectedVersion).RunAndReturn(
		func(_ context.Context, c *campaign.Campaign, _ int64) error {
			if result == nil {
				h.saved = c
			}
			return result
		},
	).Once()
}

func requireAppErr(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr), "want *apperr.Error, got %T: %v", err, err)
	assert.Equal(t, code, appErr.Code)
}

func TestCreate(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	tests := []struct {
		name     string
		cmd      campaign.CreateCommand
		guardErr error
		insert   error
		wantCode apperr.Code
		insertOK bool
	}{
		{
			name:     "Happy path: creator becomes the owner GM and the campaign is active",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "  Curse of Strahd  ", TemplateID: "dnd5e"},
			insertOK: true,
		},
		{
			name:     "Edge: blank name is rejected",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "   ", TemplateID: "dnd5e"},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: name over 80 runes is rejected",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: strings.Repeat("x", 81), TemplateID: "dnd5e"},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: template id that is not a slug is rejected",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "Ok", TemplateID: "Not A Slug"},
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: missing owner is unauthorized",
			cmd:      campaign.CreateCommand{OwnerID: "", Name: "Ok", TemplateID: "dnd5e"},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:     "Error: plan guard refusal aborts creation without insert",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "Ok", TemplateID: "dnd5e"},
			guardErr: apperr.NewError(apperr.CodePlanLimitReached, "limit", nil),
			wantCode: apperr.CodePlanLimitReached,
		},
		{
			name:     "Error: insert failure surfaces as internal error",
			cmd:      campaign.CreateCommand{OwnerID: ownerID, Name: "Ok", TemplateID: "dnd5e"},
			insert:   errors.New("boom"),
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			opts := []campaign.ServiceOption{}
			if tc.guardErr != nil {
				h.guard.EXPECT().BeforeCreate(mock.Anything, ownerID).Return(tc.guardErr).Once()
				opts = append(opts, campaign.WithCreateGuard(h.guard))
			}
			if tc.insertOK || tc.insert != nil {
				h.repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
					func(_ context.Context, c *campaign.Campaign) error {
						h.inserted = c
						return tc.insert
					},
				).Once()
			}
			view, err := h.service(opts...).Create(context.Background(), tc.cmd)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Empty(t, h.events, "no event may be published for a failed create")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.inserted)
			assert.Equal(t, "Curse of Strahd", h.inserted.Name)
			assert.Equal(t, campaign.StatusActive, h.inserted.Status)
			assert.Equal(t, int64(1), h.inserted.Version)
			assert.Equal(t, 1, h.inserted.GMCount)
			assert.Equal(t, 0, h.inserted.PlayerCount)
			require.Len(t, h.inserted.Members, 1)
			assert.Equal(t, ownerID, h.inserted.Members[0].UserID)
			assert.True(t, h.inserted.Members[0].Owner)
			assert.Equal(t, campaign.RoleGM, h.inserted.Members[0].Role)
			require.Len(t, h.events, 1)
			assert.Equal(t, campaign.EventCreated, h.events[0].Type)
			assert.Equal(t, map[string]any{"template_id": "dnd5e"}, h.events[0].Data)
			assert.Equal(t, "Curse of Strahd", view.Name)
		})
	}
}

func TestFetch(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	deleting := campaigntest.Copy(base)
	deleting.Status = campaign.StatusDeleting

	tests := []struct {
		name     string
		id       string
		user     string
		stored   *campaign.Campaign
		missing  bool
		repoErr  error
		wantCode apperr.Code
	}{
		{name: "Happy path: a player reads the campaign with their role", id: base.ID, user: player, stored: &base},
		{name: "Edge: non-member gets not found", id: base.ID, user: stranger, stored: &base, wantCode: apperr.CodeCampaignNotFound},
		{name: "Edge: a member of a deleting campaign gets not found", id: base.ID, user: player, stored: &deleting, wantCode: apperr.CodeCampaignNotFound},
		{name: "Edge: missing campaign is not found", id: base.ID, user: player, missing: true, wantCode: apperr.CodeCampaignNotFound},
		{name: "Edge: empty campaign id is invalid", id: "", user: player, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: non canonical campaign id is invalid", id: "not-a-uuid", user: player, wantCode: apperr.CodeInvalidRequest},
		{name: "Edge: empty user is unauthorized", id: base.ID, user: "", wantCode: apperr.CodeUnauthorized},
		{name: "Error: repository failure is internal", id: base.ID, user: player, repoErr: errors.New("down"), wantCode: apperr.CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			switch {
			case tc.missing:
				h.loadMissing(tc.id)
			case tc.repoErr != nil:
				h.repo.EXPECT().FetchByID(mock.Anything, tc.id).Return(nil, tc.repoErr).Once()
			case tc.stored != nil:
				h.loadOnce(*tc.stored)
			}
			view, err := h.service().Fetch(context.Background(), tc.id, tc.user)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, base.ID, view.ID)
			assert.Equal(t, campaign.RolePlayer, view.ViewerRole)
			assert.False(t, view.ViewerOwner)
			assert.Len(t, view.Members, 4)
			assert.Equal(t, 2, view.GMCount)
			assert.Equal(t, 2, view.PlayerCount)
		})
	}
}

func TestFetch_UsernameFailureDegradesToEmptyName(t *testing.T) {
	base := campaigntest.Build(t, 0, 0)
	h := newHarness(t)
	h.withUsernames("", errors.New("lookup down"))
	h.loadOnce(base)

	view, err := h.service().Fetch(context.Background(), base.ID, campaigntest.OwnerID(base))
	require.NoError(t, err)
	require.Len(t, view.Members, 1)
	assert.Empty(t, view.Members[0].Username)
}

func TestListMine(t *testing.T) {
	member := campaigntest.NewID(t)
	ownedCampaign := campaigntest.Build(t, 0, 0)
	ownedCampaign.Members[0].UserID = member
	tests := []struct {
		name     string
		user     string
		returned []campaign.Campaign
		repoErr  error
		want     int
		wantCode apperr.Code
	}{
		{name: "Happy path: returns active campaigns with the viewer role", user: member, returned: []campaign.Campaign{ownedCampaign}, want: 1},
		{name: "Edge: no campaigns yields an empty list", user: member, returned: nil, want: 0},
		{name: "Edge: empty user is unauthorized", user: "", wantCode: apperr.CodeUnauthorized},
		{name: "Error: repository failure is internal", user: member, repoErr: errors.New("down"), wantCode: apperr.CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.user != "" {
				if tc.repoErr != nil {
					h.repo.EXPECT().ListActiveByMember(mock.Anything, tc.user).Return(nil, tc.repoErr).Once()
				} else {
					h.repo.EXPECT().ListActiveByMember(mock.Anything, tc.user).Return(tc.returned, nil).Once()
				}
			}
			summaries, err := h.service().ListMine(context.Background(), tc.user)
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Len(t, summaries, tc.want)
			if tc.want == 1 {
				assert.Equal(t, campaign.RoleGM, summaries[0].ViewerRole)
				assert.True(t, summaries[0].ViewerOwner)
			}
		})
	}
}

func TestLeave(t *testing.T) {
	base := campaigntest.Build(t, 0, 2)
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)
	owner := campaigntest.OwnerID(base)
	tests := []struct {
		name     string
		id       string
		user     string
		setup    func(h *harness)
		wantCode apperr.Code
	}{
		{
			name: "Happy path: a player leaves, their character is unlinked and the event is published",
			id:   base.ID,
			user: players[0],
			setup: func(h *harness) {
				h.loadOnce(base)
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, players[0]).Return(nil).Once()
				h.expectSave(1, nil)
			},
		},
		{
			name:     "Error: owner cannot leave",
			id:       base.ID,
			user:     owner,
			setup:    func(h *harness) { h.loadOnce(base) },
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Edge: a user who is not a member gets not found",
			id:       base.ID,
			user:     campaigntest.NewID(t),
			setup:    func(h *harness) { h.loadOnce(base) },
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name: "Error: unlink failure aborts the leave without saving",
			id:   base.ID,
			user: players[0],
			setup: func(h *harness) {
				h.loadOnce(base)
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, players[0]).Return(errors.New("down")).Once()
			},
			wantCode: apperr.CodeInternal,
		},
		{
			name: "Error: concurrent change surfaces as conflict",
			id:   base.ID,
			user: players[0],
			setup: func(h *harness) {
				h.loadOnce(base)
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, players[0]).Return(nil).Once()
				h.expectSave(1, campaign.ErrVersionConflict)
			},
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "Edge: empty campaign id is invalid",
			id:       "",
			user:     players[0],
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: empty user is unauthorized",
			id:       base.ID,
			user:     "",
			wantCode: apperr.CodeUnauthorized,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.setup != nil {
				tc.setup(h)
			}
			err := h.service().Leave(context.Background(), campaign.LeaveCommand{CampaignID: tc.id, UserID: tc.user})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				assert.Empty(t, h.events)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, h.saved)
			assert.Len(t, h.saved.Members, 2)
			_, stillMember := h.saved.MemberOf(players[0])
			assert.False(t, stillMember)
			require.Len(t, h.events, 1)
			assert.Equal(t, campaign.EventMemberLeft, h.events[0].Type)
			assert.Equal(t, map[string]any{"user_id": players[0], "reason": "left"}, h.events[0].Data)
		})
	}
}

func TestPromote(t *testing.T) {
	base := campaigntest.Build(t, 0, 2)
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)
	gmFull := campaigntest.Build(t, campaign.MaxGMs-1, 2)
	gmFullPlayer := campaigntest.MembersWithRole(gmFull, campaign.RolePlayer)[0]
	owner := campaigntest.OwnerID(base)
	gmOf := campaigntest.Build(t, 1, 0)
	existingGM := campaigntest.MembersWithRole(gmOf, campaign.RoleGM)[0]
	gmOnly := campaigntest.Build(t, 1, 0)
	gmActor := campaigntest.MembersWithRole(gmOnly, campaign.RoleGM)[0]

	tests := []struct {
		name     string
		stored   campaign.Campaign
		actor    string
		target   string
		setup    func(h *harness, stored campaign.Campaign)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: owner promotes a player and their character is unlinked",
			stored: base,
			actor:  owner,
			target: players[0],
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, stored.ID, players[0]).Return(nil).Once()
				h.expectSave(1, nil)
			},
		},
		{
			name:     "Error: a GM cannot promote",
			stored:   gmOnly,
			actor:    gmActor,
			target:   campaigntest.OwnerID(gmOnly),
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignForbidden,
		},
		{
			name:     "Error: promoting a GM is invalid",
			stored:   gmOf,
			actor:    campaigntest.OwnerID(gmOf),
			target:   existingGM,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Edge: GM slots at the cap refuse the promotion without unlinking",
			stored:   gmFull,
			actor:    campaigntest.OwnerID(gmFull),
			target:   gmFullPlayer,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignFull,
		},
		{
			name:     "Edge: target who is not a member is not found",
			stored:   base,
			actor:    owner,
			target:   campaigntest.NewID(t),
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeNotFound,
		},
		{
			name:     "Edge: owner cannot promote self",
			stored:   base,
			actor:    owner,
			target:   owner,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignForbidden,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			tc.setup(h, tc.stored)
			_, err := h.service().Promote(context.Background(), campaign.RoleCommand{CampaignID: tc.stored.ID, ActorID: tc.actor, TargetID: tc.target})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			promoted, ok := h.saved.MemberOf(tc.target)
			require.True(t, ok)
			assert.Equal(t, campaign.RoleGM, promoted.Role)
			assert.Equal(t, 2, h.saved.GMCount)
			assert.Equal(t, 1, h.saved.PlayerCount)
			require.Len(t, h.events, 1)
			assert.Equal(t, campaign.EventRoleChanged, h.events[0].Type)
			assert.Equal(t, map[string]any{"user_id": tc.target, "from": "player", "to": "gm"}, h.events[0].Data)
		})
	}
}

func TestDemote(t *testing.T) {
	gmBase := campaigntest.Build(t, 1, 0)
	gm := campaigntest.MembersWithRole(gmBase, campaign.RoleGM)[0]
	playersFull := campaigntest.Build(t, 1, campaign.MaxPlayers)
	gmInFullCampaign := campaigntest.MembersWithRole(playersFull, campaign.RoleGM)[0]
	withPlayer := campaigntest.Build(t, 0, 1)
	player := campaigntest.MembersWithRole(withPlayer, campaign.RolePlayer)[0]

	tests := []struct {
		name     string
		stored   campaign.Campaign
		target   string
		setup    func(h *harness, stored campaign.Campaign)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: owner demotes a GM to player",
			stored: gmBase,
			target: gm,
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.expectSave(1, nil)
			},
		},
		{
			name:     "Edge: player slots at the cap refuse the demotion",
			stored:   playersFull,
			target:   gmInFullCampaign,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignFull,
		},
		{
			name:     "Error: demoting a player is invalid",
			stored:   withPlayer,
			target:   player,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeInvalidRequest,
		},
		{
			name:     "Error: owner cannot demote self",
			stored:   gmBase,
			target:   campaigntest.OwnerID(gmBase),
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignForbidden,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			tc.setup(h, tc.stored)
			actor := campaigntest.OwnerID(tc.stored)
			_, err := h.service().Demote(context.Background(), campaign.RoleCommand{CampaignID: tc.stored.ID, ActorID: actor, TargetID: tc.target})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			demoted, ok := h.saved.MemberOf(tc.target)
			require.True(t, ok)
			assert.Equal(t, campaign.RolePlayer, demoted.Role)
			require.Len(t, h.events, 1)
			assert.Equal(t, map[string]any{"user_id": tc.target, "from": "gm", "to": "player"}, h.events[0].Data)
		})
	}
}

func TestRemove(t *testing.T) {
	base := campaigntest.Build(t, 1, 2)
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)
	gms := campaigntest.MembersWithRole(base, campaign.RoleGM)
	owner := campaigntest.OwnerID(base)

	tests := []struct {
		name     string
		actor    string
		target   string
		wantCode apperr.Code
		wantLeft int
	}{
		{name: "Happy path: a GM removes a player and the player's character is unlinked", actor: gms[0], target: players[0], wantLeft: 3},
		{name: "Happy path: owner removes a GM", actor: owner, target: gms[0], wantLeft: 3},
		{name: "Error: a player cannot remove anyone", actor: players[0], target: players[1], wantCode: apperr.CodeCampaignForbidden},
		{name: "Error: a GM cannot remove another GM", actor: gms[0], target: owner, wantCode: apperr.CodeCampaignForbidden},
		{name: "Error: nobody removes the owner", actor: owner, target: owner, wantCode: apperr.CodeCampaignForbidden},
		{name: "Edge: target who is not a member is not found", actor: owner, target: campaigntest.NewID(t), wantCode: apperr.CodeNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.withUsernames("someone", nil)
			h.loadOnce(base)
			if tc.wantCode == "" {
				h.unlinker.EXPECT().UnlinkCharacter(mock.Anything, base.ID, tc.target).Return(nil).Once()
				h.expectSave(1, nil)
			}
			_, err := h.service().Remove(context.Background(), campaign.RoleCommand{CampaignID: base.ID, ActorID: tc.actor, TargetID: tc.target})
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Len(t, h.saved.Members, tc.wantLeft)
			_, stillMember := h.saved.MemberOf(tc.target)
			assert.False(t, stillMember)
			require.Len(t, h.events, 1)
			assert.Equal(t, map[string]any{"user_id": tc.target, "reason": "removed"}, h.events[0].Data)
		})
	}
}

func TestJoinPlayer(t *testing.T) {
	open := campaigntest.Build(t, 0, campaign.MaxPlayers-1)
	full := campaigntest.Build(t, 0, campaign.MaxPlayers)
	existing := campaigntest.MembersWithRole(open, campaign.RolePlayer)[0]
	deleting := campaigntest.Copy(open)
	deleting.Status = campaign.StatusDeleting

	tests := []struct {
		name     string
		stored   campaign.Campaign
		user     string
		setup    func(h *harness, stored campaign.Campaign)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: a user joins a campaign with a free slot",
			stored: open,
			user:   campaigntest.NewID(t),
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.expectSave(1, nil)
			},
		},
		{
			name:   "Edge: the last free player slot is granted",
			stored: open,
			user:   campaigntest.NewID(t),
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.expectSave(1, nil)
			},
		},
		{
			name:     "Edge: a full campaign refuses a new player",
			stored:   full,
			user:     campaigntest.NewID(t),
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignFull,
		},
		{
			name:     "Edge: an existing member cannot join again",
			stored:   open,
			user:     existing,
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "Edge: a deleting campaign refuses joins",
			stored:   deleting,
			user:     campaigntest.NewID(t),
			setup:    func(h *harness, stored campaign.Campaign) { h.loadOnce(stored) },
			wantCode: apperr.CodeCampaignNotFound,
		},
		{
			name:   "Error: concurrent write conflict surfaces as conflict",
			stored: open,
			user:   campaigntest.NewID(t),
			setup: func(h *harness, stored campaign.Campaign) {
				h.loadOnce(stored)
				h.expectSave(1, campaign.ErrVersionConflict)
			},
			wantCode: apperr.CodeConflict,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.setup(h, tc.stored)
			joined, err := campaign.JoinPlayer(context.Background(), h.tx, h.repo, tc.stored.ID, tc.user, campaigntest.Now())
			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.stored.PlayerCount+1, joined.PlayerCount)
			assert.Equal(t, int64(2), joined.Version)
		})
	}
}
