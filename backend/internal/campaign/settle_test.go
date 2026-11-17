package campaign_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/events"
	"legnaris/backend/shared/apperr"
)

var errInjected = errors.New("injected failure")

type memRepo struct {
	docs        map[string]campaign.Campaign
	failWriteAt int
	writes      int
}

func newMemRepo(cs ...campaign.Campaign) *memRepo {
	r := &memRepo{docs: map[string]campaign.Campaign{}}
	for _, c := range cs {
		r.docs[c.ID] = campaigntest.Copy(c)
	}
	return r
}

func (r *memRepo) Insert(_ context.Context, c *campaign.Campaign) error {
	r.docs[c.ID] = campaigntest.Copy(*c)
	return nil
}

func (r *memRepo) FetchByID(_ context.Context, id string) (*campaign.Campaign, error) {
	c, ok := r.docs[id]
	if !ok {
		return nil, campaign.ErrNotFound
	}
	cp := campaigntest.Copy(c)
	return &cp, nil
}

func (r *memRepo) FetchByInviteCode(context.Context, string) (*campaign.Campaign, error) {
	return nil, campaign.ErrNotFound
}

func (r *memRepo) ListActiveByMember(_ context.Context, userID string) ([]campaign.Campaign, error) {
	ids := make([]string, 0, len(r.docs))
	for id := range r.docs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []campaign.Campaign
	for _, id := range ids {
		c := r.docs[id]
		if _, ok := c.MemberOf(userID); ok && c.Active() {
			out = append(out, campaigntest.Copy(c))
		}
	}
	return out, nil
}

func (r *memRepo) Save(_ context.Context, c *campaign.Campaign, expected int64) error {
	return r.write(c, expected)
}

func (r *memRepo) MarkDeleting(_ context.Context, c *campaign.Campaign, expected int64) error {
	return r.write(c, expected)
}

func (r *memRepo) write(c *campaign.Campaign, expected int64) error {
	r.writes++
	if r.writes == r.failWriteAt {
		return campaign.ErrVersionConflict
	}
	stored, ok := r.docs[c.ID]
	if !ok || stored.Version != expected || !stored.Active() {
		return campaign.ErrVersionConflict
	}
	r.docs[c.ID] = campaigntest.Copy(*c)
	return nil
}

type fakeOwnership struct {
	counts      map[string]int
	limits      map[string]int
	releases    int
	failRelease bool
	reserveErr  error
}

func (o *fakeOwnership) ReserveOwnership(_ context.Context, userID string) error {
	if o.reserveErr != nil {
		return o.reserveErr
	}
	if limit, capped := o.limits[userID]; capped && o.counts[userID] >= limit {
		return apperr.NewError(apperr.CodePlanLimitReached, "owned campaign limit reached", campaign.ErrOwnershipLimitReached)
	}
	o.counts[userID]++
	return nil
}

func (o *fakeOwnership) ReleaseOwnership(_ context.Context, userID string) error {
	if o.failRelease {
		return errInjected
	}
	o.releases++
	if o.counts[userID] > 0 {
		o.counts[userID]--
	}
	return nil
}

type fakeUnlinker struct {
	unlinked []string
	fail     bool
}

func (u *fakeUnlinker) UnlinkCharacter(_ context.Context, _, userID string) error {
	if u.fail {
		return errInjected
	}
	u.unlinked = append(u.unlinked, userID)
	return nil
}

type settlePublisher struct {
	events []events.Event
}

func (p *settlePublisher) Publish(ev events.Event) {
	p.events = append(p.events, ev)
}

func (p *settlePublisher) types() []string {
	out := make([]string, 0, len(p.events))
	for _, ev := range p.events {
		out = append(out, ev.Type)
	}
	return out
}

type settleWorld struct {
	repo      *memRepo
	ownership *fakeOwnership
	unlinker  *fakeUnlinker
	publisher *settlePublisher
}

func newSettleWorld(campaigns []campaign.Campaign, owned, limits map[string]int) *settleWorld {
	counts := map[string]int{}
	for id, n := range owned {
		counts[id] = n
	}
	return &settleWorld{
		repo:      newMemRepo(campaigns...),
		ownership: &fakeOwnership{counts: counts, limits: limits},
		unlinker:  &fakeUnlinker{},
		publisher: &settlePublisher{},
	}
}

func (w *settleWorld) settler(t *testing.T) *campaign.AccountSettler {
	t.Helper()
	s, err := campaign.NewAccountSettler(w.repo, passTx{}, w.ownership, w.unlinker, w.publisher, campaigntest.Now)
	require.NoError(t, err)
	return s
}

type passTx struct{}

