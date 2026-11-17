package campaign_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
)

type deletionFixture struct {
	ctx       context.Context
	db        *mongo.Database
	repo      campaign.Repository
	store     *campaign.MongoDeletionStore
	links     campaign.SheetLinks
	npcLinks  *campaign.MongoLinkStore
	tx        *db.Transactor
	campaigns *mongo.Collection
	sheets    *mongo.Collection
	npc       *mongo.Collection
}

func newDeletionFixture(t *testing.T) deletionFixture {
	t.Helper()
	ctx := context.Background()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(ctx, mdb))
	return deletionFixture{
		ctx:       ctx,
		db:        mdb,
		repo:      campaign.NewRepository(mdb),
		store:     campaign.NewDeletionStore(mdb),
		links:     sheet.NewRepository(mdb, nil),
		npcLinks:  campaign.NewLinkStore(mdb),
		tx:        db.NewTransactor(client, nil),
		campaigns: mdb.Collection("campaigns"),
		sheets:    mdb.Collection("sheets"),
		npc:       mdb.Collection("campaign_sheet_links"),
	}
}

func (f deletionFixture) insertDeleting(t *testing.T, at time.Time) campaign.Campaign {
	t.Helper()
	c := campaigntest.Build(t, 0, 1)
	c.Status = campaign.StatusDeleting
	c.DeletingAt = &at
	require.NoError(t, f.repo.Insert(f.ctx, &c))
	return c
}

func (f deletionFixture) insertSheet(t *testing.T, campaignID string) string {
	t.Helper()
	id := campaigntest.NewID(t)
	_, err := f.sheets.InsertOne(f.ctx, bson.D{{Key: "_id", Value: id}, {Key: "campaign_id", Value: campaignID}})
	require.NoError(t, err)
	return id
}

func (f deletionFixture) insertLink(t *testing.T, campaignID, sheetID, ownerID string) string {
	t.Helper()
	id := campaigntest.NewID(t)
	_, err := f.npc.InsertOne(f.ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "campaign_id", Value: campaignID},
		{Key: "sheet_id", Value: sheetID},
		{Key: "owner_id", Value: ownerID},
		{Key: "revealed", Value: false},
		{Key: "linked_at", Value: campaigntest.Now()},
	})
	require.NoError(t, err)
	return id
}

func (f deletionFixture) linkCount(t *testing.T) int64 {
	t.Helper()
	n, err := f.npc.CountDocuments(f.ctx, bson.D{})
	require.NoError(t, err)
	return n
}

func (f deletionFixture) storedCampaign(t *testing.T, id string) bson.M {
	t.Helper()
	var doc bson.M
	require.NoError(t, f.campaigns.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
	return doc
}

func TestDeletionStore_ClaimHonorsLeaseAndState(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	deleting := f.insertDeleting(t, now.Add(-time.Minute))
	active := campaigntest.Build(t, 0, 0)
	require.NoError(t, f.repo.Insert(f.ctx, &active))

	first, err := f.store.ClaimDeleting(f.ctx, now, time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, first, 1, "only deleting campaigns are claimed; active ones never are")
	assert.Equal(t, deleting.ID, first[0].ID)

	tests := []struct {
		name    string
		at      time.Time
		wantLen int
	}{
		{name: "Edge: a claimed lease still held is not claimed again", at: now.Add(30 * time.Second), wantLen: 0},
		{name: "Edge: the lease expiring makes the campaign claimable again", at: now.Add(2 * time.Minute), wantLen: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.store.ClaimDeleting(f.ctx, tc.at, time.Minute, 10)
			require.NoError(t, err)
			assert.Len(t, got, tc.wantLen)
		})
	}
}

