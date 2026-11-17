package app_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

func revealPath() string {
	return "/campaigns/" + campaignTestID + "/npcs/" + campaignTestSheetID + "/reveal"
}

func TestCampaignRoutes_RevealNpc(t *testing.T) {
	t.Run("Happy path: a GM reveal answers 200 with the current link", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().SetNpcRevealed(mock.Anything, campaign.RevealNpcCommand{
			CampaignID: campaignTestID,
			UserID:     campaignTestUserID,
			SheetID:    campaignTestSheetID,
			Revealed:   true,
		}).Return(campaign.NpcLinkView{
			ID:        campaignTestSheetID,
			SheetID:   campaignTestSheetID,
			Name:      "Brom",
			Kind:      plan.KindNPC,
			Revealed:  true,
			IsOwner:   true,
			OwnerName: "gm-one",
		}, nil).Once()

		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, revealPath(), "")
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, `"revealed":true`)
		assert.Contains(t, body, `"sheet_id":"`+campaignTestSheetID+`"`)
		assert.Contains(t, body, `"kind":"npc"`)
		assert.Contains(t, body, `"is_owner":true`)
	})

	t.Run("Happy path: a GM hide answers 200 with the hidden link", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().SetNpcRevealed(mock.Anything, campaign.RevealNpcCommand{
			CampaignID: campaignTestID,
			UserID:     campaignTestUserID,
			SheetID:    campaignTestSheetID,
			Revealed:   false,
		}).Return(campaign.NpcLinkView{
			ID:       campaignTestSheetID,
			SheetID:  campaignTestSheetID,
			Kind:     plan.KindEnemy,
			Revealed: false,
		}, nil).Once()

		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodDelete, revealPath(), "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"revealed":false`)
	})

	t.Run("Edge: a repeated reveal still answers 200 with the current link", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().SetNpcRevealed(mock.Anything, mock.Anything).
			Return(campaign.NpcLinkView{ID: campaignTestSheetID, SheetID: campaignTestSheetID, Revealed: true}, nil).Once()

		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, revealPath(), "")
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestCampaignRoutes_RevealNpcErrorsMapToHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		code   apperr.Code
		status int
	}{
		{"Error: a non-GM reveal is 403", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: a lost campaign is 404", apperr.CodeCampaignNotFound, http.StatusNotFound},
		{"Error: a missing link is 404", apperr.CodeNotFound, http.StatusNotFound},
		{"Error: a malformed id is 400", apperr.CodeInvalidRequest, http.StatusBadRequest},
		{"Error: an unexpected failure is 500", apperr.CodeInternal, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().SetNpcRevealed(mock.Anything, mock.Anything).
				Return(campaign.NpcLinkView{}, apperr.NewError(tc.code, "refused", nil)).Once()
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, revealPath(), "")
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}

func TestCampaignRoutes_RevealNpcRejectsUnsupportedMethods(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPost, revealPath(), "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
