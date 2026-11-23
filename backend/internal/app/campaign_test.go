package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	campaignTestUserID   = "01900000-0000-7000-8000-0000000000bb"
	campaignTestID       = "01900000-0000-7000-8000-0000000000cc"
	campaignTestTargetID = "01900000-0000-7000-8000-0000000000dd"
)

func newCampaignRouter(svc campaign.Service) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(campaignTestUserID))
	app.NewCampaignController(svc).RegisterRoutes(e.Group("/campaigns"), passThroughMiddleware)
	return e
}

func doCampaignRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func campaignErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Error.Code
}

func TestCampaignRoutes_ServiceErrorsMapToHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		code   apperr.Code
		status int
	}{
		{"Error: missing campaign is 404", apperr.CodeCampaignNotFound, http.StatusNotFound},
		{"Error: role refusal is 403", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: plan limit on create is 403", apperr.CodePlanLimitReached, http.StatusForbidden},
		{"Error: full campaign is 409", apperr.CodeCampaignFull, http.StatusConflict},
		{"Error: concurrent change is 409", apperr.CodeConflict, http.StatusConflict},
		{"Error: template mismatch is 422", apperr.CodeTemplateMismatch, http.StatusUnprocessableEntity},
		{"Error: invalid id is 400", apperr.CodeInvalidRequest, http.StatusBadRequest},
		{"Error: unexpected failure is 500", apperr.CodeInternal, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().Fetch(mock.Anything, campaignTestID, campaignTestUserID).
				Return(campaign.View{}, apperr.NewError(tc.code, "boom", nil)).Once()
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodGet, "/campaigns/"+campaignTestID, "")
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}

func TestCampaignRoutes_Create(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		errCode  string
		service  bool
	}{
		{name: "Happy path: valid body creates the campaign", body: `{"name":"Curse","template_id":"dnd5e"}`, wantCode: http.StatusCreated, service: true},
		{name: "Edge: malformed JSON is rejected before the service", body: `{"name":`, wantCode: http.StatusBadRequest, errCode: "INVALID_JSON"},
		{name: "Edge: unknown field is rejected as INVALID_JSON before the service", body: `{"name":"x","template_id":"dnd5e","owner_id":"spoof"}`, wantCode: http.StatusBadRequest, errCode: "INVALID_JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			if tc.service {
				svc.EXPECT().Create(mock.Anything, campaign.CreateCommand{OwnerID: campaignTestUserID, Name: "Curse", TemplateID: "dnd5e"}).
					Return(campaign.View{ID: campaignTestID, Name: "Curse"}, nil).Once()
			}
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, "/campaigns", tc.body)
			assert.Equal(t, tc.wantCode, rec.Code)
			if tc.errCode != "" {
				assert.Equal(t, tc.errCode, campaignErrorCode(t, rec))
			}
		})
	}
}

