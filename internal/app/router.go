package app

import (
	"errors"
	"legnaris/internal/auth"
	"legnaris/internal/platform/cors"
	"legnaris/internal/platform/health"
	"legnaris/internal/platform/metrics"
	"legnaris/shared/apperr"
	"legnaris/shared/env"
	"legnaris/shared/response"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func NewRouter(deps env.Env, modules *Modules, httpMetrics *metrics.Metrics) *echo.Echo {
	router := echo.New()
	router.HTTPErrorHandler = HTTPErrorHandler

	router.Use(middleware.RequestID())
	router.Use(cors.Middleware(deps.Variables.CORSOrigins))
	router.Use(httpMetrics.Middleware)
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	httpMetrics.Register(router)

	health.RegisterHealthRoutes(router.Group(""))
	modules.Auth.Controller.RegisterRoutes(router.Group("/auth"))

	// Every sheets and templates route sits behind the JWT middleware; only
	// health, metrics, and the auth endpoints stay public (OWN-15…OWN-18).
	requireAuth := auth.Middleware(modules.Auth.TokenManager)

	templatesGroup := router.Group("/templates", requireAuth)
	modules.Template.Controller.RegisterRoutes(templatesGroup)
	modules.Sheet.Controller.RegisterTemplatePreviewRoute(templatesGroup)

	sheetsGroup := router.Group("/sheets", requireAuth)
	modules.Sheet.Controller.RegisterRoutes(sheetsGroup)

	return router
}

func HTTPErrorHandler(c *echo.Context, err error) {
	// The RequestLogger middleware (HandleError=true) already invoked the global
	// error handler; Echo's ServeHTTP calls it again for the bubbled-up error.
	// If the response is already committed, do nothing to avoid a double write.
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	var appErr *apperr.Error

	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.HttpStatus(), response.ErrorResponse{
			Error: response.ErrorBody{
				Code:    string(appErr.Code),
				Message: appErr.Message,
			},
		})
		return
	}

	_ = c.JSON(http.StatusInternalServerError, response.ErrorResponse{
		Error: response.ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		},
	})
}
