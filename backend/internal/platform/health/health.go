package health

import (
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
