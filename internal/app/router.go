package app

import (
	"time"

	"legnaris/internal/auth"
	"legnaris/internal/platform/cors"
	"legnaris/internal/platform/health"
	"legnaris/internal/platform/metrics"
	"legnaris/internal/platform/ratelimit"
	"legnaris/shared/env"
	"legnaris/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func NewRouter(deps env.Env, modules *Modules, httpMetrics *metrics.Metrics) *echo.Echo {
	router := echo.New()
	router.HTTPErrorHandler = HTTPErrorHandler
	// Installed before any route or middleware so every c.RealIP() consumer —
	// today the auth and CSP limiters — reads the resolved client, not the
	// proxy's shared address (PRQ-04).
	router.IPExtractor = modules.ClientIP

	router.Use(middleware.RequestID())
	router.Use(cors.Middleware(deps.Variables.CORSOrigins))
	router.Use(httpMetrics.Middleware)
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	httpMetrics.Register(router)

	health.RegisterHealthRoutes(router.Group(""))
	registerAuthRoutes(router.Group("/auth"), modules, deps.Variables.CORSOrigins, httpMetrics)

	// The CSP report collector is public: a violation report carries no
	// credential and the browser sends it without one (POL-04).
	registerCSPReportRoute(router, deps, httpMetrics)

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

// registerAuthRoutes mounts the auth endpoints with their per-route guards.
// Login and registration are rate limited per address and, because the body
// carries an account identity, per account; refresh is rate limited per address
// only and, like logout, refuses a request from an origin outside the allowlist
// before any credential work (AUTH-13, AUTH-17, AUTH-26).
func registerAuthRoutes(group *echo.Group, modules *Modules, origins []string, observer Observer) {
	limiter := modules.Auth.RateLimiter

	group.POST("/register", modules.Auth.Controller.Register, rateLimit(limiter, observer, true))
	group.POST("/login", modules.Auth.Controller.Login, rateLimit(limiter, observer, true))
	group.POST("/refresh", modules.Auth.Controller.Refresh, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))
	group.POST("/logout", modules.Auth.Controller.Logout, requireAllowedOrigin(origins))
}

func HTTPErrorHandler(c *echo.Context, err error) {
	// The RequestLogger middleware (HandleError=true) already invoked the global
	// error handler; Echo's ServeHTTP calls it again for the bubbled-up error.
	// If the response is already committed, do nothing to avoid a double write.
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	appErr := asAppErr(err)

	_ = c.JSON(appErr.HttpStatus(), response.ErrorResponse{
		Error: response.ErrorBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
		},
	})
}

// registerCSPReportRoute mounts the public CSP violation collector behind its
// own address limiter (POL-04, POL-05, A7).
func registerCSPReportRoute(router *echo.Echo, deps env.Env, observer *metrics.Metrics) {
	controller := NewCSPReportController(deps.Logger, observer)
	limiter := ratelimit.New(
		deps.Variables.CSPReportRateLimitAttempts,
		deps.Variables.CSPReportRateLimitWindow,
		time.Now,
	)

	router.POST("/csp-report", controller.Report, cspRateLimit(limiter))
}
