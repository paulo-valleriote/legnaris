package app

import (
	"errors"
	platformHTTP "legnaris/internal/platform/http"
	"legnaris/shared/apperr"
	"legnaris/shared/env"
	"legnaris/shared/response"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func NewRouter(deps env.Env, controllers *Controllers) *echo.Echo {
	router := echo.New()
	router.HTTPErrorHandler = HTTPErrorHandler

	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	// usershttp.RegisterRoutes(router.Group("/users"), deps.Users)
	platformHTTP.RegisterHealthRoutes(router.Group(""))
	controllers.Template.RegisterRoutes(router.Group("/templates"))
	controllers.Sheet.RegisterRoutes(router.Group("/sheets"))

	return router
}

func HTTPErrorHandler(c *echo.Context, err error) {
	var appErr *apperr.Error

	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.HttpStatus(), response.Error{
			Error: response.ErrorBody{
				Code:    string(appErr.Code),
				Message: appErr.Message,
			},
		})
		return
	}

	_ = c.JSON(http.StatusInternalServerError, response.Error{
		Error: response.ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		},
	})
}
