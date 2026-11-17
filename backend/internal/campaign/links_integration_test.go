package campaign_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

type npcRecordingPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (p *npcRecordingPublisher) Publish(ev events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
}

func (p *npcRecordingPublisher) ofType(eventType string) []events.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	var found []events.Event
	for _, ev := range p.events {
		if ev.Type == eventType {
			found = append(found, ev)
		}
	}
	return found
}

type linkMongoFixture struct {
	ctx     context.Context
	mdb     *mongo.Database
	repo    campaign.Repository
	links   *campaign.MongoLinkStore
	sheets  sheet.Repository
	tx      *db.Transactor
	service campaign.Service
	pub     *npcRecordingPublisher
}

func newLinkFixture(t *testing.T) linkMongoFixture {
	t.Helper()
	ctx := context.Background()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(ctx, mdb))
	repo := campaign.NewRepository(mdb)
	links := campaign.NewLinkStore(mdb)
	sheets := sheet.NewRepository(mdb, nil)
	tx := db.NewTransactor(client, nil)
	pub := &npcRecordingPublisher{}
	return linkMongoFixture{
		ctx:     ctx,
		mdb:     mdb,
		repo:    repo,
		links:   links,
		sheets:  sheets,
		tx:      tx,
		service: campaign.NewService(repo, tx, pub, campaign.WithCharacterStore(sheets), campaign.WithLinkStore(links), campaign.WithClock(campaigntest.Now)),
		pub:     pub,
	}
}

func (f linkMongoFixture) insertCampaign(t *testing.T) campaign.Campaign {
	t.Helper()
	c := campaigntest.Build(t, 0, 1)
	require.NoError(t, f.repo.Insert(f.ctx, &c))
	return c
}

func (f linkMongoFixture) insertSheet(t *testing.T, owner string, kind plan.SheetKind) sheet.Sheet {
	t.Helper()
	sh := sheet.Sheet{
		ID:         campaigntest.NewID(t),
		TemplateID: "dnd5e",
		OwnerID:    owner,
		Kind:       kind,
		Name:       "Brom",
		Data:       map[string]any{"stats": map[string]any{"hp": 42.0}},
		CreatedAt:  campaigntest.Now(),
		UpdatedAt:  campaigntest.Now(),
	}
	_, err := f.mdb.Collection("sheets").InsertOne(f.ctx, sh)
	require.NoError(t, err)
	return sh
}

func (f linkMongoFixture) linkCount(t *testing.T) int64 {
	t.Helper()
	count, err := f.mdb.Collection("campaign_sheet_links").CountDocuments(f.ctx, bson.D{})
	require.NoError(t, err)
	return count
}

func (f linkMongoFixture) storedVersion(t *testing.T, campaignID string) int64 {
	t.Helper()
	var doc struct {
		Version int64 `bson:"version"`
	}
	require.NoError(t, f.mdb.Collection("campaigns").FindOne(f.ctx, bson.D{{Key: "_id", Value: campaignID}}).Decode(&doc))
	return doc.Version
}

func TestLinkNpc_ConcurrentLinksOfTheSamePairCreateOneLink(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	owner := campaigntest.OwnerID(base)
	sh := f.insertSheet(t, owner, plan.KindNPC)

	const contenders = 8
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		ids      = make(map[string]struct{})
		failures []error
	)
	for range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			view, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			ids[view.ID] = struct{}{}
		}()
	}
	close(start)
	wg.Wait()

	require.Empty(t, failures)
	require.Len(t, ids, 1, "concurrent links of one pair must resolve to one link document")
	assert.Equal(t, int64(1), f.linkCount(t))
	assert.Equal(t, base.Version+contenders, f.storedVersion(t, base.ID))
	assert.Len(t, f.pub.ofType(campaign.EventSheetLinked), 1, "only the winning link emits an event")
}

func TestLinkNpc_OneSheetLinksToManyCampaigns(t *testing.T) {
	f := newLinkFixture(t)
	first := f.insertCampaign(t)
	second := f.insertCampaign(t)
	owner := campaigntest.OwnerID(first)
	second.Members[0].UserID = owner
	_, err := f.mdb.Collection("campaigns").UpdateOne(f.ctx, bson.D{{Key: "_id", Value: second.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "members", Value: second.Members}}}})
	require.NoError(t, err)
	sh := f.insertSheet(t, owner, plan.KindMonster)

	_, err = f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: first.ID, UserID: owner, SheetID: sh.ID})
	require.NoError(t, err)
	_, err = f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: second.ID, UserID: owner, SheetID: sh.ID})
	require.NoError(t, err)

	assert.Equal(t, int64(2), f.linkCount(t))
}