func TestDeletionStore_RescheduleIncrementsAttemptsAndDefersClaim(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	c := f.insertDeleting(t, now)

	require.NoError(t, f.store.RescheduleDeleting(f.ctx, c.ID, now.Add(time.Hour), "boom"))
	require.NoError(t, f.store.RescheduleDeleting(f.ctx, c.ID, now.Add(time.Hour), "boom again"))

	assert.EqualValues(t, 2, f.storedCampaign(t, c.ID)["deletion_attempts"], "each failed attempt must increment the counter")

	notYet, err := f.store.ClaimDeleting(f.ctx, now, time.Minute, 10)
	require.NoError(t, err)
	assert.Empty(t, notYet, "a rescheduled campaign waits for its next attempt time")

	due, err := f.store.ClaimDeleting(f.ctx, now.Add(2*time.Hour), time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, 2, due[0].DeletionAttempts, "the claim must carry the stored attempt count to the sweeper cap")
}

func TestDeletionStore_RescheduleAndDeadRejectSettledState(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	active := campaigntest.Build(t, 0, 0)
	require.NoError(t, f.repo.Insert(f.ctx, &active))
	missing := campaigntest.NewID(t)

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "Edge: rescheduling an active campaign loses the claim", run: func() error {
			return f.store.RescheduleDeleting(f.ctx, active.ID, now, "x")
		}},
		{name: "Edge: rescheduling an unknown campaign loses the claim", run: func() error {
			return f.store.RescheduleDeleting(f.ctx, missing, now, "x")
		}},
		{name: "Edge: marking an unknown campaign dead loses the claim", run: func() error {
			return f.store.MarkDeletionDead(f.ctx, missing, "x")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.run(), campaign.ErrDeletionClaimLost)
		})
	}
}

func TestDeletionStore_DeadCampaignsAreNotClaimed(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	c := f.insertDeleting(t, now)

	require.NoError(t, f.store.MarkDeletionDead(f.ctx, c.ID, "gave up"))

	stored := f.storedCampaign(t, c.ID)
	assert.Equal(t, string(campaign.StatusDeletionDead), stored["status"])
	claimed, err := f.store.ClaimDeleting(f.ctx, now.Add(24*time.Hour), time.Minute, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
	_, pending, err := f.store.OldestDeleting(f.ctx)
	require.NoError(t, err)
	assert.False(t, pending, "a dead deletion is not pending in the oldest-age gauge")
}

func TestDeletionStore_OldestDeletingReportsEarliestRequest(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()

	_, pending, err := f.store.OldestDeleting(f.ctx)
	require.NoError(t, err)
	require.False(t, pending, "an empty collection has nothing pending")

	f.insertDeleting(t, now.Add(-2*time.Hour))
	oldest := f.insertDeleting(t, now.Add(-3*time.Hour))
	f.insertDeleting(t, now)

	at, pending, err := f.store.OldestDeleting(f.ctx)
	require.NoError(t, err)
	require.True(t, pending)
	assert.True(t, at.Equal(*oldest.DeletingAt))
}

func TestDeletionStore_TouchIncrementsVersionByExactlyOne(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	c := f.insertDeleting(t, now)
	active := campaigntest.Build(t, 0, 0)
	require.NoError(t, f.repo.Insert(f.ctx, &active))

	tests := []struct {
		name        string
		id          string
		wantErr     error
		wantVersion int64
	}{
		{name: "Happy path: a deleting campaign gets one version bump per touch", id: c.ID, wantVersion: c.Version + 1},
		{name: "Edge: a second touch bumps once more, never twice", id: c.ID, wantVersion: c.Version + 2},
		{name: "Edge: an active campaign is not touched by the deletion batch", id: active.ID, wantErr: campaign.ErrNotDeleting},
		{name: "Edge: an unknown campaign is not touched", id: campaigntest.NewID(t), wantErr: campaign.ErrNotDeleting},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version, err := f.store.TouchDeleting(f.ctx, tc.id, now)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantVersion, version)
			assert.EqualValues(t, tc.wantVersion, f.storedCampaign(t, tc.id)["version"])
		})
	}
}

func TestDeletionStore_RemoveDeletingOnlyOnce(t *testing.T) {
	f := newDeletionFixture(t)
	c := f.insertDeleting(t, campaigntest.Now())

	removed, err := f.store.RemoveDeleting(f.ctx, c.ID)
	require.NoError(t, err)
	assert.True(t, removed)

	again, err := f.store.RemoveDeleting(f.ctx, c.ID)
	require.NoError(t, err)
	assert.False(t, again, "a repeated removal is a no-op, so retries are idempotent")
}