func TestCampaignRoutes_CreateGameSystem(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	svc.EXPECT().Create(mock.Anything, campaign.CreateCommand{OwnerID: campaignTestUserID, Name: "Curse", GameSystemID: "fifth"}).
		Return(campaign.View{ID: campaignTestID, Name: "Curse"}, nil).Once()

	rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, "/campaigns", `{"name":"Curse","game_system":"fifth"}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCampaignRoutes_ListAndFetch(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		setup func(svc *campaignmocks.MockService)
	}{
		{
			name: "Happy path: list returns the viewer's campaigns",
			path: "/campaigns",
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().ListMine(mock.Anything, campaignTestUserID).Return([]campaign.Summary{{ID: campaignTestID}}, nil).Once()
			},
		},
		{
			name: "Edge: empty list is still 200",
			path: "/campaigns",
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().ListMine(mock.Anything, campaignTestUserID).Return(nil, nil).Once()
			},
		},
		{
			name: "Happy path: fetch returns the campaign view",
			path: "/campaigns/" + campaignTestID,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().Fetch(mock.Anything, campaignTestID, campaignTestUserID).Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			tc.setup(svc)
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodGet, tc.path, "")
			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestCampaignRoutes_Leave(t *testing.T) {
	t.Run("Happy path: leaving answers 204 with no body", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().Leave(mock.Anything, campaign.LeaveCommand{CampaignID: campaignTestID, UserID: campaignTestUserID}).Return(nil).Once()
		rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, "/campaigns/"+campaignTestID+"/leave", "")
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})
	t.Run("Error: the owner leaving is 403", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().Leave(mock.Anything, mock.Anything).Return(apperr.NewError(apperr.CodeCampaignForbidden, "owner", nil)).Once()
		rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, "/campaigns/"+campaignTestID+"/leave", "")
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "CAMPAIGN_FORBIDDEN", campaignErrorCode(t, rec))
	})
}

func TestCampaignRoutes_RoleChanges(t *testing.T) {
	want := campaign.RoleCommand{CampaignID: campaignTestID, ActorID: campaignTestUserID, TargetID: campaignTestTargetID}
	tests := []struct {
		name   string
		action string
		expect func(svc *campaignmocks.MockService)
	}{
		{
			name:   "Happy path: promote passes actor and target",
			action: "promote",
			expect: func(svc *campaignmocks.MockService) {
				svc.EXPECT().Promote(mock.Anything, want).Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
		},
		{
			name:   "Happy path: demote passes actor and target",
			action: "demote",
			expect: func(svc *campaignmocks.MockService) {
				svc.EXPECT().Demote(mock.Anything, want).Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
		},
		{
			name:   "Happy path: remove passes actor and target",
			action: "remove",
			expect: func(svc *campaignmocks.MockService) {
				svc.EXPECT().Remove(mock.Anything, want).Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			tc.expect(svc)
			path := "/campaigns/" + campaignTestID + "/members/" + campaignTestTargetID + "/" + tc.action
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, path, "")
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestCampaignRoutes_RoleChangeRefusals(t *testing.T) {
	tests := []struct {
		name   string
		action string
		code   apperr.Code
		status int
	}{
		{"Error: promote by a non-owner is 403", "promote", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: demote into a full player slot is 409", "demote", apperr.CodeCampaignFull, http.StatusConflict},
		{"Error: remove of a missing member is 404", "remove", apperr.CodeNotFound, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			failure := apperr.NewError(tc.code, "refused", nil)
			switch tc.action {
			case "promote":
				svc.EXPECT().Promote(mock.Anything, mock.Anything).Return(campaign.View{}, failure).Once()
			case "demote":
				svc.EXPECT().Demote(mock.Anything, mock.Anything).Return(campaign.View{}, failure).Once()
			case "remove":
				svc.EXPECT().Remove(mock.Anything, mock.Anything).Return(campaign.View{}, failure).Once()
			}
			path := "/campaigns/" + campaignTestID + "/members/" + campaignTestTargetID + "/" + tc.action
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodPost, path, "")
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}

func TestCampaignRoutes_Delete(t *testing.T) {
	t.Run("Happy path: deleting answers 202 with the deleting status", func(t *testing.T) {
		svc := campaignmocks.NewMockService(t)
		svc.EXPECT().Delete(mock.Anything, campaign.DeleteCommand{CampaignID: campaignTestID, ActorID: campaignTestUserID}).Return(nil).Once()
		rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodDelete, "/campaigns/"+campaignTestID, "")
		assert.Equal(t, http.StatusAccepted, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"deleting"`)
	})
	tests := []struct {
		name   string
		code   apperr.Code
		status int
	}{
		{"Error: a GM or player deleting is 403", apperr.CodeCampaignForbidden, http.StatusForbidden},
		{"Error: an unknown or already deleting campaign is 404", apperr.CodeCampaignNotFound, http.StatusNotFound},
		{"Error: a failed release is 500", apperr.CodeInternal, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			svc.EXPECT().Delete(mock.Anything, mock.Anything).Return(apperr.NewError(tc.code, "boom", nil)).Once()
			rec := doCampaignRequest(t, newCampaignRouter(svc), http.MethodDelete, "/campaigns/"+campaignTestID, "")
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, string(tc.code), campaignErrorCode(t, rec))
		})
	}
}
