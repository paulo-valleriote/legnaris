package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adminAllowedID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"
	adminDeniedID  = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e10"
)

func TestAdminOnly(t *testing.T) {
	tests := []struct {
		name       string
		adminIDs   []string
		authHeader string
		setup      func(tokens *mocks.MockTokenManager)
		wantStatus int
		wantUserID string
	}{
		{
			name:       "Happy path: an allowlisted session reaches the handler",
			adminIDs:   []string{adminAllowedID},
			authHeader: "Bearer admin-token",
			setup: func(tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("admin-token").Return(adminAllowedID, nil).Once()
			},
			wantStatus: http.StatusOK,
			wantUserID: adminAllowedID,
		},
		{
			name:       "Edge: an empty allowlist hides every route",
			adminIDs:   nil,
			authHeader: "Bearer admin-token",
			setup: func(tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("admin-token").Return(adminAllowedID, nil).Once()
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Edge: a valid session outside the allowlist is hidden",
			adminIDs:   []string{adminAllowedID},
			authHeader: "Bearer user-token",
			setup: func(tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("user-token").Return(adminDeniedID, nil).Once()
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Edge: an unauthenticated request is hidden",
			adminIDs:   []string{adminAllowedID},
			authHeader: "",
			setup:      func(*mocks.MockTokenManager) {},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Edge: an invalid token is hidden",
			adminIDs:   []string{adminAllowedID},
			authHeader: "Bearer bad-token",
			setup: func(tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("bad-token").Return("", auth.ErrInvalidToken).Once()
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens := mocks.NewMockTokenManager(t)
			tc.setup(tokens)

			var seenUserID string
			var found bool

			e := echo.New()
			e.HTTPErrorHandler = app.HTTPErrorHandler
			e.GET("/admin/users/:id", func(c *echo.Context) error {
				seenUserID, found = auth.UserID(c)
				return c.NoContent(http.StatusOK)
			}, auth.AdminOnly(tokens, tc.adminIDs))

			req := httptest.NewRequest(http.MethodGet, "/admin/users/"+adminDeniedID, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus == http.StatusNotFound {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, "NOT_FOUND", body.Error.Code)
				assert.False(t, found, "a hidden route must not reach the handler")
				return
			}
			assert.True(t, found)
			assert.Equal(t, tc.wantUserID, seenUserID)
		})
	}
}
