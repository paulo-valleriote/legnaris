package response

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type Success[T any] struct {
	Data T `json:"data"`
}

type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON[T any](c *echo.Context, status int, data T) error {
	return c.JSON(status, Success[T]{
		Data: data,
	})
}

func OK[T any](c *echo.Context, data T) error {
	return JSON(c, http.StatusOK, data)
}

func Created[T any](c *echo.Context, data T) error {
	return JSON(c, http.StatusCreated, data)
}
