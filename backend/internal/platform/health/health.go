package health

import (
	"context"
	"time"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

// RegisterHealthRoutes mounts GET /health on group, behind the given route
// middleware.
func RegisterHealthRoutes(group *echo.Group, routeMiddleware ...echo.MiddlewareFunc) {
	group.GET("/health", func(c *echo.Context) error {
		return response.OK(c, map[string]any{"status": "healthy"})
	}, routeMiddleware...)
}

// MsgNotReady is the user-facing message of a failed readiness probe.
const MsgNotReady = "service not ready"

// Pinger checks that a dependency is reachable within the context deadline.
type Pinger interface {
	Ping(ctx context.Context) error
}

// RegisterReadyRoute mounts GET /ready on group. It answers 200 only while the
// pinger succeeds within timeout.
func RegisterReadyRoute(group *echo.Group, pinger Pinger, timeout time.Duration) {
	group.GET("/ready", func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			return apperr.NewError(apperr.CodeNotReady, MsgNotReady, err)
		}
		return response.OK(c, map[string]any{"status": "ready"})
	})
}
