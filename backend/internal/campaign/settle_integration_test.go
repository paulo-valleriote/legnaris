package campaign_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
)

type settleIntegrationFixture struct {
	ctx       context.Context
	db        *mongo.Database
	repo      *campaign.MongoRepository
	counter   *campaign.MongoOwnershipCounter
	limiter   *campaign.OwnershipLimiter
	sheets    sheet.Repository
	tx        *db.Transactor
	users     *mongo.Collection
	campaigns *mongo.Collection
	sheetDocs *mongo.Collection
}

func newSettleIntegrationFixture(t *testing.T) settleIntegrationFixture {
	t.Helper()
	ctx := context.Background()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(ctx, mdb))
	counter := campaign.NewOwnershipCounter(mdb)
	return settleIntegrationFixture{
		ctx:       ctx,
		db:        mdb,
		repo:      campaign.NewRepository(mdb),
		counter:   counter,
		limiter:   campaign.NewOwnershipLimiter(user.NewPlanSource(user.NewRepository(mdb, nil)), counter),
		sheets:    sheet.NewRepository(mdb, nil),
		tx:        db.NewTransactor(client, nil),
		users:     mdb.Collection("users"),
		campaigns: mdb.Collection("campaigns"),
		sheetDocs: mdb.Collection("sheets"),
	}
}

func (f settleIntegrationFixture) settler(t *testing.T, repo campaign.Repository) *campaign.AccountSettler {
	t.Helper()
	settler, err := campaign.NewAccountSettler(repo, f.tx, f.limiter, settleUnlinker{sheets: f.sheets}, events.NoOp{}, campaigntest.Now)
	require.NoError(t, err)
	return settler
}

func (f settleIntegrationFixture) insertUser(t *testing.T, id, planID string) {
	t.Helper()
	_, err := f.users.InsertOne(f.ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "email", Value: id + "@example.com"},
		{Key: "username", Value: id},
		{Key: "plan", Value: planID},
	})
	require.NoError(t, err)
}

func (f settleIntegrationFixture) insertSheet(t *testing.T, ownerID, campaignID string) string {
	t.Helper()
	id := campaigntest.NewID(t)
	_, err := f.sheetDocs.InsertOne(f.ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "owner_id", Value: ownerID},
		{Key: "campaign_id", Value: campaignID},
	})
	require.NoError(t, err)
	return id
}

func (f settleIntegrationFixture) seedHandOff(t *testing.T) (campaign.Campaign, string, string, string) {
	t.Helper()
	c := campaigntest.Build(t, 1, 0)
	owner := campaigntest.OwnerID(c)
	successor := campaigntest.MembersWithRole(c, campaign.RoleGM)[0]
	f.insertUser(t, owner, "paid")
	f.insertUser(t, successor, "paid")
	require.NoError(t, f.repo.Insert(f.ctx, &c))
	require.NoError(t, f.counter.Reserve(f.ctx, owner, 5))
	return c, owner, successor, f.insertSheet(t, owner, c.ID)
}

func (f settleIntegrationFixture) seedOwnerAlone(t *testing.T) (campaign.Campaign, string) {
	t.Helper()
	c := campaigntest.Build(t, 0, 0)
	owner := campaigntest.OwnerID(c)
	f.insertUser(t, owner, "paid")
	require.NoError(t, f.repo.Insert(f.ctx, &c))
	require.NoError(t, f.counter.Reserve(f.ctx, owner, 5))
	require.NoError(t, f.counter.Reserve(f.ctx, owner, 5))
	return c, owner
}

