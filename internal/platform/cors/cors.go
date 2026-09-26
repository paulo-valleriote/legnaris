// Package cors configures Cross-Origin Resource Sharing for the HTTP API.
package cors

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

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
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
		},
		ExposeHeaders: []string{echo.HeaderXRequestID},
	})
}
