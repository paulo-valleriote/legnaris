package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		setup      func(t *testing.T, tokens *mocks.MockTokenManager)
		wantStatus int
		wantUserID string
	}{
		{
			name:       "Happy path: valid bearer token exposes the user id",
			authHeader: "Bearer good-token",
			setup: func(t *testing.T, tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("good-token").Return("user-1", nil).Once()
			},
			wantStatus: http.StatusOK,
			wantUserID: "user-1",
		},
		{
			name:       "Edge: scheme is matched case-insensitively",
			authHeader: "bearer good-token",
			setup: func(t *testing.T, tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("good-token").Return("user-1", nil).Once()
			},
			wantStatus: http.StatusOK,
			wantUserID: "user-1",
		},
		{
			name:       "Error: missing header returns 401",
			authHeader: "",
			setup:      func(t *testing.T, tokens *mocks.MockTokenManager) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Error: wrong scheme returns 401",
			authHeader: "Token good-token",
			setup:      func(t *testing.T, tokens *mocks.MockTokenManager) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Error: empty bearer value returns 401",
			authHeader: "Bearer ",
			setup:      func(t *testing.T, tokens *mocks.MockTokenManager) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Error: invalid token returns 401",
			authHeader: "Bearer bad-token",
			setup: func(t *testing.T, tokens *mocks.MockTokenManager) {
				tokens.EXPECT().Parse("bad-token").Return("", auth.ErrInvalidToken).Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens := mocks.NewMockTokenManager(t)
			tc.setup(t, tokens)

			var seenUserID string
			var found bool

			e := echo.New()
			e.HTTPErrorHandler = app.HTTPErrorHandler
			e.Use(auth.Middleware(tokens))
			e.GET("/protected", func(c *echo.Context) error {
				seenUserID, found = auth.UserID(c)
				return c.NoContent(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantUserID != "" {
				assert.True(t, found, "user id must be present in context")
				assert.Equal(t, tc.wantUserID, seenUserID)
			}
		})
	}
}
