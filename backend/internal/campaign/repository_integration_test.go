package campaign_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/db/dbtest"
)

type mongoFixture struct {
	repo       campaign.Repository
	tx         *db.Transactor
	collection *mongo.Collection
	ctx        context.Context
}

func newMongoFixture(t *testing.T) mongoFixture {
	t.Helper()
	ctx := context.Background()
	client := dbtest.Client(t)
	mdb := dbtest.Adopt(t, client, dbtest.NewDatabaseName(t))
	require.NoError(t, db.EnsureIndexes(ctx, mdb))
	return mongoFixture{
		repo:       campaign.NewRepository(mdb),
		tx:         db.NewTransactor(client, nil),
		collection: mdb.Collection("campaigns"),
		ctx:        ctx,
	}
}

func (f mongoFixture) insert(t *testing.T, c campaign.Campaign) {
	t.Helper()
	require.NoError(t, f.repo.Insert(f.ctx, &c))
}

func (f mongoFixture) storedVersion(t *testing.T, id string) int64 {
	t.Helper()
	var doc struct {
		Version int64 `bson:"version"`
	}
	require.NoError(t, f.collection.FindOne(f.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc))
	return doc.Version
}

func TestRepository_FetchByIDMissingIsNotFound(t *testing.T) {
	f := newMongoFixture(t)
	tests := []struct {
		name string
		id   string
	}{
		{"Edge: unknown id is not found", campaigntest.NewID(t)},
		{"Edge: empty id is not found", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.repo.FetchByID(f.ctx, tc.id)
			assert.True(t, errors.Is(err, campaign.ErrNotFound))
		})
	}
}

func TestRepository_SaveRejectsStaleVersion(t *testing.T) {
	f := newMongoFixture(t)
	base := campaigntest.Build(t, 0, 0)
	f.insert(t, base)
	tests := []struct {
		name     string
		expected int64
		wantErr  error
	}{
		{"Happy path: the current version saves", 1, nil},
		{"Edge: a stale expected version is a conflict", 0, campaign.ErrVersionConflict},
		{"Edge: a future expected version is a conflict", 99, campaign.ErrVersionConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			current, err := f.repo.FetchByID(f.ctx, base.ID)
			require.NoError(t, err)
			current.UpdatedAt = current.UpdatedAt.Add(1)
			err = f.repo.Save(f.ctx, current, tc.expected)
			if tc.wantErr != nil {
				assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRepository_ListActiveByMemberExcludesDeletingAndStrangers(t *testing.T) {
	f := newMongoFixture(t)
	active := campaigntest.Build(t, 0, 1)
	deleting := campaigntest.Build(t, 0, 1)
	deleting.Status = campaign.StatusDeleting
	f.insert(t, active)
	f.insert(t, deleting)
	owner := campaigntest.OwnerID(active)

	tests := []struct {
		name string
		user string
		want int
	}{
		{"Happy path: the owner sees the active campaign", owner, 1},
		{"Edge: an unknown user sees nothing", campaigntest.NewID(t), 0},
		{"Edge: an empty user id sees nothing", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found, err := f.repo.ListActiveByMember(f.ctx, tc.user)
			require.NoError(t, err)
			assert.Len(t, found, tc.want)
		})
	}
}

func TestMutate_EveryTransactionIncrementsVersionByExactlyOne(t *testing.T) {
	f := newMongoFixture(t)
	base := campaigntest.Build(t, 0, 0)
	f.insert(t, base)
	svc := campaign.NewService(f.repo, f.tx, nil, campaign.WithClock(campaigntest.Now))

	ownerID := campaigntest.OwnerID(base)
	joined := 0
	for range 3 {
		_, err := campaign.JoinPlayer(f.ctx, f.tx, f.repo, base.ID, campaigntest.NewID(t), campaigntest.Now())
		require.NoError(t, err)
		joined++
	}
	require.Equal(t, int64(1+joined), f.storedVersion(t, base.ID))

	_, err := svc.Promote(f.ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: ownerID, TargetID: ownerID})
	requireAppErr(t, err, apperr.CodeCampaignForbidden)
	assert.Equal(t, int64(1+joined), f.storedVersion(t, base.ID), "a rejected write must not bump the version")
}

func TestMutate_DeletingCampaignRefusesWrites(t *testing.T) {
	f := newMongoFixture(t)
	base := campaigntest.Build(t, 0, 0)
	f.insert(t, base)
	_, err := f.collection.UpdateOne(f.ctx, bson.D{{Key: "_id", Value: base.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: campaign.StatusDeleting}}}})
	require.NoError(t, err)

	_, err = campaign.JoinPlayer(f.ctx, f.tx, f.repo, base.ID, campaigntest.NewID(t), campaigntest.Now())
	requireAppErr(t, err, apperr.CodeCampaignNotFound)
}

