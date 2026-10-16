// Package cors configures Cross-Origin Resource Sharing for the HTTP API.
package cors

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// refreshTokenHeader mirrors app.RefreshTokenHeader (internal/app/auth.go). The
// browser client sends the opaque refresh token on /auth/refresh and
// /auth/logout; a custom request header has to be advertised here or the
// preflight fails and the request never leaves the browser. The platform layer
// cannot import the application layer, so the name is mirrored and the pair is
// pinned by the boundary test in internal/app (cors_test.go).
const refreshTokenHeader = "X-Refresh-Token"

// Middleware returns a CORS middleware that allows requests from the given
// origins (e.g. the frontend origin). It also handles preflight OPTIONS.
func Middleware(origins []string) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		// Clone the caller's slice so a later mutation of Variables.CORSOrigins
		// cannot race with the per-request read of AllowOrigins.
		AllowOrigins: slices.Clone(origins),
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
			refreshTokenHeader,
		},
		ExposeHeaders: []string{echo.HeaderXRequestID},
	})
}
