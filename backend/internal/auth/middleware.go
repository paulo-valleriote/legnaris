package auth

import (
	"context"
	"strings"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/requestcache"

	"github.com/labstack/echo/v5"
)

const UserIDContextKey = "user_id"

const (
	MsgAuthorizationRequired = "authorization bearer token is required"
	MsgAuthorizationInvalid  = "authorization token is invalid"
)

func Middleware(tokens TokenManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw, ok := bearerToken(c.Request().Header.Get("Authorization"))
			if !ok {
				return apperr.NewError(apperr.CodeUnauthorized, MsgAuthorizationRequired, nil)
			}

			userID, err := tokens.Parse(raw)
			if err != nil {
				return apperr.NewError(apperr.CodeUnauthorized, MsgAuthorizationInvalid, err)
			}

			c.Set(UserIDContextKey, userID)
			return next(c)
		}
	}
}

type ActiveAccountChecker interface {
	EnsureActive(ctx context.Context, userID string) error
}

func RejectDeleting(checker ActiveAccountChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := UserID(c)
			if !ok {
				return apperr.NewError(apperr.CodeUnauthorized, MsgAuthorizationRequired, nil)
			}
			c.SetRequest(c.Request().WithContext(requestcache.WithScope(c.Request().Context())))
			if err := checker.EnsureActive(c.Request().Context(), userID); err != nil {
				return err
			}
			return next(c)
		}
	}
}

func UserID(c *echo.Context) (string, bool) {
	userID, ok := c.Get(UserIDContextKey).(string)
	return userID, ok
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
