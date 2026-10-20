package app

import (
	"fmt"
	"net/http"
	"time"

	"legnaris/internal/auth"
	"legnaris/internal/email/outbox"
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
// that must fail fast at boot (JWT secret, bcrypt cost, the code-digest secret)
// instead of producing a half-usable server.
func NewModules(
	database *db.Database,
	registry *template.Registry,
	cfg env.Env,
	observer auth.Observer,
	challengeObserver auth.ChallengeObserver,
) (*Modules, error) {
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

	// The code-digest secret is required and validated here, so the process
	// never starts hashing codes with a weak or absent key (F-09, OTP-18).
	codeDigest, err := auth.NewCodeDigest(cfg.Variables.EmailCodeSecret)
	if err != nil {
		return nil, fmt.Errorf("build code digest: %w", err)
	}

	challengeRepository := auth.NewChallengeRepository(mdb)
	codeMailer := NewEmailIntent(outbox.NewRepository(mdb, log), log)
	challengePolicy := auth.ChallengePolicy{
		Lifetime:         cfg.Variables.AuthOTPTTL,
		Ceiling:          cfg.Variables.AuthOTPCeiling,
		ResendCooldown:   cfg.Variables.AuthOTPResendCooldown,
		MaxAttempts:      cfg.Variables.AuthOTPMaxAttempts,
		DailySendCap:     cfg.Variables.AuthOTPDailySendCap,
		UserWindow:       cfg.Variables.AuthOTPUserWindow,
		MaxPerUserWindow: cfg.Variables.AuthOTPUserLimit,
	}

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
		auth.ChallengeDeps{
			Repository: challengeRepository,
			Generator:  auth.NewCodeGenerator(),
			Digest:     codeDigest,
			Mailer:     codeMailer,
			Observer:   challengeObserver,
			Policy:     challengePolicy,
		},
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
