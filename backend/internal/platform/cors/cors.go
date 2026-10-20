// Package cors configures Cross-Origin Resource Sharing for the HTTP API.
package cors

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// csrfHeader mirrors app's X-CSRF-Token header. The browser reads the CSRF
// cookie and echoes it on the cookie-authenticated auth routes; a custom
// request header must be advertised here or the preflight fails and the request
// never leaves the browser. The platform layer cannot import the application
// layer, so the name is mirrored (AUTH-16).
const csrfHeader = "X-CSRF-Token"

// Middleware returns a CORS middleware that allows credentialed requests from
// the given origins (e.g. the frontend origin). Echo's default origin function
// matches case-insensitively, echoes the allowlist entry exactly, refuses to
// build a wildcard with credentials, and always adds Vary: Origin (AUTH-14,
// AUTH-15). It also handles preflight OPTIONS.
func Middleware(origins []string) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		// Clone the caller's slice so a later mutation of Variables.CORSOrigins
		// cannot race with the per-request read of AllowOrigins.
		AllowOrigins:     slices.Clone(origins),
		AllowCredentials: true,
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
			csrfHeader,
		},
		ExposeHeaders: []string{echo.HeaderXRequestID},
	})
}
