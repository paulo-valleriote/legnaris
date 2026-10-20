package app

import (
	"fmt"
	"net/http"
	"time"

	"legnaris/internal/auth"
	"legnaris/internal/platform/clientip"
	"legnaris/internal/platform/ratelimit"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/template"
	"legnaris/internal/user"
	"legnaris/shared/db"
	"legnaris/shared/env"
)

type TemplateModule struct {
	Controller *TemplateController
	Service    template.Service
}

type SheetModule struct {
	Controller *SheetController
	Service    sheet.Service
}

type AuthModule struct {
	Controller   *AuthController
	Service      auth.Service
	TokenManager auth.TokenManager
	RateLimiter  *ratelimit.Limiter
}

type Modules struct {
	Template TemplateModule
	Sheet    SheetModule
	Auth     AuthModule
	// ClientIP is the boot-time address resolver installed as the router's
	// IPExtractor, so every c.RealIP() consumer reads the same resolution
	// (PRQ-03, PRQ-04).
	ClientIP func(*http.Request) string
}

// NewModules wires every domain module. It returns an error for configuration
// that must fail fast at boot (JWT secret, bcrypt cost) instead of producing a
// half-usable server.
func NewModules(database *db.Database, registry *template.Registry, cfg env.Env, observer auth.Observer) (*Modules, error) {
	log := cfg.Logger
	mdb := database.Client.Database(cfg.Variables.DatabaseName)

	// The trust boundary is configuration; an invalid CIDR must stop the boot
	// instead of silently trusting the wrong network (PRQ-03).
	clientIPPolicy, err := clientip.New(cfg.Variables.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("build client IP policy: %w", err)
	}

	templateService := template.NewService(registry)
	templateController := NewTemplateController(templateService)

	sheetRepository := sheet.NewRepository(mdb, log)
	sheetService := sheet.NewService(sheetRepository, templateService, registry)
	sheetController := NewSheetController(sheetService)

	userRepository := user.NewRepository(mdb, log)
	refreshTokenRepository := auth.NewRefreshTokenRepository(mdb)

	hasher, err := auth.NewPasswordHasher(cfg.Variables.AuthBcryptCost)
	if err != nil {
		return nil, fmt.Errorf("build password hasher: %w", err)
	}

	// One clock for every auth timestamp, so expiry boundaries are testable
	// without sleeping (PRQ-16).
	now := auth.Clock(time.Now)

	tokenManager, err := auth.NewTokenManager(cfg.Variables.JWTSecret, cfg.Variables.JWTAccessTTL, now)
	if err != nil {
		return nil, fmt.Errorf("build token manager: %w", err)
	}

	csrfManager := auth.NewCSRFTokenManager([]byte(cfg.Variables.JWTSecret))

	authService := auth.NewService(
		userRepository,
		refreshTokenRepository,
		hasher,
		tokenManager,
		auth.NewRefreshTokenGenerator(),
		csrfManager,
		observer,
		auth.Config{
			RefreshTTL: cfg.Variables.AuthRefreshTTL,
			Now:        now,
		},
		log,
	)
	authController := NewAuthController(authService)

	authLimiter := ratelimit.New(
		cfg.Variables.AuthRateLimitAttempts,
		cfg.Variables.AuthRateLimitWindow,
		time.Now,
	)

	return &Modules{
		Template: TemplateModule{
			Controller: templateController,
			Service:    templateService,
		},
		Sheet: SheetModule{
			Controller: sheetController,
			Service:    sheetService,
		},
		Auth: AuthModule{
			Controller:   authController,
			Service:      authService,
			TokenManager: tokenManager,
			RateLimiter:  authLimiter,
		},
		ClientIP: clientIPPolicy.Resolve,
	}, nil
}
