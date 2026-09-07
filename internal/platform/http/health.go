package http

import (
	"legnaris/shared/env"

	"github.com/labstack/echo/v5"
)

func RegisterRoutes(group *echo.Group, deps env.Env) {
	group.GET("/health", func(c *echo.Context) error {
		return c.JSON(200, map[string]any{"status": "healthy"})
	})
}
