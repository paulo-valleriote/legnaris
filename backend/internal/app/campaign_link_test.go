package app_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const campaignTestSheetID = "01900000-0000-7000-8000-0000000000ee"

func linkedSheetFixture() *sheet.Sheet {
	linked := campaignTestID
	return &sheet.Sheet{
		ID:            campaignTestSheetID,
		TemplateID:    "dnd5e",
		OwnerID:       campaignTestUserID,
		Kind:          plan.KindCharacter,
		Name:          "Aria",
		Data:          map[string]any{"hp": 12.0},
		Version:       4,
		LayoutVersion: 2,
		CampaignID:    &linked,
		CreatedAt:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestCampaignRoutes_LinkErrorsMapToHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		code   apperr.Code
		status int
	}{
		{"Error: template mismatch is 422", apperr.CodeTemplateMismatch, http.StatusUnprocessableEntity},
		{"Error: sheet already linked is 409", apperr.CodeSheetAlreadyLinked, http.StatusConflict},
		{"Error: GM link refusal is 403", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: lost campaign access is 404", apperr.CodeCampaignNotFound, http.StatusNotFound},
		{"Error: sheet not found is 404", apperr.CodeNotFound, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().Link(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(tc.code, "refused", nil)).Once()
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, "/campaigns/"+campaignTestID+"/character", `{"sheet_id":"`+campaignTestSheetID+`"}`)
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}

func TestCampaignRoutes_LinkPassesSheetAndCaller(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	svc.EXPECT().Link(mock.Anything, campaign.LinkCommand{
		CampaignID: campaignTestID,
		UserID:     campaignTestUserID,
		SheetID:    campaignTestSheetID,
	}).Return(campaign.View{ID: campaignTestID, Version: 3}, nil).Once()

	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, "/campaigns/"+campaignTestID+"/character", `{"sheet_id":"`+campaignTestSheetID+`"}`)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCampaignRoutes_LinkRejectsMalformedBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"Edge: empty body", ``},
		{"Edge: invalid JSON", `{"sheet_id":`},
		{"Edge: wrong type for sheet_id", `{"sheet_id":42}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodPut, "/campaigns/"+campaignTestID+"/character", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestCampaignRoutes_Unlink(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	svc.EXPECT().Unlink(mock.Anything, campaign.UnlinkCommand{
		CampaignID: campaignTestID,
		UserID:     campaignTestUserID,
	}).Return(campaign.View{ID: campaignTestID, Version: 2}, nil).Once()

	rec := doCampaignRequest(newCampaignRouter(svc), http.MethodDelete, "/campaigns/"+campaignTestID+"/character", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCampaignRoutes_ListLinkedSheets(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"Happy path: GM lists linked sheets", nil, http.StatusOK},
		{"Error: player listing is 403", apperr.NewError(apperr.CodeCampaignForbidden, "gm only", nil), http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().ListLinkedSheets(mock.Anything, campaignTestID, campaignTestUserID).
				Return([]campaign.LinkedSheetSummary{{ID: campaignTestSheetID, Name: "Aria"}}, tc.err).Once()
			rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, "/campaigns/"+campaignTestID+"/sheets", "")
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

func TestCampaignRoutes_FetchLinkedSheet(t *testing.T) {
	const etag = `"4.2.gm"`
	path := "/campaigns/" + campaignTestID + "/sheets/" + campaignTestSheetID

	t.Run("Happy path: full read carries ETag and the sheet body", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadLinkedSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{ETag: etag, Sheet: linkedSheetFixture()}, nil).Once()
		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, etag, rec.Header().Get("ETag"))
		assert.Contains(t, rec.Body.String(), `"campaign_id":"`+campaignTestID+`"`)
	})

	t.Run("Happy path: matching If-None-Match is 304 with no body", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadLinkedSheet(mock.Anything, mock.MatchedBy(func(q campaign.ReadLinkedSheetQuery) bool {
			return q.IfNoneMatch == etag && q.SheetID == campaignTestSheetID && q.ViewerID == campaignTestUserID
		})).Return(campaign.LinkedSheetResult{ETag: etag, NotModified: true}, nil).Once()

		req := newCampaignRouter(svc)
		rec := doCampaignRequestWithHeader(req, path, "If-None-Match", etag)
		assert.Equal(t, http.StatusNotModified, rec.Code)
		assert.Equal(t, etag, rec.Header().Get("ETag"))
		assert.Empty(t, rec.Body.String())
	})

	t.Run("Error: lost access is 404 even when the ETag matches", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadLinkedSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{}, apperr.NewError(apperr.CodeCampaignNotFound, "campaign not found", nil)).Once()
		rec := doCampaignRequestWithHeader(newCampaignRouter(svc), path, "If-None-Match", etag)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "CAMPAIGN_NOT_FOUND", campaignErrorCode(t, rec))
		assert.Empty(t, rec.Header().Get("ETag"))
	})

	t.Run("Error: player reading another player's sheet is 403", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().ReadLinkedSheet(mock.Anything, mock.Anything).
			Return(campaign.LinkedSheetResult{}, apperr.NewError(apperr.CodeCampaignForbidden, "forbidden", nil)).Once()
		rec := doCampaignRequest(newCampaignRouter(svc), http.MethodGet, path, "")
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func doCampaignRequestWithHeader(h http.Handler, path, name, value string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(name, value)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