func TestSheetLinks_UnlinkCampaignBatchOnlyTouchesTheCampaign(t *testing.T) {
	f := newDeletionFixture(t)
	target := campaigntest.NewID(t)
	other := campaigntest.NewID(t)
	f.insertSheet(t, target)
	f.insertSheet(t, target)
	foreign := f.insertSheet(t, other)

	tests := []struct {
		name  string
		limit int
		want  int64
	}{
		{name: "Edge: a zero limit unlinks nothing", limit: 0, want: 0},
		{name: "Edge: the batch limit caps one pass", limit: 1, want: 1},
		{name: "Happy path: the remaining linked sheet is unlinked", limit: 10, want: 1},
		{name: "Edge: nothing left to unlink", limit: 10, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := f.links.UnlinkCampaignBatch(f.ctx, target, tc.limit)
			require.NoError(t, err)
			assert.Equal(t, tc.want, n)
		})
	}

	var foreignDoc bson.M
	require.NoError(t, f.sheets.FindOne(f.ctx, bson.D{{Key: "_id", Value: foreign}}).Decode(&foreignDoc))
	assert.Equal(t, other, foreignDoc["campaign_id"], "a sheet linked to another campaign must not be unlinked")
}

type recordingPublisher struct {
	events []events.Event
}

func (p *recordingPublisher) Publish(ev events.Event) {
	p.events = append(p.events, ev)
}

func TestDeletionDeps_EndToEndRemovesCampaignAndUnlinksSheets(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	c := f.insertDeleting(t, now)
	sheetA := f.insertSheet(t, c.ID)
	sheetB := f.insertSheet(t, c.ID)
	npcSheet := campaigntest.NewID(t)
	f.insertLink(t, c.ID, npcSheet, campaigntest.NewID(t))
	foreignCampaign := campaigntest.NewID(t)
	foreignLink := f.insertLink(t, foreignCampaign, campaigntest.NewID(t), campaigntest.NewID(t))
	pub := &recordingPublisher{}

	deps := campaign.DeletionDeps{
		Store:     f.store,
		Links:     f.links,
		NpcLinks:  f.npcLinks,
		Tx:        f.tx,
		Publisher: pub,
		Now:       campaigntest.Now,
	}
	require.NoError(t, deps.Handle(f.ctx, c))

	count, err := f.campaigns.CountDocuments(f.ctx, bson.D{{Key: "_id", Value: c.ID}})
	require.NoError(t, err)
	assert.Zero(t, count, "the campaign is removed once no sheet and no link points at it")
	for _, id := range []string{sheetA, sheetB} {
		var doc bson.M
		require.NoError(t, f.sheets.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
		assert.Nil(t, doc["campaign_id"], "every linked sheet is unlinked")
	}
	assert.Equal(t, int64(1), f.linkCount(t), "only the link of another campaign survives")
	var surviving bson.M
	require.NoError(t, f.npc.FindOne(f.ctx, bson.D{{Key: "_id", Value: foreignLink}}).Decode(&surviving))
	assert.Equal(t, foreignCampaign, surviving["campaign_id"])
	require.Len(t, pub.events, 1)
	assert.Equal(t, campaign.EventDeleted, pub.events[0].Type)

	require.NoError(t, deps.Handle(f.ctx, c), "a redelivered handle after removal is a no-op")
	assert.Len(t, pub.events, 1, "campaign.deleted is announced once")
	assert.Equal(t, int64(1), f.linkCount(t), "a repeated deletion removes nothing more")
}

func TestDeletionDeps_DeletingLinksEmitsNoUnlinkEvent(t *testing.T) {
	f := newDeletionFixture(t)
	c := f.insertDeleting(t, campaigntest.Now())
	f.insertSheet(t, c.ID)
	f.insertLink(t, c.ID, campaigntest.NewID(t), campaigntest.NewID(t))
	pub := &recordingPublisher{}
	deps := campaign.DeletionDeps{
		Store:     f.store,
		Links:     f.links,
		NpcLinks:  f.npcLinks,
		Tx:        f.tx,
		Publisher: pub,
		Now:       campaigntest.Now,
	}

	require.NoError(t, deps.Handle(f.ctx, c))

	require.Len(t, pub.events, 1)
	assert.Equal(t, campaign.EventDeleted, pub.events[0].Type)
	for _, ev := range pub.events {
		assert.NotEqual(t, campaign.EventSheetUnlinked, ev.Type, "campaign deletion reports only campaign.deleted")
	}
}

type betweenBatchesTx struct {
	inner      campaign.TransactionRunner
	t          *testing.T
	fixture    deletionFixture
	campaignID string
	committed  int
	late       string
}

// The late sheet is written after the first batch commits, so the next batch snapshot sees it.
// A link written inside a running batch is rejected by the campaign version write conflict,
// because a link goes through campaign.Mutate and bumps the Campaign.
func (r *betweenBatchesTx) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := r.inner.WithTransaction(ctx, fn); err != nil {
		return err
	}
	r.committed++
	if r.committed == 1 {
		r.late = r.fixture.insertSheet(r.t, r.campaignID)
	}
	return nil
}

