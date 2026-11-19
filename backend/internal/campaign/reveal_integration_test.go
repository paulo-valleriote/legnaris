package campaign_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

func (f linkMongoFixture) insertCampaignFor(t *testing.T, ownerID string) campaign.Campaign {
	t.Helper()
	c := campaigntest.Build(t, 0, 1)
	c.Members[0].UserID = ownerID
	c.InviteCode = campaigntest.NewID(t)
	require.NoError(t, f.repo.Insert(f.ctx, &c))
	return c
}

func TestReveal_IsPerCampaignAndLetsPlayersReadOnlyThere(t *testing.T) {
	f := newLinkFixture(t)
	ownerID := campaigntest.NewID(t)
	first := f.insertCampaignFor(t, ownerID)
	second := f.insertCampaignFor(t, ownerID)
	sh := f.insertSheet(t, ownerID, plan.KindNPC)

	_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: first.ID, UserID: ownerID, SheetID: sh.ID})
	require.NoError(t, err)
	_, err = f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: second.ID, UserID: ownerID, SheetID: sh.ID})
	require.NoError(t, err)

	revealed, err := f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: first.ID, UserID: ownerID, SheetID: sh.ID, Revealed: true,
	})
	require.NoError(t, err)
	assert.True(t, revealed.Revealed)
	assert.Equal(t, first.Version+2, f.storedVersion(t, first.ID))
	assert.Equal(t, second.Version+1, f.storedVersion(t, second.ID))

	firstLink, err := f.links.Fetch(f.ctx, first.ID, sh.ID)
	require.NoError(t, err)
	assert.True(t, firstLink.Revealed)
	secondLink, err := f.links.Fetch(f.ctx, second.ID, sh.ID)
	require.NoError(t, err)
	assert.False(t, secondLink.Revealed, "reveal belongs to the link, not the sheet")

	firstPlayer := first.Members[1].UserID
	secondPlayer := second.Members[1].UserID
	listed, err := f.service.ListNpcLinks(f.ctx, first.ID, firstPlayer)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, sh.ID, listed[0].SheetID)
	otherListed, err := f.service.ListNpcLinks(f.ctx, second.ID, secondPlayer)
	require.NoError(t, err)
	assert.Empty(t, otherListed)

	read, err := f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: first.ID, ViewerID: firstPlayer, SheetID: sh.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, read.Sheet)
	assert.Equal(t, campaign.SheetETag(sh.Version, sh.LayoutVersion, campaign.RolePlayer), read.ETag)

	notModified, err := f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: first.ID, ViewerID: firstPlayer, SheetID: sh.ID, IfNoneMatch: read.ETag,
	})
	require.NoError(t, err)
	assert.True(t, notModified.NotModified)
	assert.Nil(t, notModified.Sheet)

	_, err = f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: second.ID, ViewerID: secondPlayer, SheetID: sh.ID,
	})
	requireAppErr(t, err, apperr.CodeNotFound)

	again, err := f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: first.ID, UserID: ownerID, SheetID: sh.ID, Revealed: true,
	})
	require.NoError(t, err)
	assert.True(t, again.Revealed)
	assert.Equal(t, first.Version+3, f.storedVersion(t, first.ID), "an idempotent reveal still runs the campaign transaction")
	assert.Len(t, f.pub.ofType(campaign.EventSheetRevealed), 1, "an idempotent reveal emits nothing")

	revelations := f.pub.ofType(campaign.EventSheetRevealed)
	require.Len(t, revelations, 1)
	data, ok := revelations[0].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"sheet_id": sh.ID,
		"kind":     "npc",
		"owner_id": ownerID,
	}, data)
	assert.NotContains(t, data, "name")
	assert.NotContains(t, data, "data")
}

func TestHide_CutsPlayerAccessAtOnce(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	ownerID := campaigntest.OwnerID(base)
	playerID := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	sh := f.insertSheet(t, ownerID, plan.KindNPC)

	_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID})
	require.NoError(t, err)
	_, err = f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID, Revealed: true,
	})
	require.NoError(t, err)

	read, err := f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: base.ID, ViewerID: playerID, SheetID: sh.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, read.Sheet)

	hidden, err := f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID, Revealed: false,
	})
	require.NoError(t, err)
	assert.False(t, hidden.Revealed)

	_, err = f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: base.ID, ViewerID: playerID, SheetID: sh.ID, IfNoneMatch: read.ETag,
	})
	requireAppErr(t, err, apperr.CodeNotFound)

	listed, err := f.service.ListNpcLinks(f.ctx, base.ID, playerID)
	require.NoError(t, err)
	assert.Empty(t, listed)

	gmRead, err := f.service.ReadNpcSheet(f.ctx, campaign.ReadNpcSheetQuery{
		CampaignID: base.ID, ViewerID: ownerID, SheetID: sh.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, gmRead.Sheet, "GMs keep reading hidden links")

	_, err = f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID, Revealed: false,
	})
	require.NoError(t, err)
	assert.Len(t, f.pub.ofType(campaign.EventSheetHidden), 1, "an idempotent hide emits nothing")
	assert.Len(t, f.pub.ofType(campaign.EventSheetRevealed), 1)
}

