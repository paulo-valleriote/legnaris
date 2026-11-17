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
