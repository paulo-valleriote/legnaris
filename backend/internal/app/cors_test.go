package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	authmocks "legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/platform/cors"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// webOrigin mirrors the deployment's WEB_DOMAIN (deploy/.env.example), the one
// origin the browser client calls the API from.
const webOrigin = "https://app.legnaris.com.br"

func newAuthRouterWithCORS(t *testing.T, service *authmocks.MockService) http.Handler {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(cors.Middleware([]string{webOrigin}))

	controller := app.NewAuthController(service)
	group := e.Group("/auth")
	group.POST("/register", controller.Register)
	group.POST("/login", controller.Login)
	group.POST("/refresh", controller.Refresh)
	group.POST("/logout", controller.Logout)
	return e
}

// advertisedHeaders splits the Access-Control-Allow-Headers response header.
func advertisedHeaders(rec *httptest.ResponseRecorder) []string {
	raw := rec.Header().Get(echo.HeaderAccessControlAllowHeaders)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func TestCORSPreflight(t *testing.T) {
	t.Run("Happy path: the allowlisted origin is echoed with credentials and X-CSRF-Token", func(t *testing.T) {
		service := authmocks.NewMockService(t) // a preflight never reaches the service
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodOptions, "/auth/refresh", "")
		req.Header.Set(echo.HeaderOrigin, webOrigin)
		req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
		req.Header.Set(echo.HeaderAccessControlRequestHeaders, "X-CSRF-Token")

		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		assert.Equal(t, webOrigin, rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
		assert.Equal(t, "true", rec.Header().Get(echo.HeaderAccessControlAllowCredentials))
		assert.Contains(t, rec.Header().Get(echo.HeaderVary), echo.HeaderOrigin)

		advertised := advertisedHeaders(rec)
		assert.Contains(t, advertised, "X-CSRF-Token")
		assert.NotContains(t, advertised, "X-Refresh-Token", "the retired native header must not be advertised")
	})

	t.Run("Edge: a preflight from an unlisted origin advertises nothing", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodOptions, "/auth/refresh", "")
		req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
		req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
		req.Header.Set(echo.HeaderAccessControlRequestHeaders, "X-CSRF-Token")

		rec := serve(t, h, req)
		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowHeaders))
	})
}

func TestCORSActualRequest(t *testing.T) {
	t.Run("Happy path: the allowlisted origin is echoed exactly, never a wildcard", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().Login(mock.Anything, mock.Anything).Return(sessionOutcome(), nil).Once()
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		req.Header.Set(echo.HeaderOrigin, webOrigin)

		rec := serve(t, h, req)
		require.Equal(t, http.StatusOK, rec.Code)

		allowOrigin := rec.Header().Get(echo.HeaderAccessControlAllowOrigin)
		assert.Equal(t, webOrigin, allowOrigin)
		assert.NotEqual(t, "*", allowOrigin)
		assert.Equal(t, "true", rec.Header().Get(echo.HeaderAccessControlAllowCredentials))
		assert.Contains(t, rec.Header().Get(echo.HeaderVary), echo.HeaderOrigin)
	})

	t.Run("Edge: a request from an unlisted origin receives no allow-origin and never a wildcard", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		service.EXPECT().Login(mock.Anything, mock.Anything).Return(sessionOutcome(), nil).Once()
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodPost, "/auth/login", `{"email":"player@example.com","password":"sup3r-secret"}`)
		req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")

		rec := serve(t, h, req)
		allowOrigin := rec.Header().Get(echo.HeaderAccessControlAllowOrigin)
		assert.Empty(t, allowOrigin)
		assert.NotEqual(t, "*", allowOrigin)
	})
}