func (passTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func gmMember(userID string, joined time.Time, owner bool) campaign.Member {
	return campaign.Member{UserID: userID, Role: campaign.RoleGM, Owner: owner, JoinedAt: joined}
}

func playerMember(userID string, joined time.Time) campaign.Member {
	return campaign.Member{UserID: userID, Role: campaign.RolePlayer, JoinedAt: joined}
}

func campaignWith(t *testing.T, members ...campaign.Member) campaign.Campaign {
	t.Helper()
	c := campaign.Campaign{
		ID:         campaigntest.NewID(t),
		Name:       "Curse of Strahd",
		TemplateID: "dnd5e",
		Status:     campaign.StatusActive,
		Version:    1,
		Members:    members,
		CreatedAt:  campaigntest.Now(),
		UpdatedAt:  campaigntest.Now(),
	}
	for _, m := range members {
		if m.Role == campaign.RoleGM {
			c.GMCount++
		} else {
			c.PlayerCount++
		}
	}
	return c
}

func at(hours int) time.Time {
	return campaigntest.Now().Add(time.Duration(hours) * time.Hour)
}

func TestAccountSettler_SettleAccount(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	olderGM := campaigntest.NewID(t)
	newerGM := campaigntest.NewID(t)
	player := campaigntest.NewID(t)
	smallerID, largerID := orderedPair(t)

	tests := []struct {
		name        string
		failUnlink  bool
		failRelease bool
		failWriteAt int
		reserveErr  error
		build       func(t *testing.T) (campaigns []campaign.Campaign, owned, limits map[string]int, user string)
		check       func(t *testing.T, w *settleWorld, campaigns []campaign.Campaign, err error)
	}{
		{
			name: "Happy path: owner hands off to the only other GM and leaves",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, olderGM, campaigntest.OwnerID(*stored))
				assert.Len(t, stored.Members, 1)
				assert.Equal(t, int64(3), stored.Version, "hand-off and leave are two transactions")
				assert.Equal(t, 1, stored.GMCount)
				assert.Equal(t, []string{ownerID}, w.unlinker.unlinked)
				assert.Equal(t, 0, w.ownership.counts[ownerID], "the leaving owner's slot is released")
				assert.Equal(t, 1, w.ownership.counts[olderGM], "the successor takes a slot")
				assert.Equal(t, []string{campaign.EventOwnershipTransferred, campaign.EventMemberLeft}, w.publisher.types())
			},
		},
		{
			name: "Happy path: the longest-standing GM takes ownership, not the first in the list",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(newerGM, at(5), false), gmMember(olderGM, at(2), false))
				return []campaign.Campaign{c}, nil, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, olderGM, campaigntest.OwnerID(*stored))
			},
		},
		{
			name: "Edge: a join-time tie goes to the smaller user id whatever the member order",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(largerID, at(1), false), gmMember(smallerID, at(1), false))
				return []campaign.Campaign{c}, nil, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, smallerID, campaigntest.OwnerID(*stored))
			},
		},
		{
			name: "Edge: a GM at the owned-campaign limit is skipped for the next GM",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false), gmMember(newerGM, at(2), false))
				return []campaign.Campaign{c}, nil, map[string]int{olderGM: 0}, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, newerGM, campaigntest.OwnerID(*stored))
				assert.Equal(t, campaign.StatusActive, stored.Status)
			},
		},
		{
			name: "Edge: when no GM can take ownership the campaign moves to deleting and the owner's slot is released",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false), playerMember(player, at(2)))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, map[string]int{olderGM: 0}, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, campaign.StatusDeleting, stored.Status)
				assert.Equal(t, 1, w.ownership.releases)
				assert.Equal(t, 0, w.ownership.counts[ownerID])
				assert.Empty(t, w.unlinker.unlinked, "unlinking Sheets is left to the deletion queue")
				assert.Empty(t, w.publisher.events)
			},
		},
		{
			name: "Edge: an owner alone in the campaign moves it to deleting",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, campaign.StatusDeleting, stored.Status)
				assert.Equal(t, 0, w.ownership.counts[ownerID])
			},
		},
		{
			name: "Happy path: a player is unlinked and removed from the campaign",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), playerMember(player, at(1)))
				return []campaign.Campaign{c}, nil, nil, player
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, 0, stored.PlayerCount)
				assert.Equal(t, int64(2), stored.Version)
				assert.Equal(t, []string{player}, w.unlinker.unlinked)
				assert.Equal(t, []string{campaign.EventMemberLeft}, w.publisher.types())
				assert.Equal(t, 0, w.ownership.releases, "a non-owner does not touch the owned-campaign counter")
			},
		},
		{
			name: "Happy path: a non-owner GM leaves and the GM count is recounted",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, nil, nil, olderGM
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, 1, stored.GMCount)
				assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
			},
		},
		{
			name: "Edge: a user in no campaign writes nothing",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true))
				return []campaign.Campaign{c}, nil, nil, campaigntest.NewID(t)
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, int64(1), stored.Version)
				assert.Empty(t, w.publisher.events)
				assert.Empty(t, w.unlinker.unlinked)
			},
		},
		{
			name: "Edge: a campaign already in deleting is skipped",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				c.Status = campaign.StatusDeleting
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, int64(1), stored.Version)
				assert.Equal(t, 0, w.ownership.releases)
				assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
			},
		},
		{
			name: "Edge: a user in several campaigns settles each one",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				owned := campaignWith(t, gmMember(ownerID, at(0), true))
				joined := campaignWith(t, gmMember(olderGM, at(0), true), playerMember(ownerID, at(1)))
				return []campaign.Campaign{owned, joined}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				require.NoError(t, err)
				owned, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				joined, _ := w.repo.FetchByID(context.Background(), cs[1].ID)
				assert.Equal(t, campaign.StatusDeleting, owned.Status)
				assert.Equal(t, 0, joined.PlayerCount)
				assert.Equal(t, []string{ownerID}, w.unlinker.unlinked)
			},
		},
		{
			name: "Error: an empty user id is rejected as unauthorized",
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				return nil, nil, nil, ""
			},
			check: func(t *testing.T, _ *settleWorld, _ []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeUnauthorized)
			},
		},
		{
			name:        "Error: a failed release leaves the campaign untouched",
			failRelease: true,
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeInternal)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
				assert.Equal(t, int64(1), stored.Version)
				assert.Empty(t, w.publisher.events)
			},
		},
		{
			name:       "Error: a failed unlink during leave rolls back the leave",
			failUnlink: true,
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), playerMember(player, at(1)))
				return []campaign.Campaign{c}, nil, nil, player
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeInternal)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, 1, stored.PlayerCount)
				assert.Equal(t, int64(1), stored.Version)
			},
		},
		{
			name:        "Error: a version conflict during hand-off surfaces as a conflict",
			failWriteAt: 1,
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeConflict)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
				assert.Equal(t, int64(1), stored.Version)
				assert.Empty(t, w.publisher.events)
			},
		},
		{
			name:        "Error: a version conflict during leave surfaces as a conflict",
			failWriteAt: 2,
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeConflict)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, olderGM, campaigntest.OwnerID(*stored), "the hand-off committed before the leave conflicted")
				assert.Equal(t, int64(2), stored.Version)
				_, stillMember := stored.MemberOf(ownerID)
				assert.True(t, stillMember)
			},
		},
		{
			name:       "Error: a non-plan-limit reservation failure aborts the settlement without writing",
			reserveErr: errInjected,
			build: func(t *testing.T) ([]campaign.Campaign, map[string]int, map[string]int, string) {
				c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(olderGM, at(1), false))
				return []campaign.Campaign{c}, map[string]int{ownerID: 1}, nil, ownerID
			},
			check: func(t *testing.T, w *settleWorld, cs []campaign.Campaign, err error) {
				requireAppErr(t, err, apperr.CodeInternal)
				require.ErrorIs(t, err, errInjected)
				stored, _ := w.repo.FetchByID(context.Background(), cs[0].ID)
				assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
				assert.Equal(t, int64(1), stored.Version)
				assert.Equal(t, 1, w.ownership.counts[ownerID])
				assert.Zero(t, w.ownership.counts[olderGM])
				assert.Empty(t, w.unlinker.unlinked)
				assert.Empty(t, w.publisher.events)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			campaigns, owned, limits, user := tc.build(t)
			w := newSettleWorld(campaigns, owned, limits)
			w.repo.failWriteAt = tc.failWriteAt
			w.ownership.reserveErr = tc.reserveErr
			w.unlinker.fail = tc.failUnlink
			w.ownership.failRelease = tc.failRelease
			err := w.settler(t).SettleAccount(context.Background(), user)
			tc.check(t, w, campaigns, err)
		})
	}
}

