package auth

import (
	"context"
	"strings"

	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
)

// UserIDContextKey is where the middleware stores the authenticated user id.
const UserIDContextKey = "user_id"

const (
	MsgAuthorizationRequired = "authorization bearer token is required"
	MsgAuthorizationInvalid  = "authorization token is invalid"
)

// Middleware authenticates requests from the Authorization: Bearer header and
// exposes the user id through UserID. It is applied to protected route groups in
// Phase 2 (ownership); Phase 1 only ships and tests it.
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

// ActiveAccountChecker reports whether an authenticated account may still act. It
// returns a typed error when the account is unknown or being deleted.
type ActiveAccountChecker interface {
	EnsureActive(ctx context.Context, userID string) error
}

// RejectDeleting refuses an authenticated request whose account is being deleted
// (LGN-190). It must be installed after Middleware, which sets the user id.
func RejectDeleting(checker ActiveAccountChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := UserID(c)
			if !ok {
				return apperr.NewError(apperr.CodeUnauthorized, MsgAuthorizationRequired, nil)
			}
			if err := checker.EnsureActive(c.Request().Context(), userID); err != nil {
				return err
			}
			return next(c)
		}
	}
}

// UserID returns the authenticated user id set by Middleware.
func UserID(c *echo.Context) (string, bool) {
	userID, ok := c.Get(UserIDContextKey).(string)
	return userID, ok
}

// bearerToken extracts the token from an Authorization header, matching the
// scheme case-insensitively and requiring a non-empty value.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