func TestJoinPlayer_ConcurrentJoinsNeverExceedPlayerCap(t *testing.T) {
	f := newMongoFixture(t)
	base := campaigntest.Build(t, 0, 0)
	f.insert(t, base)

	const contenders = campaign.MaxPlayers + 15
	var (
		wg        sync.WaitGroup
		start     = make(chan struct{})
		mu        sync.Mutex
		succeeded int
		full      int
		failures  []error
	)
	for range contenders {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			<-start
			_, err := campaign.JoinPlayer(f.ctx, f.tx, f.repo, base.ID, userID, campaigntest.Now())
			mu.Lock()
			defer mu.Unlock()
			var appErr *apperr.Error
			switch {
			case err == nil:
				succeeded++
			case errors.As(err, &appErr) && appErr.Code == apperr.CodeCampaignFull:
				full++
			default:
				failures = append(failures, err)
			}
		}(campaigntest.NewID(t))
	}
	close(start)
	wg.Wait()

	require.Empty(t, failures, "unexpected errors: %v", failures)
	assert.Equal(t, campaign.MaxPlayers, succeeded)
	assert.Equal(t, contenders-campaign.MaxPlayers, full)

	stored, err := f.repo.FetchByID(f.ctx, base.ID)
	require.NoError(t, err)
	assert.Equal(t, campaign.MaxPlayers, stored.PlayerCount)
	assert.Equal(t, int64(1+campaign.MaxPlayers), stored.Version, "version must advance once per successful join")
}

func TestPromote_ConcurrentPromotionsNeverExceedGMCap(t *testing.T) {
	f := newMongoFixture(t)
	base := campaigntest.Build(t, 0, 8)
	f.insert(t, base)
	svc := campaign.NewService(f.repo, f.tx, nil, campaign.WithClock(campaigntest.Now))
	owner := campaigntest.OwnerID(base)
	players := campaigntest.MembersWithRole(base, campaign.RolePlayer)

	var (
		wg        sync.WaitGroup
		start     = make(chan struct{})
		mu        sync.Mutex
		succeeded int
		full      int
		failures  []error
	)
	for _, target := range players {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			<-start
			_, err := svc.Promote(f.ctx, campaign.RoleCommand{CampaignID: base.ID, ActorID: owner, TargetID: target})
			mu.Lock()
			defer mu.Unlock()
			var appErr *apperr.Error
			switch {
			case err == nil:
				succeeded++
			case errors.As(err, &appErr) && appErr.Code == apperr.CodeCampaignFull:
				full++
			default:
				failures = append(failures, err)
			}
		}(target)
	}
	close(start)
	wg.Wait()

	require.Empty(t, failures, "unexpected errors: %v", failures)
	assert.Equal(t, campaign.MaxGMs-1, succeeded)
	assert.Equal(t, len(players)-(campaign.MaxGMs-1), full)

	stored, err := f.repo.FetchByID(f.ctx, base.ID)
	require.NoError(t, err)
	assert.Equal(t, campaign.MaxGMs, stored.GMCount)
}
