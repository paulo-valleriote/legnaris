package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/app"
	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/shared/apperr"
)

const campaignInviteTestCode = "Abcdefghijklmnopqrstuvwx"

func passThroughMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return next
}

func rateLimitedMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(*echo.Context) error {
		return apperr.NewError(apperr.CodeRateLimited, "too many attempts", nil)
	}
}

func newInviteRouter(svc campaign.Service, joinIPLimit echo.MiddlewareFunc) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(campaignTestUserID))
	app.NewCampaignController(svc).RegisterRoutes(e.Group("/campaigns"), joinIPLimit)
	return e
}

func TestCampaignRoutes_Join(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		setup    func(svc *campaignmocks.MockService)
		wantCode int
		errCode  string
	}{
		{
			name: "Happy path: a valid code joins and returns the view",
			body: `{"code":"` + campaignInviteTestCode + `"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, campaign.JoinCommand{UserID: campaignTestUserID, Code: campaignInviteTestCode}).
					Return(campaign.View{ID: campaignTestID, ViewerRole: campaign.RolePlayer}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Happy path: a password travels to the service with the code",
			body: `{"code":"` + campaignInviteTestCode + `","password":"correct horse"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, campaign.JoinCommand{UserID: campaignTestUserID, Code: campaignInviteTestCode, Password: "correct horse"}).
					Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "Edge: malformed JSON is rejected before the service",
			body:     `{"code":`,
			wantCode: http.StatusBadRequest,
			errCode:  "INVALID_JSON",
		},
		{
			name:     "Edge: a client-supplied user id is rejected as an unknown field",
			body:     `{"code":"` + campaignInviteTestCode + `","user_id":"spoof"}`,
			wantCode: http.StatusBadRequest,
			errCode:  "INVALID_JSON",
		},
		{
			name: "Error: an unknown or rotated code is 404 INVALID_INVITE",
			body: `{"code":"` + campaignInviteTestCode + `"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeInvalidInvite, "invalid", nil)).Once()
			},
			wantCode: http.StatusNotFound,
			errCode:  "INVALID_INVITE",
		},
		{
			name: "Error: a missing password for a protected campaign is 403 PASSWORD_REQUIRED",
			body: `{"code":"` + campaignInviteTestCode + `"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodePasswordRequired, "password", nil)).Once()
			},
			wantCode: http.StatusForbidden,
			errCode:  "PASSWORD_REQUIRED",
		},
		{
			name: "Error: a per-campaign limit is 429 RATE_LIMITED",
			body: `{"code":"` + campaignInviteTestCode + `"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeRateLimited, "retry", nil)).Once()
			},
			wantCode: http.StatusTooManyRequests,
			errCode:  "RATE_LIMITED",
		},
		{
			name: "Error: a full campaign is 409 CAMPAIGN_FULL",
			body: `{"code":"` + campaignInviteTestCode + `"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().JoinByCode(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeCampaignFull, "full", nil)).Once()
			},
			wantCode: http.StatusConflict,
			errCode:  "CAMPAIGN_FULL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			if tc.setup != nil {
				tc.setup(svc)
			}
			rec := doCampaignRequest(newInviteRouter(svc, passThroughMiddleware), http.MethodPost, "/campaigns/join", tc.body)
			assert.Equal(t, tc.wantCode, rec.Code)
			if tc.errCode != "" {
				assert.Equal(t, tc.errCode, campaignErrorCode(t, rec))
				assert.NotContains(t, rec.Body.String(), campaignInviteTestCode, "an error must never echo the invite code")
			}
		})
	}
}

func TestCampaignRoutes_JoinIsRateLimitedPerIPBeforeTheService(t *testing.T) {
	svc := campaignmocks.NewMockService(t)
	rec := doCampaignRequest(newInviteRouter(svc, rateLimitedMiddleware), http.MethodPost, "/campaigns/join", `{"code":"`+campaignInviteTestCode+`"}`)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "RATE_LIMITED", campaignErrorCode(t, rec))
}

func TestCampaignRoutes_RotateInvite(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(svc *campaignmocks.MockService)
		wantCode int
		errCode  string
	}{
		{
			name: "Happy path: a GM rotates the code and gets the new view",
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().RotateInviteCode(mock.Anything, campaign.InviteCommand{CampaignID: campaignTestID, ActorID: campaignTestUserID}).
					Return(campaign.View{ID: campaignTestID, InviteCode: campaignInviteTestCode}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name: "Error: a player cannot rotate the code",
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().RotateInviteCode(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeCampaignForbidden, "gm only", nil)).Once()
			},
			wantCode: http.StatusForbidden,
			errCode:  "CAMPAIGN_FORBIDDEN",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			tc.setup(svc)
			rec := doCampaignRequest(newInviteRouter(svc, passThroughMiddleware), http.MethodPost, "/campaigns/"+campaignTestID+"/invite/rotate", "")
			assert.Equal(t, tc.wantCode, rec.Code)
			if tc.errCode != "" {
				assert.Equal(t, tc.errCode, campaignErrorCode(t, rec))
			}
		})
	}
}

func TestCampaignRoutes_Password(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		setup    func(svc *campaignmocks.MockService)
		wantCode int
		errCode  string
	}{
		{
			name:   "Happy path: PUT sets the password",
			method: http.MethodPut,
			body:   `{"password":"correct horse"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().SetPassword(mock.Anything, campaign.PasswordCommand{CampaignID: campaignTestID, ActorID: campaignTestUserID, Password: "correct horse"}).
					Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "Edge: PUT with an unknown field is rejected before the service",
			method:   http.MethodPut,
			body:     `{"password":"correct horse","extra":true}`,
			wantCode: http.StatusBadRequest,
			errCode:  "INVALID_JSON",
		},
		{
			name:   "Error: PUT with a password that is too short is 400",
			method: http.MethodPut,
			body:   `{"password":"short"}`,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().SetPassword(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeInvalidRequest, "too short", nil)).Once()
			},
			wantCode: http.StatusBadRequest,
			errCode:  "INVALID_REQUEST",
		},
		{
			name:   "Happy path: DELETE clears the password",
			method: http.MethodDelete,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().ClearPassword(mock.Anything, campaign.InviteCommand{CampaignID: campaignTestID, ActorID: campaignTestUserID}).
					Return(campaign.View{ID: campaignTestID}, nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name:   "Error: DELETE by a player is 403",
			method: http.MethodDelete,
			setup: func(svc *campaignmocks.MockService) {
				svc.EXPECT().ClearPassword(mock.Anything, mock.Anything).Return(campaign.View{}, apperr.NewError(apperr.CodeCampaignForbidden, "gm only", nil)).Once()
			},
			wantCode: http.StatusForbidden,
			errCode:  "CAMPAIGN_FORBIDDEN",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := campaignmocks.NewMockService(t)
			if tc.setup != nil {
				tc.setup(svc)
			}
			req := httptest.NewRequest(tc.method, "/campaigns/"+campaignTestID+"/password", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newInviteRouter(svc, passThroughMiddleware).ServeHTTP(rec, req)
			assert.Equal(t, tc.wantCode, rec.Code)
			if tc.errCode != "" {
				assert.Equal(t, tc.errCode, campaignErrorCode(t, rec))
			}
		})
	}
}
