package health

import (
	"context"
	"time"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
)

func RegisterHealthRoutes(group *echo.Group, routeMiddleware ...echo.MiddlewareFunc) {
	group.GET("/health", func(c *echo.Context) error {
		return response.OK(c, map[string]any{"status": "healthy"})
	}, routeMiddleware...)
}

const MsgNotReady = "service not ready"

type Pinger interface {
	Ping(ctx context.Context) error
}

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