func TestSetNpcRevealed_RejectsPlayersAndNonMembers(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	ownerID := campaigntest.OwnerID(base)
	playerID := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	stranger := campaigntest.NewID(t)
	sh := f.insertSheet(t, ownerID, plan.KindNPC)

	_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID})
	require.NoError(t, err)
	afterLink := f.storedVersion(t, base.ID)

	_, err = f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: playerID, SheetID: sh.ID, Revealed: true,
	})
	requireAppErr(t, err, apperr.CodeCampaignForbidden)

	_, err = f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: stranger, SheetID: sh.ID, Revealed: true,
	})
	requireAppErr(t, err, apperr.CodeCampaignNotFound)

	assert.Equal(t, afterLink, f.storedVersion(t, base.ID))
	link, err := f.links.Fetch(f.ctx, base.ID, sh.ID)
	require.NoError(t, err)
	assert.False(t, link.Revealed)
	assert.Empty(t, f.pub.ofType(campaign.EventSheetRevealed))
}

func TestSetNpcRevealed_UnlinkedSheetIsNotFound(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	ownerID := campaigntest.OwnerID(base)
	sh := f.insertSheet(t, ownerID, plan.KindNPC)

	_, err := f.service.SetNpcRevealed(f.ctx, campaign.RevealNpcCommand{
		CampaignID: base.ID, UserID: ownerID, SheetID: sh.ID, Revealed: true,
	})
	requireAppErr(t, err, apperr.CodeNotFound)
	assert.Equal(t, base.Version, f.storedVersion(t, base.ID))
}

func TestLinkStore_SetRevealedUpdatesTheLinkAndReportsMissing(t *testing.T) {
	f := newLinkFixture(t)
	campaignID := campaigntest.NewID(t)
	sheetID := campaigntest.NewID(t)
	link := campaign.SheetLink{
		ID:         campaigntest.NewID(t),
		CampaignID: campaignID,
		SheetID:    sheetID,
		OwnerID:    campaigntest.NewID(t),
		LinkedAt:   campaigntest.Now(),
	}
	_, _, err := f.links.Upsert(f.ctx, &link)
	require.NoError(t, err)

	revealed, changed, err := f.links.SetRevealed(f.ctx, campaignID, sheetID, true)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, link.ID, revealed.ID)
	assert.True(t, revealed.Revealed)

	again, changed, err := f.links.SetRevealed(f.ctx, campaignID, sheetID, true)
	require.NoError(t, err)
	assert.False(t, changed, "a repeated reveal reports no change")
	assert.True(t, again.Revealed)

	stored, err := f.links.Fetch(f.ctx, campaignID, sheetID)
	require.NoError(t, err)
	assert.True(t, stored.Revealed)

	hidden, changed, err := f.links.SetRevealed(f.ctx, campaignID, sheetID, false)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, hidden.Revealed)

	_, changed, err = f.links.SetRevealed(f.ctx, campaignID, campaigntest.NewID(t), true)
	require.ErrorIs(t, err, campaign.ErrLinkNotFound)
	assert.False(t, changed)
	assert.Equal(t, int64(1), f.linkCount(t), "a missing link is never inserted")
}

func TestLinkStore_SetRevealedIsOneConditionalUpdate(t *testing.T) {
	f := newLinkFixture(t)
	campaignID := campaigntest.NewID(t)
	sheetID := campaigntest.NewID(t)
	link := campaign.SheetLink{
		ID:         campaigntest.NewID(t),
		CampaignID: campaignID,
		SheetID:    sheetID,
		OwnerID:    campaigntest.NewID(t),
		LinkedAt:   campaigntest.Now(),
	}
	_, _, err := f.links.Upsert(f.ctx, &link)
	require.NoError(t, err)

	const contenders = 8
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		changed  int
		failures []error
	)
	for range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, didChange, err := f.links.SetRevealed(f.ctx, campaignID, sheetID, true)
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				return
			}
			if didChange {
				mu.Lock()
				changed++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	require.Empty(t, failures)
	assert.Equal(t, 1, changed, "only one concurrent reveal may report a change")
	stored, err := f.links.Fetch(f.ctx, campaignID, sheetID)
	require.NoError(t, err)
	assert.True(t, stored.Revealed)
}
