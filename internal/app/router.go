package app

import (
	platformHTTP "legnaris/internal/platform/http"
	"legnaris/shared/env"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func NewRouter(deps env.Env) *echo.Echo {
	router := echo.New()

	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	// usershttp.RegisterRoutes(router.Group("/users"), deps.Users)
	platformHTTP.RegisterRoutes(router.Group(""), deps)

	return router
}
