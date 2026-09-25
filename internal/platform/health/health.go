package health

import (
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
)

func RegisterHealthRoutes(group *echo.Group) {
	group.GET("/health", func(c *echo.Context) error {
		return response.OK(c, map[string]any{"status": "healthy"})
	})
}
