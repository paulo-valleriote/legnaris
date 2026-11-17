package campaign_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/mocks"
)

func TestNewOrphanLinksQueue(t *testing.T) {
	tests := []struct {
		name    string
		links   campaign.OrphanLinks
		batch   int
		wantErr bool
	}{
		{name: "Happy path: the reconciliation queue is built with a daily interval", links: mocks.NewMockOrphanLinks(t), batch: 25},
		{name: "Error: nil links are refused", links: nil, batch: 25, wantErr: true},
		{name: "Error: a zero batch is refused by the sweeper", links: mocks.NewMockOrphanLinks(t), batch: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			queue, err := campaign.NewOrphanLinksQueue(tc.links, tc.batch)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, campaign.OrphanQueueName, queue.Name)
			assert.Equal(t, campaign.OrphanInterval, queue.Interval)
		})
	}
}

func TestReconcileOrphans(t *testing.T) {
	tests := []struct {
		name    string
		passes  []int64
		errAt   int
		wantErr bool
		calls   int
	}{
		{name: "Happy path: passes repeat until a pass finds no orphans", passes: []int64{3, 2, 0}, calls: 3},
		{name: "Edge: nothing to clear finishes after one pass", passes: []int64{0}, calls: 1},
		{name: "Error: a failing pass is reported so the sweeper retries", passes: []int64{2}, errAt: 1, wantErr: true, calls: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := mocks.NewMockOrphanLinks(t)
			call := 0
			links.EXPECT().UnlinkOrphans(mock.Anything, 25).RunAndReturn(
				func(context.Context, int) (int64, error) {
					call++
					if tc.errAt == call {
						return 0, errors.New("boom")
					}
					return tc.passes[call-1], nil
				},
			).Times(tc.calls)

			err := campaign.ReconcileOrphans(context.Background(), links, 25)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestReconcileOrphans_StopsOnCancelledContext(t *testing.T) {
	links := mocks.NewMockOrphanLinks(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, campaign.ReconcileOrphans(ctx, links, 25), context.Canceled)
}
