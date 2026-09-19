package http

import (
	"github.com/labstack/echo/v5"
)

func RegisterHealthRoutes(group *echo.Group) {
	group.GET("/health", func(c *echo.Context) error {
		return c.JSON(200, map[string]any{"status": "healthy"})
	})
}