func TestUnlinkNpc_RemovesTheLinkAndIsIdempotent(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	owner := campaigntest.OwnerID(base)
	sh := f.insertSheet(t, owner, plan.KindNPC)

	_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), f.linkCount(t))

	require.NoError(t, f.service.UnlinkNpc(f.ctx, campaign.UnlinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID}))
	assert.Equal(t, int64(0), f.linkCount(t))
	unlinked := f.pub.ofType(campaign.EventSheetUnlinked)
	require.Len(t, unlinked, 1)
	unlinkedData, ok := unlinked[0].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, campaign.ReasonUnlinked, unlinkedData["reason"])

	require.NoError(t, f.service.UnlinkNpc(f.ctx, campaign.UnlinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID}))
	assert.Len(t, f.pub.ofType(campaign.EventSheetUnlinked), 1, "an already removed link emits nothing")
}

func TestLinkStore_UpsertIsIdempotentUnderTheUniqueIndex(t *testing.T) {
	f := newLinkFixture(t)
	campaignID := campaigntest.NewID(t)
	sheetID := campaigntest.NewID(t)
	first := campaign.SheetLink{
		ID:         campaigntest.NewID(t),
		CampaignID: campaignID,
		SheetID:    sheetID,
		OwnerID:    campaigntest.NewID(t),
		LinkedAt:   campaigntest.Now(),
	}
	stored, created, err := f.links.Upsert(f.ctx, &first)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, first.ID, stored.ID)

	second := first
	second.ID = campaigntest.NewID(t)
	second.Revealed = true
	stored, created, err = f.links.Upsert(f.ctx, &second)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, stored.ID, "the existing link wins")
	assert.False(t, stored.Revealed, "a re-link never changes reveal state")

	count, err := f.mdb.Collection("campaign_sheet_links").CountDocuments(f.ctx, bson.D{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestLinkStore_FetchDeleteAndListBySheet(t *testing.T) {
	f := newLinkFixture(t)
	owner := campaigntest.NewID(t)
	sheetID := campaigntest.NewID(t)
	firstCampaign := campaigntest.NewID(t)
	secondCampaign := campaigntest.NewID(t)
	first := campaign.SheetLink{ID: campaigntest.NewID(t), CampaignID: firstCampaign, SheetID: sheetID, OwnerID: owner, LinkedAt: campaigntest.Now()}
	second := campaign.SheetLink{ID: campaigntest.NewID(t), CampaignID: secondCampaign, SheetID: sheetID, OwnerID: owner, LinkedAt: campaigntest.Now().Add(time.Minute)}
	_, _, err := f.links.Upsert(f.ctx, &first)
	require.NoError(t, err)
	_, _, err = f.links.Upsert(f.ctx, &second)
	require.NoError(t, err)

	found, err := f.links.Fetch(f.ctx, firstCampaign, sheetID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, found.ID)

	_, err = f.links.Fetch(f.ctx, campaigntest.NewID(t), sheetID)
	assert.True(t, errors.Is(err, campaign.ErrLinkNotFound))

	listed, err := f.links.ListByCampaignID(f.ctx, firstCampaign)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	removed, err := f.links.DeleteBySheetID(f.ctx, sheetID)
	require.NoError(t, err)
	assert.Len(t, removed, 2)
	assert.Equal(t, int64(0), f.linkCount(t))

	deleted, err := f.links.Delete(f.ctx, firstCampaign, sheetID)
	require.NoError(t, err)
	assert.False(t, deleted, "deleting a missing link reports nothing removed")
}

func TestLinkStore_ConcurrentUpsertsCreateOneDocument(t *testing.T) {
	f := newLinkFixture(t)
	link := campaign.SheetLink{
		CampaignID: campaigntest.NewID(t),
		SheetID:    campaigntest.NewID(t),
		OwnerID:    campaigntest.NewID(t),
		LinkedAt:   campaigntest.Now(),
	}
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		created  int
		failures []error
	)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			candidate := link
			id, err := idgen.New()
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				return
			}
			candidate.ID = id
			_, inserted, err := f.links.Upsert(f.ctx, &candidate)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, err)
				return
			}
			if inserted {
				created++
			}
		}()
	}
	close(start)
	wg.Wait()

	require.Empty(t, failures)
	assert.Equal(t, 1, created)
	assert.Equal(t, int64(1), f.linkCount(t))
}

