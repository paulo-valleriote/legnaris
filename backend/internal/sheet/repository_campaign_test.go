package sheet_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	campaignLinkA   = "01900000-0000-7000-8000-0000000000c1"
	campaignLinkB   = "01900000-0000-7000-8000-0000000000c2"
	campaignSheetS1 = "01900000-0000-7000-8000-0000000000d1"
	campaignSheetS2 = "01900000-0000-7000-8000-0000000000d2"
	campaignSheetS3 = "01900000-0000-7000-8000-0000000000d3"
	campaignOwnerP1 = "owner-link-1"
	campaignOwnerP2 = "owner-link-2"
)

func seedCampaignSheet(t *testing.T, repo sheet.Repository, id, owner string, linkedTo *string) {
	t.Helper()
	require.NoError(t, repo.Create(context.Background(), &sheet.Sheet{
		ID:         id,
		TemplateID: "dnd5e",
		OwnerID:    owner,
		Kind:       plan.KindCharacter,
		Name:       "Character " + id,
		CampaignID: linkedTo,
	}))
}

func strRef(s string) *string {
	return &s
}

func TestRepository_LinkCampaignCompareAndSet(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		seedLink *string
		expected *string
		want     bool
	}{
		{name: "Happy path: free sheet links when expecting unlinked", seedLink: nil, expected: nil, want: true},
		{name: "Edge: already linked sheet refuses an unlinked expectation", seedLink: strRef(campaignLinkA), expected: nil, want: false},
		{name: "Edge: stale expectation of another campaign refuses", seedLink: strRef(campaignLinkA), expected: strRef(campaignLinkB), want: false},
		{name: "Happy path: dangling value is replaced when expected exactly", seedLink: strRef(campaignLinkB), expected: strRef(campaignLinkB), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepositoryTest(t)
			seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, tc.seedLink)

			got, err := repo.LinkCampaign(ctx, campaignSheetS1, tc.expected, campaignLinkA)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			stored, err := repo.FetchByID(ctx, campaignSheetS1)
			require.NoError(t, err)
			if tc.want {
				require.NotNil(t, stored.CampaignID)
				assert.Equal(t, campaignLinkA, *stored.CampaignID)
			}
		})
	}
}

func TestRepository_LinkCampaignMissingSheetIsNotMatched(t *testing.T) {
	repo := newRepositoryTest(t)
	got, err := repo.LinkCampaign(context.Background(), "01900000-0000-7000-8000-0000000000ff", nil, campaignLinkA)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRepository_ConcurrentLinksNeverLinkOneSheetTwice(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()
	seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, nil)

	const racers = 16
	var wins atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		target := campaignLinkA
		if i%2 == 1 {
			target = campaignLinkB
		}
		go func(campaignID string) {
			defer wg.Done()
			<-start
			ok, err := repo.LinkCampaign(ctx, campaignSheetS1, nil, campaignID)
			if err == nil && ok {
				wins.Add(1)
			}
		}(target)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int64(1), wins.Load(), "exactly one racer may link the sheet")
}

func TestRepository_UnlinkOwnerCampaignOnlyTouchesOwnerInCampaign(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()
	seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, strRef(campaignLinkA))
	seedCampaignSheet(t, repo, campaignSheetS2, campaignOwnerP2, strRef(campaignLinkA))
	seedCampaignSheet(t, repo, campaignSheetS3, campaignOwnerP1, strRef(campaignLinkB))

	tests := []struct {
		name      string
		owner     string
		campaign  string
		wantCount int64
	}{
		{name: "Edge: owner with no sheet in the campaign unlinks nothing", owner: "nobody", campaign: campaignLinkA, wantCount: 0},
		{name: "Edge: unknown campaign unlinks nothing", owner: campaignOwnerP1, campaign: "01900000-0000-7000-8000-0000000000ff", wantCount: 0},
		{name: "Happy path: owner's sheet in the campaign is unlinked", owner: campaignOwnerP1, campaign: campaignLinkA, wantCount: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.UnlinkOwnerCampaign(ctx, tc.owner, tc.campaign)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, got)
		})
	}

	s1, err := repo.FetchByID(ctx, campaignSheetS1)
	require.NoError(t, err)
	assert.Nil(t, s1.CampaignID, "the owner's campaign sheet is unlinked")

	s2, err := repo.FetchByID(ctx, campaignSheetS2)
	require.NoError(t, err)
	require.NotNil(t, s2.CampaignID, "another owner's sheet in the same campaign stays linked")

	s3, err := repo.FetchByID(ctx, campaignSheetS3)
	require.NoError(t, err)
	require.NotNil(t, s3.CampaignID, "the owner's sheet in another campaign stays linked")
}

