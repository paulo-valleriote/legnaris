package main

import (
	"log/slog"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.GET("/health", func(c *echo.Context) error {
		return c.String(200, "OK")
	})

	if err := e.Start(":8080"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}