func TestLinkStore_DuplicateKeyInsideATransactionIsRetried(t *testing.T) {
	f := newLinkFixture(t)
	taken := campaign.SheetLink{
		ID:         campaigntest.NewID(t),
		CampaignID: campaigntest.NewID(t),
		SheetID:    campaigntest.NewID(t),
		OwnerID:    campaigntest.NewID(t),
		LinkedAt:   campaigntest.Now(),
	}
	_, _, err := f.links.Upsert(f.ctx, &taken)
	require.NoError(t, err)

	attempts := 0
	var stored campaign.SheetLink
	err = f.tx.WithTransaction(f.ctx, func(txCtx context.Context) error {
		attempts++
		candidate := campaign.SheetLink{
			ID:         taken.ID,
			CampaignID: campaigntest.NewID(t),
			SheetID:    campaigntest.NewID(t),
			OwnerID:    campaigntest.NewID(t),
			LinkedAt:   campaigntest.Now(),
		}
		if attempts > 1 {
			candidate.ID = campaigntest.NewID(t)
		}
		var upsertErr error
		stored, _, upsertErr = f.links.Upsert(txCtx, &candidate)
		return upsertErr
	})
	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "a duplicate key inside a transaction surfaces as transient and is retried")
	assert.NotEqual(t, taken.ID, stored.ID)
	assert.Equal(t, int64(2), f.linkCount(t))
}

type freePlanSource struct{}

func (freePlanSource) PlanOf(context.Context, string) (plan.Plan, error) {
	return plan.Free, nil
}

func TestSheetDeletion_RemovesLinksAndReleasesTheSlotInOneTransaction(t *testing.T) {
	f := newLinkFixture(t)
	base := f.insertCampaign(t)
	owner := campaigntest.OwnerID(base)
	sh := f.insertSheet(t, owner, plan.KindNPC)

	_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: sh.ID})
	require.NoError(t, err)
	_, err = f.mdb.Collection("sheet_counts").InsertOne(f.ctx, bson.D{
		{Key: "owner_id", Value: owner},
		{Key: "kind", Value: string(plan.KindNPC)},
		{Key: "count", Value: 1},
	})
	require.NoError(t, err)

	sheets := sheet.NewService(f.sheets, nil, nil, logger.NewLogger(slog.LevelError), f.pub,
		sheet.WithPlanLimits(freePlanSource{}, f.tx),
		sheet.WithLinkCleaner(campaign.NewSheetLinkCleaner(f.links, f.pub)),
	)
	require.NoError(t, sheets.Delete(f.ctx, sheet.DeleteSheetCommand{SheetID: sh.ID, OwnerID: owner}))

	assert.Equal(t, int64(0), f.linkCount(t))
	var counts struct {
		Count int `bson:"count"`
	}
	require.NoError(t, f.mdb.Collection("sheet_counts").FindOne(f.ctx, bson.D{
		{Key: "owner_id", Value: owner},
		{Key: "kind", Value: string(plan.KindNPC)},
	}).Decode(&counts))
	assert.Equal(t, 0, counts.Count)

	unlinked := f.pub.ofType(campaign.EventSheetUnlinked)
	require.Len(t, unlinked, 1)
	unlinkedData, ok := unlinked[0].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, campaign.ReasonSheetDeleted, unlinkedData["reason"])
	assert.Equal(t, sh.ID, unlinkedData["sheet_id"])
	assert.NotContains(t, unlinkedData, "name")
	assert.NotContains(t, unlinkedData, "data")
}

func (f linkMongoFixture) insertLink(t *testing.T, campaignID, sheetID, ownerID string) campaign.SheetLink {
	t.Helper()
	link := campaign.SheetLink{
		ID:         campaigntest.NewID(t),
		CampaignID: campaignID,
		SheetID:    sheetID,
		OwnerID:    ownerID,
		LinkedAt:   campaigntest.Now(),
	}
	stored, _, err := f.links.Upsert(f.ctx, &link)
	require.NoError(t, err)
	return stored
}

func TestLinkStore_DeleteByOwnerOnlyTouchesThatOwnerInThatCampaign(t *testing.T) {
	f := newLinkFixture(t)
	campaignA := campaigntest.NewID(t)
	campaignB := campaigntest.NewID(t)
	firstOwner := campaigntest.NewID(t)
	secondOwner := campaigntest.NewID(t)
	first := f.insertLink(t, campaignA, campaigntest.NewID(t), firstOwner)
	second := f.insertLink(t, campaignA, campaigntest.NewID(t), firstOwner)
	otherOwner := f.insertLink(t, campaignA, campaigntest.NewID(t), secondOwner)
	otherCampaign := f.insertLink(t, campaignB, campaigntest.NewID(t), firstOwner)

	removed, err := f.links.DeleteByOwner(f.ctx, campaignA, firstOwner)
	require.NoError(t, err)
	require.Len(t, removed, 2)
	removedIDs := make([]string, 0, len(removed))
	for _, link := range removed {
		removedIDs = append(removedIDs, link.ID)
	}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, removedIDs)
	assert.Equal(t, int64(2), f.linkCount(t), "the other owner's link and the other campaign's link stay")

	for _, id := range []string{otherOwner.ID, otherCampaign.ID} {
		count, err := f.mdb.Collection("campaign_sheet_links").CountDocuments(f.ctx, bson.D{{Key: "_id", Value: id}})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	}

	again, err := f.links.DeleteByOwner(f.ctx, campaignA, firstOwner)
	require.NoError(t, err)
	assert.Empty(t, again, "a repeated delete is a no-op")
}