func TestRepository_UnlinkCampaignBatchDrainsInBoundedSteps(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()
	seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, strRef(campaignLinkA))
	seedCampaignSheet(t, repo, campaignSheetS2, campaignOwnerP2, strRef(campaignLinkA))
	seedCampaignSheet(t, repo, campaignSheetS3, campaignOwnerP1, strRef(campaignLinkB))

	tests := []struct {
		name      string
		campaign  string
		limit     int
		wantCount int64
	}{
		{name: "Edge: zero limit is a no-op", campaign: campaignLinkA, limit: 0, wantCount: 0},
		{name: "Edge: negative limit is a no-op", campaign: campaignLinkA, limit: -1, wantCount: 0},
		{name: "Edge: unknown campaign is a no-op", campaign: "01900000-0000-7000-8000-0000000000ff", limit: 10, wantCount: 0},
		{name: "Happy path: a bounded batch unlinks at most the limit", campaign: campaignLinkA, limit: 1, wantCount: 1},
		{name: "Happy path: a later batch drains the rest of the campaign", campaign: campaignLinkA, limit: 10, wantCount: 1},
		{name: "Edge: drained campaign has nothing left", campaign: campaignLinkA, limit: 10, wantCount: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.UnlinkCampaignBatch(ctx, tc.campaign, tc.limit)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, got)
		})
	}

	s3, err := repo.FetchByID(ctx, campaignSheetS3)
	require.NoError(t, err)
	require.NotNil(t, s3.CampaignID, "other campaigns are untouched by a batch")
}

func TestRepository_ListByCampaignIDReturnsOnlyLinkedSheets(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()
	seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, strRef(campaignLinkA))
	seedCampaignSheet(t, repo, campaignSheetS2, campaignOwnerP2, nil)
	seedCampaignSheet(t, repo, campaignSheetS3, campaignOwnerP2, strRef(campaignLinkB))

	tests := []struct {
		name     string
		campaign string
		wantIDs  []string
	}{
		{name: "Edge: unknown campaign lists nothing", campaign: "01900000-0000-7000-8000-0000000000ff", wantIDs: nil},
		{name: "Happy path: campaign lists only its linked sheets", campaign: campaignLinkA, wantIDs: []string{campaignSheetS1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found, err := repo.ListByCampaignID(ctx, tc.campaign)
			require.NoError(t, err)
			ids := make([]string, 0, len(found))
			for i := range found {
				ids = append(ids, found[i].ID)
			}
			assert.Equal(t, tc.wantIDs, ids)
		})
	}
}

func TestRepository_UpdateLayoutBumpsLayoutVersionOnly(t *testing.T) {
	repo := newRepositoryTest(t)
	ctx := context.Background()
	seedCampaignSheet(t, repo, campaignSheetS1, campaignOwnerP1, nil)

	before, err := repo.FetchByID(ctx, campaignSheetS1)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateLayout(ctx, campaignSheetS1, &sheet.Layout{SectionOrder: []string{"stats"}}))
	require.NoError(t, repo.UpdateLayout(ctx, campaignSheetS1, nil))

	after, err := repo.FetchByID(ctx, campaignSheetS1)
	require.NoError(t, err)
	assert.Equal(t, before.Version, after.Version, "a layout change must not bump the sheet version")
	assert.Equal(t, int64(2), after.LayoutVersion, "each layout change bumps layout_version by one")
}
