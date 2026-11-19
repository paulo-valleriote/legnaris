package app

import (
	"net/http"
	"strconv"
	"strings"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/platform/cors"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const requestIDContextKey = "request_id"

func NewRouter(deps env.Env, modules *Modules, httpMetrics *metrics.Metrics) *echo.Echo {
	router := echo.NewWithConfig(echo.Config{
		Router: echo.NewRouter(echo.RouterConfig{
			MethodNotAllowedHandler: methodNotAllowedHandler,
		}),
	})
	router.HTTPErrorHandler = HTTPErrorHandler
	router.IPExtractor = modules.ClientIP

	router.Use(serverRequestID())
	router.Use(maskAdminOptions)
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

	campaignsGroup := router.Group("/campaigns", requireAuth, requireActive, limitUser)
	modules.Campaign.Controller.RegisterRoutes(campaignsGroup, rateLimit(modules.Campaign.JoinIPLimiter, nil, httpMetrics))

	generationGroup := router.Group("/generations", requireAuth, requireActive)
	modules.Generation.Controller.RegisterRoutes(generationGroup, modules.Generation.CreateLimiter, modules.Generation.ReadLimiter)

	llmConsentGroup := router.Group("/me/llm-consent", requireAuth, requireActive, limitUser)
	modules.LLMConsent.Controller.RegisterRoutes(llmConsentGroup)
	meGroup := router.Group("/me", requireAuth, requireActive, limitUser)
	modules.Plan.RegisterRoutes(meGroup)

	billingGroup := router.Group("/billing", requireAuth, requireActive, limitUser)
	modules.Billing.Controller.RegisterRoutes(billingGroup)
	router.POST("/billing/webhook", modules.Billing.Webhook.Receive)

	adminGroup := router.Group("/admin",
		auth.AdminOnly(modules.Auth.TokenManager, deps.Variables.AdminUserIDs),
		auth.AdminActive(modules.activeAccounts()),
		limitUser,
	)
	modules.Admin.Controller.RegisterRoutes(adminGroup)

	return router
}

func methodNotAllowedHandler(c *echo.Context) error {
	path := c.Request().URL.Path
	if isAdminPath(path) {
		return apperr.NewError(apperr.CodeNotFound, auth.MsgNotFound, nil)
	}
	if allow, ok := c.Get(echo.ContextKeyHeaderAllow).(string); ok && allow != "" {
		c.Response().Header().Set(echo.HeaderAllow, allow)
	}
	return echo.ErrMethodNotAllowed
}

func serverRequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			id, err := idgen.New()
			if err != nil {
				return apperr.NewError(apperr.CodeInternal, "internal server error", err)
			}
			c.Set(requestIDContextKey, id)
			c.Response().Header().Set(echo.HeaderXRequestID, id)
			return next(c)
		}
	}
}

func requestIDOf(c *echo.Context) string {
	id, _ := c.Get(requestIDContextKey).(string)
	return id
}

func maskAdminOptions(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if c.Request().Method == http.MethodOptions && isAdminPath(c.Request().URL.Path) {
			return apperr.NewError(apperr.CodeNotFound, auth.MsgNotFound, nil)
		}
		return next(c)
	}
}

func isAdminPath(path string) bool {
	return path == "/admin" || strings.HasPrefix(path, "/admin/")
}

func (m *Modules) activeAccounts() auth.ActiveAccountChecker {
	if m.ActiveAccounts != nil {
		return m.ActiveAccounts
	}
	return m.Auth.Service
}

func registerAuthRoutes(group *echo.Group, modules *Modules, origins []string, observer Observer) {
	authLimiter := modules.Auth.RateLimiter
	sessionLimiter := modules.Auth.SessionLimiter
	challengeController := NewChallengeController(modules.Auth.Service, observer)

	group.POST("/register", modules.Auth.Controller.Register, registerRateLimit(modules.Auth.RegisterIPLimiter, modules.Auth.RegisterEmailLimiter, observer))
	group.POST("/login", modules.Auth.Controller.Login, rateLimit(authLimiter, modules.Auth.AccountLimiter, observer))
	group.POST("/refresh", modules.Auth.Controller.Refresh, rateLimit(sessionLimiter, nil, observer), requireAllowedOrigin(origins))
	group.GET("/csrf", modules.Auth.Controller.CSRF, rateLimit(sessionLimiter, nil, observer), requireAllowedOrigin(origins))
	group.POST("/logout", modules.Auth.Controller.Logout, rateLimit(sessionLimiter, nil, observer), requireAllowedOrigin(origins))

	group.POST("/challenge/verify", challengeController.Verify, rateLimit(authLimiter, nil, observer))
	group.POST("/challenge/resend", challengeController.Resend, rateLimit(authLimiter, nil, observer))

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
