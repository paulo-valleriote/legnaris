package app_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	authmocks "legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
)

const deletionTestUserID = "01900000-0000-7000-8000-000000000001"

const (
	deletionChallengeID = "01900000-0000-7000-8000-000000000055"
	deletionCode        = "042917"
)

var deletionBody = `{"challenge_id":"` + deletionChallengeID + `","code":"` + deletionCode + `"}`

func newAccountDeletionRouter(t *testing.T, service auth.Service, tokens auth.TokenManager) http.Handler {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	controller := app.NewAuthController(service)
	e.POST("/me/deletion", controller.RequestAccountDeletion, auth.Middleware(tokens), auth.RejectDeleting(service))
	e.DELETE("/me", controller.DeleteAccount, auth.Middleware(tokens))
	return e
}

func newGatedRouter(t *testing.T, service auth.Service, tokens auth.TokenManager) http.Handler {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.GET("/sheets", func(c *echo.Context) error { return c.NoContent(http.StatusOK) },
		auth.Middleware(tokens), auth.RejectDeleting(service))
	return e
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Error.Code
}

func TestRequestAccountDeletionController(t *testing.T) {
	tokens, err := auth.NewTokenManager("deletion-test-secret", 15*time.Minute, time.Now)
	require.NoError(t, err)
	access, err := tokens.Issue(deletionTestUserID)
	require.NoError(t, err)

	issued := &auth.ChallengeIssued{ChallengeID: deletionChallengeID, ExpiresIn: 600, ResendAfter: 60}

	tests := []struct {
		name       string
		authHeader string
		setup      func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "Happy path: the code is sent and the challenge is returned",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().EnsureActive(mock.Anything, deletionTestUserID).Return(nil).Once()
				service.EXPECT().RequestAccountDeletion(mock.Anything, deletionTestUserID).Return(issued, nil).Once()
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Error: a deleting account cannot ask for another code",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().EnsureActive(mock.Anything, deletionTestUserID).
					Return(apperr.NewError(apperr.CodeForbidden, auth.MsgAccountDeleting, nil)).Once()
				service.AssertNotCalled(t, "RequestAccountDeletion", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   string(apperr.CodeForbidden),
		},
		{
			name:       "Error: a request without a credential is unauthorized and sends nothing",
			authHeader: "",
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "RequestAccountDeletion", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: the daily send cap is the typed unavailable outcome",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().EnsureActive(mock.Anything, deletionTestUserID).Return(nil).Once()
				service.EXPECT().RequestAccountDeletion(mock.Anything, deletionTestUserID).
					Return(nil, apperr.NewError(apperr.CodeEmailUnavailable, auth.MsgEmailUnavailable, nil)).Once()
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   string(apperr.CodeEmailUnavailable),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := authmocks.NewMockService(t)
			tc.setup(service)

			req := newRequest(http.MethodPost, "/me/deletion", "")
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := serve(t, newAccountDeletionRouter(t, service, tokens), req)
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())

			if tc.wantStatus == http.StatusAccepted {
				var body struct {
					Data auth.ChallengeIssued `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, *issued, body.Data)
				return
			}
			assert.Equal(t, tc.wantCode, errorCode(t, rec))
		})
	}
}

func TestAccountDeletionController(t *testing.T) {
	tokens, err := auth.NewTokenManager("deletion-test-secret", 15*time.Minute, time.Now)
	require.NoError(t, err)
	access, err := tokens.Issue(deletionTestUserID)
	require.NoError(t, err)

	wantCmd := auth.DeleteAccountCommand{UserID: deletionTestUserID, ChallengeID: deletionChallengeID, Code: deletionCode}

	tests := []struct {
		name       string
		authHeader string
		body       string
		requests   int
		setup      func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "Happy path: the accepted code starts the deletion",
			authHeader: "Bearer " + access.Value,
			body:       deletionBody,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, wantCmd).Return(nil).Once()
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Edge: a repeated confirmation for a deleting account is accepted again",
			authHeader: "Bearer " + access.Value,
			body:       deletionBody,
			requests:   2,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, wantCmd).Return(nil).Twice()
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Edge: a missing challenge_id is an invalid request and changes nothing",
			authHeader: "Bearer " + access.Value,
			body:       `{"code":"042917"}`,
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "DeleteAccount", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidRequest),
		},
		{
			name:       "Edge: a missing code is an invalid request and changes nothing",
			authHeader: "Bearer " + access.Value,
			body:       `{"challenge_id":"` + deletionChallengeID + `"}`,
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "DeleteAccount", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidRequest),
		},
		{
			name:       "Edge: an empty body is invalid JSON and changes nothing",
			authHeader: "Bearer " + access.Value,
			body:       "",
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "DeleteAccount", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidJSON),
		},
		{
			name:       "Error: a wrong code is the uniform code refusal",
			authHeader: "Bearer " + access.Value,
			body:       deletionBody,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, wantCmd).
					Return(apperr.NewError(apperr.CodeOTPInvalid, auth.MsgOTPInvalid, nil)).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeOTPInvalid),
		},
		{
			name:       "Error: a request without a credential is unauthorized and changes nothing",
			authHeader: "",
			body:       deletionBody,
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "DeleteAccount", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: an unknown account is unauthorized",
			authHeader: "Bearer " + access.Value,
			body:       deletionBody,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, wantCmd).
					Return(apperr.NewError(apperr.CodeUnauthorized, auth.MsgAccountUnavailable, user.ErrUserNotFound)).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: a store failure is an internal error",
			authHeader: "Bearer " + access.Value,
			body:       deletionBody,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, wantCmd).
					Return(apperr.NewError(apperr.CodeInternal, auth.MsgInternal, errors.New("mongo down"))).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   string(apperr.CodeInternal),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := authmocks.NewMockService(t)
			tc.setup(service)

			requests := max(tc.requests, 1)
			var rec *httptest.ResponseRecorder
			for range requests {
				req := newRequest(http.MethodDelete, "/me", tc.body)
				if tc.authHeader != "" {
					req.Header.Set("Authorization", tc.authHeader)
				}
				rec = serve(t, newAccountDeletionRouter(t, service, tokens), req)
				require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			}

			if tc.wantStatus == http.StatusAccepted {
				var body struct {
					Data struct {
						Status string `json:"status"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, user.StatusDeleting, body.Data.Status)
				return
			}
			assert.Equal(t, tc.wantCode, errorCode(t, rec))
		})
	}
}

func TestActiveGate(t *testing.T) {
	tokens, err := auth.NewTokenManager("gate-test-secret", 15*time.Minute, time.Now)
	require.NoError(t, err)
	access, err := tokens.Issue(deletionTestUserID)
	require.NoError(t, err)

	tests := []struct {
		name       string
		authHeader string
		setup      func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "Happy path: an active account reaches the protected route",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().EnsureActive(mock.Anything, deletionTestUserID).Return(nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Error: a deleting account is forbidden on a protected route",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().EnsureActive(mock.Anything, deletionTestUserID).
					Return(apperr.NewError(apperr.CodeForbidden, auth.MsgAccountDeleting, nil)).Once()
			},
			wantStatus: http.StatusForbidden,
			wantCode:   string(apperr.CodeForbidden),
		},
		{
			name:       "Error: a missing credential never reaches the gate",
			authHeader: "",
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "EnsureActive", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := authmocks.NewMockService(t)
			tc.setup(service)

			req := newRequest(http.MethodGet, "/sheets", "")
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := serve(t, newGatedRouter(t, service, tokens), req)
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantCode != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
			}
		})
	}
}
