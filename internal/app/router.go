package app

import (
	"errors"
	platformHTTP "legnaris/internal/platform/http"
	"legnaris/shared/apperr"
	"legnaris/shared/env"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(deps env.Env, controllers *Controllers) *echo.Echo {
	router := echo.New()
	router.HTTPErrorHandler = HTTPErrorHandler

	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	// usershttp.RegisterRoutes(router.Group("/users"), deps.Users)
	platformHTTP.RegisterHealthRoutes(router.Group(""))
	controllers.Template.RegisterRoutes(router.Group("/templates"))

	return router
}

func HTTPErrorHandler(c *echo.Context, err error) {
	var appErr *apperr.Error

	if errors.As(err, &appErr) {
		handleAppError(c, appErr)
		return
	}
	handleInternalError(c, err)
}

func handleAppError(c *echo.Context, err *apperr.Error) {
	status := err.HttpStatus()

	response := ErrorResponse{
		Error: ErrorBody{
			Code:    string(err.Code),
			Message: err.Message,
		},
	}
	_ = c.JSON(status, response)
}

func handleInternalError(c *echo.Context, err error) {
	slog.Error(err.Error())

	_ = c.JSON(http.StatusInternalServerError, map[string]any{
		"error": map[string]any{
			"code":    apperr.CodeInternal,
			"message": "internal server error",
		},
	})
}