func (f settleIntegrationFixture) storedCampaign(t *testing.T, id string) bson.M {
	t.Helper()
	var doc bson.M
	require.NoError(t, f.campaigns.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
	return doc
}

func (f settleIntegrationFixture) storedSheet(t *testing.T, id string) bson.M {
	t.Helper()
	var doc bson.M
	require.NoError(t, f.sheetDocs.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
	return doc
}

func (f settleIntegrationFixture) ownerCount(t *testing.T, userID string) int {
	t.Helper()
	count, err := f.counter.Count(f.ctx, userID)
	require.NoError(t, err)
	return count
}

type settleUnlinker struct {
	sheets sheet.Repository
}

func (u settleUnlinker) UnlinkCharacter(ctx context.Context, campaignID, userID string) error {
	_, err := u.sheets.UnlinkOwnerCampaign(ctx, userID, campaignID)
	return err
}

type staleFetchRepository struct {
	campaign.Repository
}

func (r staleFetchRepository) FetchByID(ctx context.Context, id string) (*campaign.Campaign, error) {
	c, err := r.Repository.FetchByID(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Version = 0
	return c, nil
}

func TestAccountSettler_Integration(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Happy path: hand-off commits the owner change, both counter moves and the sheet unlink",
			run: func(t *testing.T) {
				f := newSettleIntegrationFixture(t)
				c, owner, successor, sheetID := f.seedHandOff(t)

				require.NoError(t, f.settler(t, f.repo).SettleAccount(f.ctx, owner))

				stored := f.storedCampaign(t, c.ID)
				assert.Equal(t, string(campaign.StatusActive), stored["status"])
				assert.EqualValues(t, c.Version+2, stored["version"])
				loaded, err := f.repo.FetchByID(f.ctx, c.ID)
				require.NoError(t, err)
				assert.Equal(t, successor, campaigntest.OwnerID(*loaded))
				_, stillMember := loaded.MemberOf(owner)
				assert.False(t, stillMember)
				assert.Zero(t, f.ownerCount(t, owner))
				assert.Equal(t, 1, f.ownerCount(t, successor))
				assert.NotContains(t, f.storedSheet(t, sheetID), "campaign_id")
			},
		},
		{
			name: "Error: a version conflict during hand-off rolls back the successor reservation",
			run: func(t *testing.T) {
				f := newSettleIntegrationFixture(t)
				c, owner, successor, sheetID := f.seedHandOff(t)

				err := f.settler(t, staleFetchRepository{Repository: f.repo}).SettleAccount(f.ctx, owner)

				requireAppErr(t, err, apperr.CodeConflict)
				stored := f.storedCampaign(t, c.ID)
				assert.EqualValues(t, c.Version, stored["version"])
				loaded, fetchErr := f.repo.FetchByID(f.ctx, c.ID)
				require.NoError(t, fetchErr)
				assert.Equal(t, owner, campaigntest.OwnerID(*loaded))
				assert.Equal(t, 1, f.ownerCount(t, owner))
				assert.Zero(t, f.ownerCount(t, successor))
				assert.Contains(t, f.storedSheet(t, sheetID), "campaign_id")
			},
		},
		{
			name: "Edge: the deletion path releases the owner slot exactly once across two runs",
			run: func(t *testing.T) {
				f := newSettleIntegrationFixture(t)
				c, owner := f.seedOwnerAlone(t)
				settler := f.settler(t, f.repo)

				require.NoError(t, settler.SettleAccount(f.ctx, owner))
				require.NoError(t, settler.SettleAccount(f.ctx, owner))

				stored := f.storedCampaign(t, c.ID)
				assert.Equal(t, string(campaign.StatusDeleting), stored["status"])
				assert.EqualValues(t, c.Version+1, stored["version"])
				assert.Equal(t, 1, f.ownerCount(t, owner))
			},
		},
		{
			name: "Edge: a second SettleAccount run converges on the same state",
			run: func(t *testing.T) {
				f := newSettleIntegrationFixture(t)
				c, owner, successor, sheetID := f.seedHandOff(t)
				settler := f.settler(t, f.repo)

				require.NoError(t, settler.SettleAccount(f.ctx, owner))
				require.NoError(t, settler.SettleAccount(f.ctx, owner))

				stored := f.storedCampaign(t, c.ID)
				assert.EqualValues(t, c.Version+2, stored["version"])
				loaded, err := f.repo.FetchByID(f.ctx, c.ID)
				require.NoError(t, err)
				assert.Equal(t, successor, campaigntest.OwnerID(*loaded))
				assert.Zero(t, f.ownerCount(t, owner))
				assert.Equal(t, 1, f.ownerCount(t, successor))
				assert.NotContains(t, f.storedSheet(t, sheetID), "campaign_id")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}
