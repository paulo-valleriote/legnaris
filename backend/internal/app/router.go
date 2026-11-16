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
	router.IPExtractor = modules.ClientIP

	router.Use(middleware.RequestID())
	router.Use(cors.Middleware(deps.Variables.CORSOrigins))
	router.Use(httpMetrics.Middleware)
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())

	httpMetrics.Register(router)

	probes := router.Group("")
	health.RegisterHealthRoutes(probes)
	health.RegisterReadyRoute(probes, modules.Readiness, deps.Variables.DatabasePingTimeout)
	registerAuthRoutes(router.Group("/auth"), modules, deps.Variables.CORSOrigins, httpMetrics)

	registerCSPReportRoute(router, deps, modules.PublicLimiter, httpMetrics)

	requireAuth := auth.Middleware(modules.Auth.TokenManager)
	requireActive := auth.RejectDeleting(modules.activeAccounts())
	limitUser := userRateLimit(modules.UserLimiter)

	templatesGroup := router.Group("/templates", requireAuth, requireActive, limitUser)
	modules.Template.Controller.RegisterRoutes(templatesGroup)
	modules.Sheet.Controller.RegisterTemplatePreviewRoute(templatesGroup)

	sheetsGroup := router.Group("/sheets", requireAuth, requireActive, limitUser)
	modules.Sheet.Controller.RegisterRoutes(sheetsGroup)

	generationGroup := router.Group("/generations", requireAuth, requireActive)
	modules.Generation.Controller.RegisterRoutes(generationGroup, modules.Generation.CreateLimiter, modules.Generation.ReadLimiter)

	llmConsentGroup := router.Group("/me/llm-consent", requireAuth, requireActive, limitUser)
	modules.LLMConsent.Controller.RegisterRoutes(llmConsentGroup)
	meGroup := router.Group("/me", requireAuth, requireActive, limitUser)
	modules.Plan.RegisterRoutes(meGroup)

	billingGroup := router.Group("/billing", requireAuth, requireActive, limitUser)
	modules.Billing.Controller.RegisterRoutes(billingGroup)
	router.POST("/billing/webhook", modules.Billing.Webhook.Receive)

	return router
}

func (m *Modules) activeAccounts() auth.ActiveAccountChecker {
	if m.ActiveAccounts != nil {
		return m.ActiveAccounts
	}
	return m.Auth.Service
}

func registerAuthRoutes(group *echo.Group, modules *Modules, origins []string, observer Observer) {
	limiter := modules.Auth.RateLimiter
	challengeController := NewChallengeController(modules.Auth.Service, observer)

	group.POST("/register", modules.Auth.Controller.Register, registerRateLimit(modules.Auth.RegisterIPLimiter, modules.Auth.RegisterEmailLimiter, observer))
	group.POST("/login", modules.Auth.Controller.Login, rateLimit(limiter, observer, true))
	group.POST("/refresh", modules.Auth.Controller.Refresh, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))
	group.GET("/csrf", modules.Auth.Controller.CSRF, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))
	group.POST("/logout", modules.Auth.Controller.Logout, rateLimit(limiter, observer, false), requireAllowedOrigin(origins))

	group.POST("/challenge/verify", challengeController.Verify, rateLimit(limiter, observer, false))
	group.POST("/challenge/resend", challengeController.Resend, rateLimit(limiter, observer, false))

	group.GET("/me", modules.Auth.Controller.Me, auth.Middleware(modules.Auth.TokenManager), userRateLimit(modules.UserLimiter))

	group.POST("/me/deletion", modules.Auth.Controller.RequestAccountDeletion, auth.Middleware(modules.Auth.TokenManager), auth.RejectDeleting(modules.activeAccounts()), userRateLimit(modules.UserLimiter))
	group.DELETE("/me", modules.Auth.Controller.DeleteAccount, auth.Middleware(modules.Auth.TokenManager), userRateLimit(modules.UserLimiter))
}

func HTTPErrorHandler(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	appErr := asAppErr(err)

	if appErr.RetryAfter > 0 {
		c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(retryAfterSeconds(appErr.RetryAfter)))
	}

	_ = c.JSON(appErr.HttpStatus(), response.ErrorResponse{
		Error: response.ErrorBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}

func registerCSPReportRoute(router *echo.Echo, deps env.Env, limiter ratelimit.Allower, observer *metrics.Metrics) {
	controller := NewCSPReportController(deps.Logger, observer)

	router.POST("/csp-report", controller.Report, cspRateLimit(limiter))
}