func TestLinkStore_DeleteByCampaignIDBatchIsBoundedAndIdempotent(t *testing.T) {
	f := newLinkFixture(t)
	target := campaigntest.NewID(t)
	other := campaigntest.NewID(t)
	f.insertLink(t, target, campaigntest.NewID(t), campaigntest.NewID(t))
	f.insertLink(t, target, campaigntest.NewID(t), campaigntest.NewID(t))
	f.insertLink(t, target, campaigntest.NewID(t), campaigntest.NewID(t))
	kept := f.insertLink(t, other, campaigntest.NewID(t), campaigntest.NewID(t))

	tests := []struct {
		name  string
		limit int
		want  int64
	}{
		{name: "Edge: a zero limit deletes nothing", limit: 0, want: 0},
		{name: "Happy path: the batch limit caps one pass", limit: 1, want: 1},
		{name: "Happy path: the remaining links of the campaign are deleted", limit: 10, want: 2},
		{name: "Edge: nothing left to delete", limit: 10, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := f.links.DeleteByCampaignIDBatch(f.ctx, target, tc.limit)
			require.NoError(t, err)
			assert.Equal(t, tc.want, n)
		})
	}

	count, err := f.mdb.Collection("campaign_sheet_links").CountDocuments(f.ctx, bson.D{{Key: "_id", Value: kept.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "a link of another campaign is never deleted")
}

func TestGmExit_DeletesOnlyThatGMsLinks(t *testing.T) {
	tests := []struct {
		name string
		run  func(ctx context.Context, service campaign.Service, campaignID, ownerID, gmID string) error
	}{
		{
			name: "Happy path: a GM who leaves loses only their links in this campaign",
			run: func(ctx context.Context, service campaign.Service, campaignID, _, gmID string) error {
				return service.Leave(ctx, campaign.LeaveCommand{CampaignID: campaignID, UserID: gmID})
			},
		},
		{
			name: "Happy path: a demoted GM loses only their links in this campaign",
			run: func(ctx context.Context, service campaign.Service, campaignID, ownerID, gmID string) error {
				_, err := service.Demote(ctx, campaign.RoleCommand{CampaignID: campaignID, ActorID: ownerID, TargetID: gmID})
				return err
			},
		},
		{
			name: "Happy path: a removed GM loses only their links in this campaign",
			run: func(ctx context.Context, service campaign.Service, campaignID, ownerID, gmID string) error {
				_, err := service.Remove(ctx, campaign.RoleCommand{CampaignID: campaignID, ActorID: ownerID, TargetID: gmID})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newLinkFixture(t)
			base := campaigntest.Build(t, 1, 0)
			require.NoError(t, f.repo.Insert(f.ctx, &base))
			owner := campaigntest.OwnerID(base)
			coGM := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
			sh := f.insertSheet(t, coGM, plan.KindMonster)
			otherCampaign := campaigntest.Build(t, 0, 0)
			otherCampaign.Members[0].UserID = coGM
			require.NoError(t, f.repo.Insert(f.ctx, &otherCampaign))
			ownerSheet := f.insertSheet(t, owner, plan.KindNPC)

			_, err := f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: coGM, SheetID: sh.ID})
			require.NoError(t, err)
			_, err = f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: base.ID, UserID: owner, SheetID: ownerSheet.ID})
			require.NoError(t, err)
			_, err = f.service.LinkNpc(f.ctx, campaign.LinkNpcCommand{CampaignID: otherCampaign.ID, UserID: coGM, SheetID: sh.ID})
			require.NoError(t, err)
			require.Equal(t, int64(3), f.linkCount(t))

			require.NoError(t, tc.run(f.ctx, f.service, base.ID, owner, coGM))

			assert.Equal(t, int64(2), f.linkCount(t), "only the exiting GM's link in this campaign is deleted")
			remaining, err := f.links.ListByCampaignID(f.ctx, base.ID)
			require.NoError(t, err)
			require.Len(t, remaining, 1)
			assert.Equal(t, owner, remaining[0].OwnerID)
			other, err := f.links.ListByCampaignID(f.ctx, otherCampaign.ID)
			require.NoError(t, err)
			assert.Len(t, other, 1, "the same GM's link in another campaign stays")

			unlinked := f.pub.ofType(campaign.EventSheetUnlinked)
			require.Len(t, unlinked, 1)
			unlinkedData, ok := unlinked[0].Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, campaign.ReasonOwnerLeft, unlinkedData["reason"])
			assert.Equal(t, sh.ID, unlinkedData["sheet_id"])
			assert.NotContains(t, unlinkedData, "name")
			assert.NotContains(t, unlinkedData, "data")
		})
	}
}