func TestDeletionDeps_LateLinkIsUndoneBeforeRemoval(t *testing.T) {
	f := newDeletionFixture(t)
	c := f.insertDeleting(t, campaigntest.Now())
	first := f.insertSheet(t, c.ID)
	tx := &betweenBatchesTx{inner: f.tx, t: t, fixture: f, campaignID: c.ID}
	pub := &recordingPublisher{}
	deps := campaign.DeletionDeps{Store: f.store, Links: f.links, NpcLinks: f.npcLinks, Tx: tx, Publisher: pub, Now: campaigntest.Now}

	require.NoError(t, deps.Handle(f.ctx, c))

	require.NotEmpty(t, tx.late, "the late link must have been written between batches")
	for _, id := range []string{first, tx.late} {
		var doc bson.M
		require.NoError(t, f.sheets.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
		assert.Nil(t, doc["campaign_id"], "every link written while deleting is undone before the campaign is removed")
	}
	count, err := f.campaigns.CountDocuments(f.ctx, bson.D{{Key: "_id", Value: c.ID}})
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.Len(t, pub.events, 1)
}

type lateNpcLinkTx struct {
	inner      campaign.TransactionRunner
	t          *testing.T
	fixture    deletionFixture
	campaignID string
	committed  int
	late       string
}

func (r *lateNpcLinkTx) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := r.inner.WithTransaction(ctx, fn); err != nil {
		return err
	}
	r.committed++
	if r.committed == 1 {
		r.late = r.fixture.insertLink(r.t, r.campaignID, campaigntest.NewID(r.t), campaigntest.NewID(r.t))
	}
	return nil
}

func TestDeletionDeps_LateNpcLinkIsDeletedBeforeRemoval(t *testing.T) {
	f := newDeletionFixture(t)
	c := f.insertDeleting(t, campaigntest.Now())
	f.insertLink(t, c.ID, campaigntest.NewID(t), campaigntest.NewID(t))
	tx := &lateNpcLinkTx{inner: f.tx, t: t, fixture: f, campaignID: c.ID}
	pub := &recordingPublisher{}
	deps := campaign.DeletionDeps{Store: f.store, Links: f.links, NpcLinks: f.npcLinks, Tx: tx, Publisher: pub, Now: campaigntest.Now}

	require.NoError(t, deps.Handle(f.ctx, c))

	require.NotEmpty(t, tx.late, "the late NPC link must have been written between batches")
	assert.Zero(t, f.linkCount(t), "every NPC link written while deleting is deleted before the campaign is removed")
	count, err := f.campaigns.CountDocuments(f.ctx, bson.D{{Key: "_id", Value: c.ID}})
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.Len(t, pub.events, 1)
}

