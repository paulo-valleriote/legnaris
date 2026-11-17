package campaign_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	"legnaris/backend/internal/campaign/campaigntest"
	"legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/shared/apperr"
)

func (h *harness) expectMarkDeleting(expectedVersion int64, result error) {
	h.repo.EXPECT().MarkDeleting(mock.Anything, mock.Anything, expectedVersion).RunAndReturn(
		func(_ context.Context, c *campaign.Campaign, _ int64) error {
			h.saved = c
			return result
		},
	).Once()
}

func deletingCopy(base campaign.Campaign) campaign.Campaign {
	out := campaigntest.Copy(base)
	out.Status = campaign.StatusDeleting
	return out
}

func deadCopy(base campaign.Campaign) campaign.Campaign {
	out := campaigntest.Copy(base)
	out.Status = campaign.StatusDeletionDead
	return out
}

func TestDelete(t *testing.T) {
	base := campaigntest.Build(t, 1, 1)
	releaser := mocks.NewMockOwnershipReleaser(t)
	owner := campaigntest.OwnerID(base)
	gm := campaigntest.MembersWithRole(base, campaign.RoleGM)[0]
	player := campaigntest.MembersWithRole(base, campaign.RolePlayer)[0]
	outsider := campaigntest.NewID(t)

	tests := []struct {
		name       string
		campaignID string
		actorID    string
		stored     *campaign.Campaign
		loadErr    error
		markErr    error
		wantCode   apperr.Code
		wantMark   bool
	}{
		{
			name:       "Happy path: the owner moves an active campaign to deleting",
			campaignID: base.ID,
			actorID:    owner,
			stored:     &base,
			wantMark:   true,
		},
		{
			name:       "Edge: empty actor is unauthorized without touching storage",
			campaignID: base.ID,
			actorID:    "",
			wantCode:   apperr.CodeUnauthorized,
		},
		{
			name:       "Edge: malformed campaign id is an invalid request",
			campaignID: "not-a-uuid",
			actorID:    owner,
			wantCode:   apperr.CodeInvalidRequest,
		},
		{
			name:       "Edge: empty campaign id is an invalid request",
			campaignID: "",
			actorID:    owner,
			wantCode:   apperr.CodeInvalidRequest,
		},
		{
			name:       "Edge: unknown campaign is not found",
			campaignID: base.ID,
			actorID:    owner,
			loadErr:    campaign.ErrNotFound,
			wantCode:   apperr.CodeCampaignNotFound,
		},
		{
			name:       "Edge: non-member gets not found, not forbidden",
			campaignID: base.ID,
			actorID:    outsider,
			stored:     &base,
			wantCode:   apperr.CodeCampaignNotFound,
		},
		{
			name:       "Edge: a campaign already deleting is not found for every caller",
			campaignID: base.ID,
			actorID:    owner,
			stored:     ptr(deletingCopy(base)),
			wantCode:   apperr.CodeCampaignNotFound,
		},
		{
			name:       "Edge: a dead deletion is not found for every caller",
			campaignID: base.ID,
			actorID:    owner,
			stored:     ptr(deadCopy(base)),
			wantCode:   apperr.CodeCampaignNotFound,
		},
		{
			name:       "Error: a GM cannot delete the campaign",
			campaignID: base.ID,
			actorID:    gm,
			stored:     &base,
			wantCode:   apperr.CodeCampaignForbidden,
		},
		{
			name:       "Error: a player cannot delete the campaign",
			campaignID: base.ID,
			actorID:    player,
			stored:     &base,
			wantCode:   apperr.CodeCampaignForbidden,
		},
		{
			name:       "Error: storage read failure is internal",
			campaignID: base.ID,
			actorID:    owner,
			loadErr:    errors.New("boom"),
			wantCode:   apperr.CodeInternal,
		},
		{
			name:       "Error: a concurrent change on the mark is a conflict",
			campaignID: base.ID,
			actorID:    owner,
			stored:     &base,
			markErr:    campaign.ErrVersionConflict,
			wantCode:   apperr.CodeConflict,
			wantMark:   true,
		},
		{
			name:       "Error: a failed mark write is internal",
			campaignID: base.ID,
			actorID:    owner,
			stored:     &base,
			markErr:    errors.New("write failed"),
			wantCode:   apperr.CodeInternal,
			wantMark:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.stored != nil {
				h.repo.EXPECT().FetchByID(mock.Anything, tc.campaignID).RunAndReturn(
					func(context.Context, string) (*campaign.Campaign, error) {
						cp := campaigntest.Copy(*tc.stored)
						return &cp, nil
					},
				).Once()
			}
			if tc.loadErr != nil {
				h.repo.EXPECT().FetchByID(mock.Anything, tc.campaignID).Return(nil, tc.loadErr).Once()
			}
			if tc.wantMark {
				h.expectMarkDeleting(base.Version, tc.markErr)
			}
			if tc.wantMark && tc.markErr == nil {
				releaser.EXPECT().ReleaseOwnership(mock.Anything, owner).Return(nil).Once()
			}

			err := h.service(campaign.WithOwnershipReleaser(releaser)).Delete(context.Background(), campaign.DeleteCommand{CampaignID: tc.campaignID, ActorID: tc.actorID})

			if tc.wantCode != "" {
				requireAppErr(t, err, tc.wantCode)
			} else {
				require.NoError(t, err)
			}
			if !tc.wantMark {
				assert.Nil(t, h.saved, "a rejected deletion must not reach the mark")
				return
			}
			require.NotNil(t, h.saved)
			assert.Equal(t, campaign.StatusDeleting, h.saved.Status)
			require.NotNil(t, h.saved.DeletingAt, "deleting_at must be stamped")
			assert.Equal(t, 0, h.saved.DeletionAttempts, "a new deletion starts with zero attempts")
			assert.Nil(t, h.saved.DeletionLockedUntil)
			assert.Nil(t, h.saved.DeletionNextAttempt)
			assert.Equal(t, base.Version+1, h.saved.Version)
		})
	}
}

