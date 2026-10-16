package app_test

import (
	"net/http"
	"strings"
	"testing"

	"legnaris/internal/app"
	authmocks "legnaris/internal/auth/mocks"
	"legnaris/internal/platform/cors"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
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
	app.NewAuthController(service).RegisterRoutes(e.Group("/auth"))
	return e
}

// TestPreflightAllowsRefreshTokenHeader pins the pair a browser depends on: the
// client sends the opaque refresh token in app.RefreshTokenHeader, and the CORS
// middleware must advertise that exact name, or the preflight for
// /auth/refresh and /auth/logout fails and the request never leaves the browser.
// The platform layer cannot import the application layer, so the name exists
// twice and is kept equal here.
func TestPreflightAllowsRefreshTokenHeader(t *testing.T) {
	t.Run("Happy path: the refresh header is advertised on the auth routes", func(t *testing.T) {
		// No expectations: a preflight is answered by the middleware alone and
		// must never reach the service.
		service := authmocks.NewMockService(t)
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodOptions, "/auth/refresh", "")
		req.Header.Set(echo.HeaderOrigin, webOrigin)
		req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
		req.Header.Set(echo.HeaderAccessControlRequestHeaders, app.RefreshTokenHeader)

		rec := serve(t, h, req)
		require.Equal(t, http.StatusNoContent, rec.Code)

		advertised := strings.Split(rec.Header().Get(echo.HeaderAccessControlAllowHeaders), ",")
		assert.Contains(t, advertised, app.RefreshTokenHeader)
	})

	t.Run("Edge: a preflight from an unlisted origin advertises nothing", func(t *testing.T) {
		service := authmocks.NewMockService(t)
		h := newAuthRouterWithCORS(t, service)

		req := newRequest(http.MethodOptions, "/auth/refresh", "")
		req.Header.Set(echo.HeaderOrigin, "https://evil.example.com")
		req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)

		rec := serve(t, h, req)
		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowOrigin))
		assert.Empty(t, rec.Header().Get(echo.HeaderAccessControlAllowHeaders))
	})
}