func TestAccountSettler_LeaveWithTheOwnerStillPresent(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	successor := campaigntest.NewID(t)
	c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(successor, at(1), false))
	w := newSettleWorld([]campaign.Campaign{c}, map[string]int{ownerID: 1}, nil)

	settler, err := campaign.NewAccountSettler(staleListRepo{w.repo}, passTx{}, w.ownership, w.unlinker, w.publisher, campaigntest.Now)
	require.NoError(t, err)
	err = settler.SettleAccount(context.Background(), ownerID)

	requireAppErr(t, err, apperr.CodeInternal)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, campaign.MsgHandOffIncomplete, appErr.Message)
	stored, _ := w.repo.FetchByID(context.Background(), c.ID)
	assert.Equal(t, ownerID, campaigntest.OwnerID(*stored))
	assert.Equal(t, int64(1), stored.Version)
	assert.Empty(t, w.unlinker.unlinked)
}

type staleListRepo struct {
	*memRepo
}

func (r staleListRepo) ListActiveByMember(ctx context.Context, userID string) ([]campaign.Campaign, error) {
	found, err := r.memRepo.ListActiveByMember(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range found {
		for j := range found[i].Members {
			if found[i].Members[j].UserID == userID {
				found[i].Members[j].Owner = false
			}
		}
	}
	return found, nil
}

func TestAccountSettler_CrashBetweenStepsConverges(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	successor := campaigntest.NewID(t)
	other := campaigntest.NewID(t)

	c := campaignWith(t, gmMember(ownerID, at(0), true), gmMember(successor, at(1), false), playerMember(other, at(2)))
	w := newSettleWorld([]campaign.Campaign{c}, map[string]int{ownerID: 1}, nil)
	w.unlinker.fail = true

	err := w.settler(t).SettleAccount(context.Background(), ownerID)
	requireAppErr(t, err, apperr.CodeInternal)

	w.unlinker.fail = false
	require.NoError(t, w.settler(t).SettleAccount(context.Background(), ownerID))
	require.NoError(t, w.settler(t).SettleAccount(context.Background(), ownerID), "a third run is a no-op")

	stored, _ := w.repo.FetchByID(context.Background(), c.ID)
	assert.Equal(t, successor, campaigntest.OwnerID(*stored))
	assert.Equal(t, 1, w.ownership.counts[successor])
	assert.Equal(t, 0, w.ownership.counts[ownerID], "the old owner's slot is released exactly once")
	assert.Equal(t, 1, w.ownership.releases)
	_, stillMember := stored.MemberOf(ownerID)
	assert.False(t, stillMember)
}

func TestAccountSettler_SecondRunAfterDeletingIsNoOp(t *testing.T) {
	ownerID := campaigntest.NewID(t)
	c := campaignWith(t, gmMember(ownerID, at(0), true))
	w := newSettleWorld([]campaign.Campaign{c}, map[string]int{ownerID: 1}, nil)

	require.NoError(t, w.settler(t).SettleAccount(context.Background(), ownerID))
	require.NoError(t, w.settler(t).SettleAccount(context.Background(), ownerID))

	stored, _ := w.repo.FetchByID(context.Background(), c.ID)
	assert.Equal(t, campaign.StatusDeleting, stored.Status)
	assert.Equal(t, 1, w.ownership.releases, "the second run must not release the slot again")
}

func TestNewAccountSettler(t *testing.T) {
	w := newSettleWorld(nil, nil, nil)
	tests := []struct {
		name      string
		repo      campaign.Repository
		tx        campaign.TransactionRunner
		ownership campaign.OwnershipPort
		unlinker  campaign.CharacterUnlinker
		wantErr   bool
	}{
		{name: "Happy path: complete dependencies build a settler", repo: w.repo, tx: passTx{}, ownership: w.ownership, unlinker: w.unlinker},
		{name: "Error: a nil repository is refused", tx: passTx{}, ownership: w.ownership, unlinker: w.unlinker, wantErr: true},
		{name: "Error: a nil transaction runner is refused", repo: w.repo, ownership: w.ownership, unlinker: w.unlinker, wantErr: true},
		{name: "Error: a nil ownership port is refused", repo: w.repo, tx: passTx{}, unlinker: w.unlinker, wantErr: true},
		{name: "Error: a nil unlinker is refused", repo: w.repo, tx: passTx{}, ownership: w.ownership, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settler, err := campaign.NewAccountSettler(tc.repo, tc.tx, tc.ownership, tc.unlinker, w.publisher, campaigntest.Now)
			if tc.wantErr {
				requireAppErr(t, err, apperr.CodeInternal)
				assert.Nil(t, settler)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, settler)
		})
	}
}

func TestAccountSettler_NilPublisherDoesNotPanic(t *testing.T) {
	playerID := campaigntest.NewID(t)
	c := campaignWith(t, gmMember(campaigntest.NewID(t), at(0), true), playerMember(playerID, at(1)))
	w := newSettleWorld([]campaign.Campaign{c}, nil, nil)
	settler, err := campaign.NewAccountSettler(w.repo, passTx{}, w.ownership, w.unlinker, nil, nil)
	require.NoError(t, err)

	require.NoError(t, settler.SettleAccount(context.Background(), playerID))
}

func orderedPair(t *testing.T) (string, string) {
	t.Helper()
	a, b := campaigntest.NewID(t), campaigntest.NewID(t)
	if a < b {
		return a, b
	}
	return b, a
}
