package app

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/email/disposable"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/internal/platform/clientip"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
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
	RateLimiter  ratelimit.Allower
	// RegisterIPLimiter and RegisterEmailLimiter are the registration budgets
	// per real address and per canonical e-mail (LGN-150).
	RegisterIPLimiter    ratelimit.Allower
	RegisterEmailLimiter ratelimit.Allower
}

type Modules struct {
	Template TemplateModule
	Sheet    SheetModule
	Auth     AuthModule
	// ClientIP is the boot-time address resolver installed as the router's
	// IPExtractor, so every c.RealIP() consumer reads the same resolution
	// (PRQ-03, PRQ-04).
	ClientIP func(*http.Request) string
	// UserLimiter caps the authenticated routes per user id (LGN-30).
	UserLimiter ratelimit.Allower
	// PublicLimiter caps the unauthenticated CSP report collector per address.
	PublicLimiter ratelimit.Allower
	// Readiness backs GET /ready: the instance is ready only while it answers.
	Readiness health.Pinger
	// closers release resources owned by the modules (the Redis client).
	closers []io.Closer
}

// Close releases the resources the modules opened. It is safe to call on a
// deployment without Redis.
func (m *Modules) Close() error {
	var errs []error
	for _, closer := range m.closers {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}

// authServiceConfig maps the resolved environment onto the auth service's
// configuration. The rollout gate is carried here, so the service — and not
// the controller — is the single place that decides whether a session or a code
// step is issued (TFA-01, TFA-02, TFA-07).
func authServiceConfig(vars env.Variables, now auth.Clock) auth.Config {
	cfg := auth.Config{
		RefreshTTL: vars.AuthRefreshTTL,
		Now:        now,
	}
	if vars.AuthDisposableEmailCheck {
		cfg.EmailChecker = disposable.NewChecker(nil)
	}
	return cfg
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
	sheetService := sheet.NewService(sheetRepository, templateService, registry, log)
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
		ResendCapRatio:   cfg.Variables.AuthOTPResendCapRatio,
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
		authServiceConfig(cfg.Variables, now),
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

	limiterClient, closers, err := newLimiterClient(cfg.Variables.RedisURL)
	if err != nil {
		return nil, err
	}
	limiter := func(namespace string, attempts int, window time.Duration) ratelimit.Allower {
		report := storeFailureReporter(log, namespace, limiterFailureLogInterval, time.Now)
		return newLimiter(limiterClient, namespace, attempts, window, report)
	}

	authLimiter := limiter("auth", cfg.Variables.AuthRateLimitAttempts, cfg.Variables.AuthRateLimitWindow)
	userLimiter := limiter("user", cfg.Variables.UserRateLimitAttempts, cfg.Variables.UserRateLimitWindow)
	publicLimiter := limiter("public", cfg.Variables.CSPReportRateLimitAttempts, cfg.Variables.CSPReportRateLimitWindow)
	registerIPLimiter := limiter("register_ip", cfg.Variables.RegisterIPRateLimitAttempts, cfg.Variables.RegisterIPRateLimitWindow)
	registerEmailLimiter := limiter("register_email", cfg.Variables.RegisterEmailRateLimitAttempts, cfg.Variables.RegisterEmailRateLimitWindow)

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

			RegisterIPLimiter:    registerIPLimiter,
			RegisterEmailLimiter: registerEmailLimiter,
		},
		ClientIP:      clientIPPolicy.Resolve,
		UserLimiter:   userLimiter,
		PublicLimiter: publicLimiter,
		Readiness:     database,
		closers:       closers,
	}, nil
}
