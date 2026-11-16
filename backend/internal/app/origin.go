package app

import (
	"net/http"
	"net/url"
	"strings"

	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
)

const MsgOriginNotAllowed = "origin is not allowed"

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
