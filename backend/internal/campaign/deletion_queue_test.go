package campaign_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
)

type deletionHarness struct {
	store *mocks.MockDeletionStore
	links *mocks.MockSheetLinks
	npc   *mocks.MockNpcSheetLinks
	tx    *mocks.MockTransactionRunner
	pub   *eventsmocks.MockPublisher
	sent  []events.Event
}

func newDeletionHarness(t *testing.T) *deletionHarness {
	t.Helper()
	h := &deletionHarness{
		store: mocks.NewMockDeletionStore(t),
		links: mocks.NewMockSheetLinks(t),
		npc:   mocks.NewMockNpcSheetLinks(t),
		tx:    mocks.NewMockTransactionRunner(t),
		pub:   eventsmocks.NewMockPublisher(t),
	}
	h.tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	).Maybe()
	h.pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
		h.sent = append(h.sent, ev)
	}).Return().Maybe()
	return h
}

func (h *deletionHarness) deps() campaign.DeletionDeps {
	return campaign.DeletionDeps{
		Store:     h.store,
		Links:     h.links,
		NpcLinks:  h.npc,
		Tx:        h.tx,
		Publisher: h.pub,
		Now:       campaigntest.Now,
	}
}

func deletionRemovalFixture(t *testing.T) campaign.Campaign {
	t.Helper()
	base := campaigntest.Build(t, 0, 2)
	base.Status = campaign.StatusDeleting
	return base
}

func TestDeletionDeps_Handle(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(h *deletionHarness, c campaign.Campaign)
		wantErr     bool
		wantEvent   bool
		wantVersion int64
	}{
		{
			name: "Happy path: linked sheets are unlinked in batches, then the campaign is removed and announced",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(3), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(2), nil).Once()
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(true, nil).Once()
			},
			wantEvent:   true,
			wantVersion: 4,
		},
		{
			name: "Edge: a late link committed before removal is unlinked by a further batch",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(5), nil).Times(3)
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(1), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(1), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(true, nil).Once()
			},
			wantEvent:   true,
			wantVersion: 5,
		},
		{
			name: "Happy path: NPC and Enemy links are deleted in batches before the campaign is removed",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(6), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(1), nil).Once()
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(7), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(2), nil).Once()
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(8), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(true, nil).Once()
			},
			wantEvent:   true,
			wantVersion: 8,
		},
		{
			name: "Edge: a late NPC link committed between batches is deleted before removal",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(3), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(1), nil).Once()
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(1), nil).Once()
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, campaigntest.Now()).Return(int64(5), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(true, nil).Once()
			},
			wantEvent:   true,
			wantVersion: 5,
		},
		{
			name: "Edge: a campaign that is no longer deleting is already settled and emits nothing",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(0), campaign.ErrNotDeleting).Once()
			},
		},
		{
			name: "Edge: a concurrent removal that already won emits nothing",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(false, nil).Once()
			},
		},
		{
			name: "Error: a failed version touch aborts the batch without removing",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(0), errors.New("boom")).Once()
			},
			wantErr: true,
		},
		{
			name: "Error: a failed unlink leaves the campaign for a retry",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), errors.New("boom")).Once()
			},
			wantErr: true,
		},
		{
			name: "Error: a failed NPC link deletion leaves the campaign for a retry",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), errors.New("boom")).Once()
			},
			wantErr: true,
		},
		{
			name: "Error: a failed removal is not announced",
			setup: func(h *deletionHarness, c campaign.Campaign) {
				h.store.EXPECT().TouchDeleting(mock.Anything, c.ID, mock.Anything).Return(int64(4), nil).Once()
				h.links.EXPECT().UnlinkCampaignBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.npc.EXPECT().DeleteByCampaignIDBatch(mock.Anything, c.ID, campaign.DeletionUnlinkCampaignBatch).Return(int64(0), nil).Once()
				h.store.EXPECT().RemoveDeleting(mock.Anything, c.ID).Return(false, errors.New("boom")).Once()
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newDeletionHarness(t)
			c := deletionRemovalFixture(t)
			tc.setup(h, c)

			err := h.deps().Handle(context.Background(), c)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tc.wantEvent {
				assert.Empty(t, h.sent, "campaign.deleted is only announced after a removal")
				return
			}
			require.Len(t, h.sent, 1)
			assert.Equal(t, campaign.EventDeleted, h.sent[0].Type)
			assert.Equal(t, c.ID, h.sent[0].AggregateID)
			assert.Equal(t, tc.wantVersion, h.sent[0].AggregateVersion)
			assert.Nil(t, h.sent[0].ActorID, "the sweeper acts without a user")
		})
	}
}

func TestDeletionDeps_HandleStopsOnCancelledContext(t *testing.T) {
	h := newDeletionHarness(t)
	c := deletionRemovalFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := h.deps().Handle(ctx, c)

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, h.sent)
}

func TestNewDeletionQueue(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(d *campaign.DeletionDeps)
		attemptCap int
		batch      int
		wantErr    bool
	}{
		{name: "Happy path: a complete deletion queue is built", attemptCap: 5, batch: 25},
		{name: "Edge: a nil publisher falls back to a no-op", mutate: func(d *campaign.DeletionDeps) { d.Publisher = nil }, attemptCap: 5, batch: 25},
		{name: "Edge: a nil clock falls back to the wall clock", mutate: func(d *campaign.DeletionDeps) { d.Now = nil }, attemptCap: 5, batch: 25},
		{name: "Error: a missing store is refused", mutate: func(d *campaign.DeletionDeps) { d.Store = nil }, attemptCap: 5, batch: 25, wantErr: true},
		{name: "Error: missing sheet links are refused", mutate: func(d *campaign.DeletionDeps) { d.Links = nil }, attemptCap: 5, batch: 25, wantErr: true},
		{name: "Error: missing NPC links are refused", mutate: func(d *campaign.DeletionDeps) { d.NpcLinks = nil }, attemptCap: 5, batch: 25, wantErr: true},
		{name: "Error: a missing transaction runner is refused", mutate: func(d *campaign.DeletionDeps) { d.Tx = nil }, attemptCap: 5, batch: 25, wantErr: true},
		{name: "Error: a zero batch is refused by the sweeper", attemptCap: 5, batch: 0, wantErr: true},
		{name: "Error: a zero attempt cap is refused by the sweeper", attemptCap: 0, batch: 25, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newDeletionHarness(t)
			deps := h.deps()
			if tc.mutate != nil {
				tc.mutate(&deps)
			}

			queue, err := campaign.NewDeletionQueue(deps, tc.attemptCap, tc.batch)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, campaign.DeletionQueueName, queue.Name)
			assert.Equal(t, 5*time.Minute, queue.Interval)
		})
	}
}