func TestRepository_MarkDeletingIncrementsVersionByExactlyOne(t *testing.T) {
	f := newDeletionFixture(t)
	now := campaigntest.Now()
	base := campaigntest.Build(t, 0, 1)
	require.NoError(t, f.repo.Insert(f.ctx, &base))

	tests := []struct {
		name     string
		expected int64
		wantErr  error
	}{
		{name: "Edge: a stale expected version is a conflict and writes nothing", expected: base.Version + 1, wantErr: campaign.ErrVersionConflict},
		{name: "Happy path: the current version moves to deleting with one bump", expected: base.Version},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := campaigntest.Copy(base)
			c.Status = campaign.StatusDeleting
			c.DeletingAt = &now
			c.Version = tc.expected + 1
			err := f.repo.MarkDeleting(f.ctx, &c, tc.expected)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.EqualValues(t, base.Version, f.storedCampaign(t, base.ID)["version"])
				assert.Equal(t, string(campaign.StatusActive), f.storedCampaign(t, base.ID)["status"])
				return
			}
			require.NoError(t, err)
			stored := f.storedCampaign(t, base.ID)
			assert.Equal(t, string(campaign.StatusDeleting), stored["status"])
			assert.EqualValues(t, base.Version+1, stored["version"])
			assert.EqualValues(t, 0, stored["deletion_attempts"])
		})
	}
}

func TestOrphanLinks_ClearsOnlyLinksToMissingCampaigns(t *testing.T) {
	f := newDeletionFixture(t)
	valid := campaigntest.Build(t, 0, 0)
	require.NoError(t, f.repo.Insert(f.ctx, &valid))
	missing := campaigntest.NewID(t)
	orphan := f.insertSheet(t, missing)
	kept := f.insertSheet(t, valid.ID)
	links := campaign.NewOrphanLinks(f.db)

	n, err := links.UnlinkOrphans(f.ctx, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	var orphanDoc bson.M
	require.NoError(t, f.sheets.FindOne(f.ctx, bson.D{{Key: "_id", Value: orphan}}).Decode(&orphanDoc))
	assert.Nil(t, orphanDoc["campaign_id"], "a link to a missing Campaign is cleared")
	var keptDoc bson.M
	require.NoError(t, f.sheets.FindOne(f.ctx, bson.D{{Key: "_id", Value: kept}}).Decode(&keptDoc))
	assert.Equal(t, valid.ID, keptDoc["campaign_id"], "a link to a live Campaign is kept")

	again, err := links.UnlinkOrphans(f.ctx, 10)
	require.NoError(t, err)
	assert.Zero(t, again, "a second pass finds nothing, so reconciliation is idempotent")
}

func TestOrphanLinks_DeletesLinksToMissingCampaignsOrSheets(t *testing.T) {
	f := newDeletionFixture(t)
	live := campaigntest.Build(t, 0, 0)
	require.NoError(t, f.repo.Insert(f.ctx, &live))
	liveSheet := campaigntest.NewID(t)
	_, err := f.sheets.InsertOne(f.ctx, bson.D{{Key: "_id", Value: liveSheet}, {Key: "owner_id", Value: campaigntest.NewID(t)}})
	require.NoError(t, err)
	valid := f.insertLink(t, live.ID, liveSheet, campaigntest.NewID(t))
	missingCampaign := f.insertLink(t, campaigntest.NewID(t), liveSheet, campaigntest.NewID(t))
	missingSheet := f.insertLink(t, live.ID, campaigntest.NewID(t), campaigntest.NewID(t))
	links := campaign.NewOrphanLinks(f.db)

	n, err := links.UnlinkOrphans(f.ctx, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)

	for _, id := range []string{missingCampaign, missingSheet} {
		count, err := f.npc.CountDocuments(f.ctx, bson.D{{Key: "_id", Value: id}})
		require.NoError(t, err)
		assert.Zero(t, count, "a link to a missing campaign or sheet is deleted")
	}
	count, err := f.npc.CountDocuments(f.ctx, bson.D{{Key: "_id", Value: valid}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "a link whose campaign and sheet exist is kept")

	again, err := links.UnlinkOrphans(f.ctx, 10)
	require.NoError(t, err)
	assert.Zero(t, again, "reconciliation is idempotent")
}
