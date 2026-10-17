package app

import (
	"net/http"
	"net/url"
	"strings"

	"legnaris/shared/apperr"

	"github.com/labstack/echo/v5"
)

// MsgOriginNotAllowed is the user-facing message for a request refused by the
// origin guard (AUTH-13).
const MsgOriginNotAllowed = "origin is not allowed"

// requireAllowedOrigin guards the cookie-authenticated state-changing auth
// routes: it refuses a request whose Origin (falling back to the origin of
// Referer) is absent or outside the allowlist with 403 before the handler runs,
// so no credential work happens for an untrusted origin (AUTH-13).
func requireAllowedOrigin(origins []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !originAllowed(c.Request(), origins) {
				return apperr.NewError(apperr.CodeForbidden, MsgOriginNotAllowed, nil)
			}
			return next(c)
		}
	}
}

// originAllowed matches the request origin against the allowlist by exact,
// case-insensitive comparison; a wildcard is never honoured (AUTH-13, AUTH-14).
func originAllowed(r *http.Request, origins []string) bool {
	origin := requestOrigin(r)
	if origin == "" {
		return false
	}
	for _, allowed := range origins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}

// requestOrigin returns the request's Origin header, or the scheme+host origin
// of its Referer when Origin is absent.
func requestOrigin(r *http.Request) string {
	if origin := r.Header.Get(echo.HeaderOrigin); origin != "" {
		return origin
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		return ""
	}

	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
