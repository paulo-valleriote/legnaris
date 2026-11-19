package auth

import (
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/requestcache"

	"github.com/labstack/echo/v5"
)

const MsgNotFound = "not found"

func AdminOnly(tokens TokenManager, adminIDs []string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(adminIDs))
	for _, id := range adminIDs {
		allowed[id] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw, ok := bearerToken(c.Request().Header.Get("Authorization"))
			if !ok {
				return hiddenNotFound()
			}
			userID, err := tokens.Parse(raw)
			if err != nil {
				return hiddenNotFound()
			}
			if _, allowed := allowed[userID]; !allowed {
				return hiddenNotFound()
			}

			c.Set(UserIDContextKey, userID)
			return next(c)
		}
	}
}

func AdminActive(checker ActiveAccountChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := UserID(c)
			if !ok {
				return hiddenNotFound()
			}
			c.SetRequest(c.Request().WithContext(requestcache.WithScope(c.Request().Context())))
			if err := checker.EnsureActive(c.Request().Context(), userID); err != nil {
				return hiddenNotFound()
			}
			return next(c)
		}
	}
}

func hiddenNotFound() error {
	return apperr.NewError(apperr.CodeNotFound, MsgNotFound, nil)
}
