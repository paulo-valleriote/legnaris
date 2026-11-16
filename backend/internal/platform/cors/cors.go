package cors

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const csrfHeader = "X-CSRF-Token"

func Middleware(origins []string) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     slices.Clone(origins),
		AllowCredentials: true,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
			csrfHeader,
		},
		ExposeHeaders: []string{echo.HeaderXRequestID},
	})
}
