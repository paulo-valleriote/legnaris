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

// newAccountDeletionRouter mounts DELETE /me as the production router does: behind
// the bearer middleware, without the active gate, so a repeat request is idempotent.
func newAccountDeletionRouter(t *testing.T, service auth.Service, tokens auth.TokenManager) http.Handler {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	controller := app.NewAuthController(service)
	e.DELETE("/me", controller.DeleteAccount, auth.Middleware(tokens))
	return e
}

// newGatedRouter mounts a protected route the way the router does: bearer
// middleware, then the active gate.
func newGatedRouter(t *testing.T, service auth.Service, tokens auth.TokenManager) http.Handler {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.GET("/sheets", func(c *echo.Context) error { return c.NoContent(http.StatusOK) },
		auth.Middleware(tokens), auth.RejectDeleting(service))
	return e
}

// TestAccountDeletionController covers DELETE /me: 202 with the deleting status,
// the refusal without a usable credential, and the typed error of a store failure.
func TestAccountDeletionController(t *testing.T) {
	tokens, err := auth.NewTokenManager("deletion-test-secret", 15*time.Minute, time.Now)
	require.NoError(t, err)
	access, err := tokens.Issue(deletionTestUserID)
	require.NoError(t, err)

	tests := []struct {
		name       string
		authHeader string
		requests   int
		setup      func(service *authmocks.MockService)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "Happy path: the request is accepted with the deleting status",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, deletionTestUserID).Return(nil).Once()
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Edge: a repeated request for a deleting account is accepted again",
			authHeader: "Bearer " + access.Value,
			requests:   2,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, deletionTestUserID).Return(nil).Twice()
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "Error: a request without a credential is unauthorized and changes nothing",
			authHeader: "",
			setup: func(service *authmocks.MockService) {
				service.AssertNotCalled(t, "DeleteAccount", mock.Anything, mock.Anything)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: an unknown account is unauthorized",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, deletionTestUserID).
					Return(apperr.NewError(apperr.CodeUnauthorized, auth.MsgAccountUnavailable, user.ErrUserNotFound)).Once()
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   string(apperr.CodeUnauthorized),
		},
		{
			name:       "Error: a store failure is an internal error",
			authHeader: "Bearer " + access.Value,
			setup: func(service *authmocks.MockService) {
				service.EXPECT().DeleteAccount(mock.Anything, deletionTestUserID).
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
				req := newRequest(http.MethodDelete, "/me", "")
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
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
		})
	}
}

// TestActiveGate covers the gate on a protected route: an active account reaches the
// handler, a deleting or dead one is forbidden, and a missing credential stays unauthorized.
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
