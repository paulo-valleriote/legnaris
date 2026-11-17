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

func TestCampaignRoutes_LinkNpcStatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		code   apperr.Code
		status int
	}{
		{"Error: a character sheet is 422", apperr.CodeSheetKindMismatch, http.StatusUnprocessableEntity},
		{"Error: a template mismatch is 422", apperr.CodeTemplateMismatch, http.StatusUnprocessableEntity},
		{"Error: a non-GM is 403", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: a foreign sheet is 404", apperr.CodeNotFound, http.StatusNotFound},
		{"Error: a lost campaign is 404", apperr.CodeCampaignNotFound, http.StatusNotFound},
		{"Error: an over-limit duplicate is 403", apperr.CodePlanLimitReached, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().LinkNpc(mock.Anything, mock.Anything).
				Return(campaign.NpcLinkView{}, apperr.NewError(tc.code, "refused", nil)).Once()
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut,
				"/campaigns/"+campaignTestID+"/npcs", `{"sheet_id":"`+campaignTestSheetID+`"}`)
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}

func TestCampaignRoutes_LinkNpcPassesTheCommand(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	svc.EXPECT().LinkNpc(mock.Anything, campaign.LinkNpcCommand{
		CampaignID: campaignTestID,
		UserID:     campaignTestUserID,
		SheetID:    campaignTestSheetID,
	}).Return(campaign.NpcLinkView{ID: campaignTestSheetID, SheetID: campaignTestSheetID, Kind: plan.KindNPC}, nil).Once()

	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut,
		"/campaigns/"+campaignTestID+"/npcs", `{"sheet_id":"`+campaignTestSheetID+`"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"sheet_id":"`+campaignTestSheetID+`"`)
}

func TestCampaignRoutes_LinkNpcRejectsMalformedBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"Edge: empty body", ``},
		{"Edge: invalid JSON", `{"sheet_id":`},
		{"Edge: wrong type for sheet_id", `{"sheet_id":42}`},
		{"Edge: unknown field", `{"sheet_id":"x","campaign_id":"spoof"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, "/campaigns/"+campaignTestID+"/npcs", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestCampaignRoutes_UnlinkNpc(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	svc.EXPECT().UnlinkNpc(mock.Anything, campaign.UnlinkNpcCommand{
		CampaignID: campaignTestID,
		UserID:     campaignTestUserID,
		SheetID:    campaignTestSheetID,
	}).Return(nil).Once()

	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodDelete,
		"/campaigns/"+campaignTestID+"/npcs/"+campaignTestSheetID, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestCampaignRoutes_ListNpcLinks(t *testing.T) {
	tests := []struct {
		name   string
		links  []campaign.NpcLinkView
		err    error
		status int
	}{
		{
			name: "Happy path: a GM lists links with owner and reveal state",
			links: []campaign.NpcLinkView{{
				ID:        campaignTestSheetID,
				SheetID:   campaignTestSheetID,
				Name:      "Brom",
				Kind:      plan.KindMonster,
				OwnerID:   campaignTestUserID,
				OwnerName: "player-one",
				Revealed:  true,
			}},
			status: http.StatusOK,
		},
		{"Edge: an empty list is still 200", nil, nil, http.StatusOK},
		{"Error: a player listing is 403", nil, apperr.NewError(apperr.CodeCampaignForbidden, "gm only", nil), http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().ListNpcLinks(mock.Anything, campaignTestID, campaignTestUserID).Return(tc.links, tc.err).Once()
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, "/campaigns/"+campaignTestID+"/npcs", "")
			assert.Equal(t, tc.status, rec.Code)
			if tc.status == http.StatusOK && len(tc.links) == 1 {
				body := rec.Body.String()
				assert.Contains(t, body, `"owner_name":"player-one"`)
				assert.Contains(t, body, `"revealed":true`)
				assert.Contains(t, body, `"kind":"monster"`)
				assert.Contains(t, body, `"is_owner":false`)
			}
		})
	}
}

func TestCampaignRoutes_FetchNpcSheet(t *testing.T) {
	const etag = `"5.3.gm"`
	path := "/campaigns/" + campaignTestID + "/npcs/" + campaignTestSheetID

	t.Run("Happy path: a GM read carries the ETag and the sheet body", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		linked := linkedSheetFixture()
		linked.Kind = plan.KindNPC
		svc.EXPECT().ReadNpcSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{ETag: etag, Sheet: linked}, nil).Once()
		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, etag, rec.Header().Get("ETag"))
		assert.Contains(t, rec.Body.String(), `"kind":"npc"`)
	})

	t.Run("Happy path: a matching If-None-Match is 304", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadNpcSheet(mock.Anything, mock.MatchedBy(func(q campaign.ReadNpcSheetQuery) bool {
			return q.IfNoneMatch == etag && q.SheetID == campaignTestSheetID && q.ViewerID == campaignTestUserID
		})).Return(campaign.LinkedSheetResult{ETag: etag, NotModified: true}, nil).Once()
		rec := doCampaignRequestWithHeader(newCampaignRouter(svc), path, "If-None-Match", etag)
		assert.Equal(t, http.StatusNotModified, rec.Code)
		assert.Equal(t, etag, rec.Header().Get("ETag"))
		assert.Empty(t, rec.Body.String())
	})

	t.Run("Error: a hidden NPC is 404 for a player", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadNpcSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{}, apperr.NewError(apperr.CodeNotFound, "sheet link not found", nil)).Once()
		rec := doCampaignRequestWithHeader(newCampaignRouter(svc), path, "If-None-Match", etag)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NOT_FOUND", campaignErrorCode(t, rec))
		assert.Empty(t, rec.Header().Get("ETag"))
	})

	t.Run("Error: an unlinked sheet is 404", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadNpcSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{}, apperr.NewError(apperr.CodeNotFound, "sheet link not found", nil)).Once()
		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, path, "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NOT_FOUND", campaignErrorCode(t, rec))
	})
}

func TestCampaignRoutes_NpcRoutesRejectUnsupportedMethods(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPatch, "/campaigns/"+campaignTestID+"/npcs", `{}`)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