func TestRequestDeletion(t *testing.T) {
	base := campaigntest.Build(t, 0, 1)
	owner := campaigntest.OwnerID(base)
	tests := []struct {
		name       string
		stored     *campaign.Campaign
		loadErr    error
		wantErr    apperr.Code
		wantMark   bool
		releaseErr error
		wantState  campaign.Status
	}{
		{
			name:       "Error: a failed ownership release aborts the deletion",
			stored:     &base,
			wantMark:   true,
			releaseErr: errors.New("counter down"),
			wantErr:    apperr.CodeInternal,
		},
		{
			name:      "Happy path: an active campaign moves to deleting",
			stored:    &base,
			wantMark:  true,
			wantState: campaign.StatusDeleting,
		},
		{
			name:      "Edge: a campaign already deleting is a no-op success",
			stored:    ptr(deletingCopy(base)),
			wantState: campaign.StatusDeleting,
		},
		{
			name:      "Edge: a dead deletion is a no-op success",
			stored:    ptr(deadCopy(base)),
			wantState: campaign.StatusDeletionDead,
		},
		{
			name:    "Error: a missing campaign is not found",
			loadErr: campaign.ErrNotFound,
			wantErr: apperr.CodeCampaignNotFound,
		},
		{
			name:    "Error: a storage read failure is internal",
			loadErr: errors.New("boom"),
			wantErr: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.stored != nil {
				h.loadOnce(*tc.stored)
			}
			if tc.loadErr != nil {
				h.repo.EXPECT().FetchByID(mock.Anything, base.ID).Return(nil, tc.loadErr).Once()
			}
			releaser := mocks.NewMockOwnershipReleaser(t)
			if tc.wantMark {
				h.expectMarkDeleting(base.Version, nil)
				releaser.EXPECT().ReleaseOwnership(mock.Anything, owner).Return(tc.releaseErr).Once()
			}

			got, err := campaign.RequestDeletion(context.Background(), h.tx, h.repo, releaser, base.ID, campaigntest.Now())

			if tc.wantErr != "" {
				requireAppErr(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantState, got.Status)
			if !tc.wantMark {
				assert.Nil(t, h.saved, "an already settled campaign must not be written again")
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
