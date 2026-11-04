package app

import (
	"strconv"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/platform/cors"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/response"

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

	// The probes are deliberately outside every rate limiter: an orchestrator
	// or scraper that gets a 429 would restart or evict a healthy instance.
	httpMetrics.Register(router)

	probes := router.Group("")
	health.RegisterHealthRoutes(probes)
	health.RegisterReadyRoute(probes, modules.Readiness, deps.Variables.DatabasePingTimeout)
	registerAuthRoutes(router.Group("/auth"), modules, deps.Variables.CORSOrigins, httpMetrics)

	// The CSP report collector is public: a violation report carries no
	// credential and the browser sends it without one (POL-04).
	registerCSPReportRoute(router, deps, modules.PublicLimiter, httpMetrics)

	// Every sheets and templates route sits behind the JWT middleware; only
	// health, metrics, and the auth endpoints stay public (OWN-15…OWN-18).
	requireAuth := auth.Middleware(modules.Auth.TokenManager)
	limitUser := userRateLimit(modules.UserLimiter)

	templatesGroup := router.Group("/templates", requireAuth, limitUser)
	modules.Template.Controller.RegisterRoutes(templatesGroup)
	modules.Sheet.Controller.RegisterTemplatePreviewRoute(templatesGroup)

	sheetsGroup := router.Group("/sheets", requireAuth, limitUser)
	modules.Sheet.Controller.RegisterRoutes(sheetsGroup)

	return router
}

// registerAuthRoutes mounts the auth endpoints with their per-route guards.
// Login is rate limited per address and, because the body carries an account
// identity, per account; registration has its own hourly per-address and
// per-e-mail budgets (LGN-150); refresh and logout are rate limited
// per address only and refuse a request from an origin outside the allowlist
// before any credential work (AUTH-13, AUTH-17, AUTH-26).
func registerAuthRoutes(group *echo.Group, modules *Modules, origins []string, observer Observer) {
	limiter := modules.Auth.RateLimiter
	challengeController := NewChallengeController(modules.Auth.Service, observer)

	group.POST("/register", modules.Auth.Controller.Register, registerRateLimit(modules.Auth.RegisterIPLimiter, modules.Auth.RegisterEmailLimiter, observer))
	group.POST("/login", modules.Auth.Controller.Login, rateLimit(limiter, observer, true))
	group.POST("/refresh", modules.Auth.Controller.Refresh, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))
	group.POST("/logout", modules.Auth.Controller.Logout, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))

	// The two unified code-step routes present a capability and no session
	// credential, so they take the per-address limiter and no origin guard —
	// exactly like sign-in and registration (OTP-19, R13).
	group.POST("/challenge/verify", challengeController.Verify, rateLimit(limiter, observer, false))
	group.POST("/challenge/resend", challengeController.Resend, rateLimit(limiter, observer, false))

	// The identity read is the only route that needs an access credential: it
	// answers the signed-in account and its verification state, and it reuses
	// the one bearer parser the protected groups use (TFA-08, TFA-09).
	group.GET("/me", modules.Auth.Controller.Me, auth.Middleware(modules.Auth.TokenManager), userRateLimit(modules.UserLimiter))
}

func HTTPErrorHandler(c *echo.Context, err error) {
	// The RequestLogger middleware (HandleError=true) already invoked the global
	// error handler; Echo's ServeHTTP calls it again for the bubbled-up error.
	// If the response is already committed, do nothing to avoid a double write.
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	appErr := asAppErr(err)

	// A refused request that carries a wait hint advertises it, so the client
	// can render a countdown (OTP-13, OTP-14).
	if appErr.RetryAfter > 0 {
		c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds(appErr.RetryAfter)))
	}

	_ = c.JSON(appErr.HttpStatus(), response.ErrorResponse{
		Error: response.ErrorBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
		},
	})
}

// registerCSPReportRoute mounts the public CSP violation collector behind its
// own address limiter (POL-04, POL-05, A7).
func registerCSPReportRoute(router *echo.Echo, deps env.Env, limiter ratelimit.Allower, observer *metrics.Metrics) {
	controller := NewCSPReportController(deps.Logger, observer)

	router.POST("/csp-report", controller.Report, cspRateLimit(limiter))
}
