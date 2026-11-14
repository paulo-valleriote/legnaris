package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/email/disposable"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/platform/clientip"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/internal/trial"
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

type LLMConsentModule struct {
	Controller *LLMConsentController
	Service    llmconsent.Service
}

// GenerationModule is the character generation flow: the routes and the service
// that gates, queues and accepts generations, with the per-user budgets of its
// two route groups (LGN-186).
type GenerationModule struct {
	Controller    *GenerationController
	Service       *generation.Service
	CreateLimiter ratelimit.Allower
	ReadLimiter   ratelimit.Allower
}

// moduleOptions carries the collaborators that only the process knows, such as the
// broker connection and the metrics registry.
type moduleOptions struct {
	generationPublisher generation.Publisher
	capObserver         generation.CapObserver
}

// ModuleOption configures the optional collaborators of NewModules.
type ModuleOption func(*moduleOptions)

// WithGenerationPublisher supplies the broker port that publishes generation jobs.
// Without it the jobs are not published on request, and the republish sweep is the
// only path that delivers them.
func WithGenerationPublisher(publisher generation.Publisher) ModuleOption {
	return func(o *moduleOptions) {
		o.generationPublisher = publisher
	}
}

// WithGenerationObserver supplies the sink of the global daily cap metric.
func WithGenerationObserver(observer generation.CapObserver) ModuleOption {
	return func(o *moduleOptions) {
		o.capObserver = observer
	}
}

// unconfiguredPublisher refuses every publish, so a process built without a broker
// keeps answering 202 and leaves the job to the republish sweep.
type unconfiguredPublisher struct{}

// ErrPublisherNotConfigured reports a publish attempted without a broker port.
var ErrPublisherNotConfigured = errors.New("generation publisher is not configured")

func (unconfiguredPublisher) Publish(context.Context, generation.Message) error {
	return ErrPublisherNotConfigured
}

// noopCapObserver discards the cap metric when the process supplies none.
type noopCapObserver struct{}

func (noopCapObserver) GenerationCapUsage(int, int) {}

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
	Template   TemplateModule
	Sheet      SheetModule
	LLMConsent LLMConsentModule
	Auth       AuthModule
	Generation GenerationModule
	// Plan serves GET /me/plan: the caller's generation allowance (ADR 0009) and
	// per-kind Sheet usage (LGN-183).
	Plan *PlanController
	// ClientIP is the boot-time address resolver installed as the router's
	// IPExtractor, so every c.RealIP() consumer reads the same resolution
	// (PRQ-03, PRQ-04).
	ClientIP func(*http.Request) string
	// ActiveAccounts gates every authenticated route on the account not being
	// deleted (LGN-190). Nil means the auth service answers the check; a test
	// that has no database sets a checker of its own before building the router.
	ActiveAccounts auth.ActiveAccountChecker
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
		RefreshTTL:          vars.AuthRefreshTTL,
		Now:                 now,
		RegisterMinDuration: vars.AuthRegisterMinDuration,
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
	publisher events.Publisher,
	opts ...ModuleOption,
) (*Modules, error) {
	var options moduleOptions
	for _, opt := range opts {
		opt(&options)
	}
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

	userRepository := user.NewRepository(mdb, log)

	// Sheet limits read the owner's plan from the user store and reserve their
	// slot inside a transaction with the Sheet write (LGN-183, ADR 0013).
	sheetRepository := sheet.NewRepository(mdb, log)
	sheetService := sheet.NewService(sheetRepository, templateService, registry, log,
		sheet.WithPlanLimits(user.NewPlanSource(userRepository), db.NewTransactor(database.Client, nil)),
	)
	sheetController := NewSheetController(sheetService)
	planController := NewPlanController(sheetService)

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

	quotaStore := quota.NewMongoStore(mdb)

	limiterClient, closers, err := newLimiterClient(cfg.Variables.RedisURL)
	if err != nil {
		return nil, err
	}
	limiter := func(namespace string, attempts int, window time.Duration) ratelimit.Allower {
		report := storeFailureReporter(log, namespace, limiterFailureLogInterval, time.Now)
		return newLimiter(limiterClient, namespace, attempts, window, report)
	}

	emailVerifier := newTrialVerifier(
		quota.NewTrialGranter(quotaStore),
		disposable.NewChecker(nil),
		limiter("trial_ip", trial.MaxAccountsPerIP, trial.IPWindow),
		log,
	)

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
			Repository:    challengeRepository,
			Generator:     auth.NewCodeGenerator(),
			Digest:        codeDigest,
			Mailer:        codeMailer,
			Notices:       auth.NewNoticeRepository(mdb),
			Observer:      challengeObserver,
			Policy:        challengePolicy,
			EmailVerifier: emailVerifier,
		},
		publisher,
	)
	authController := NewAuthController(authService)

	quotaService := quota.NewService(quotaStore, nil)
	llmConsentService := llmconsent.NewService(
		llmconsent.NewConsentStore(mdb),
		llmconsent.NewGenerationStore(mdb),
		generationQuotaRefunder{quota: quotaService},
		db.NewTransactor(database.Client, nil),
		time.Now,
	)
	generationService := newGenerationService(cfg, options, generation.Deps{
		Store:     generation.NewMongoStore(mdb),
		Consent:   llmConsentService,
		Sheets:    sheetService,
		Plans:     user.NewPlanSource(userRepository),
		Templates: templateService,
		Quota:     quotaService,
		Tx:        db.NewTransactor(database.Client, nil),
		Logger:    log,
	})
	planController := NewPlanController(quotaService, FreePlanResolver,
		WithSheetUsage(sheetService),
		WithGenerationStatus(llmConsentService, generationService),
	)

	authLimiter := limiter("auth", cfg.Variables.AuthRateLimitAttempts, cfg.Variables.AuthRateLimitWindow)
	userLimiter := limiter("user", cfg.Variables.UserRateLimitAttempts, cfg.Variables.UserRateLimitWindow)
	generationCreateLimiter := limiter("generation_create", cfg.Variables.GenerationCreateRateLimitAttempts, cfg.Variables.GenerationCreateRateLimitWindow)
	generationReadLimiter := limiter("generation_read", cfg.Variables.GenerationReadRateLimitAttempts, cfg.Variables.GenerationReadRateLimitWindow)
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
		LLMConsent: LLMConsentModule{
			Controller: NewLLMConsentController(llmConsentService),
			Service:    llmConsentService,
		},
		Auth: AuthModule{
			Controller:   authController,
			Service:      authService,
			TokenManager: tokenManager,
			RateLimiter:  authLimiter,

			RegisterIPLimiter:    registerIPLimiter,
			RegisterEmailLimiter: registerEmailLimiter,
		},
		Generation: GenerationModule{
			Controller:    NewGenerationController(generationService),
			Service:       generationService,
			CreateLimiter: generationCreateLimiter,
			ReadLimiter:   generationReadLimiter,
		},
		Plan:          planController,
		ClientIP:      clientIPPolicy.Resolve,
		UserLimiter:   userLimiter,
		PublicLimiter: publicLimiter,
		Readiness:     database,
		closers:       closers,
	}, nil
}

// newGenerationService builds the generation service from the process
// configuration and the module collaborators.
func newGenerationService(cfg env.Env, options moduleOptions, deps generation.Deps) *generation.Service {
	deps.Publisher = options.generationPublisher
	if deps.Publisher == nil {
		deps.Publisher = unconfiguredPublisher{}
	}
	deps.Observer = options.capObserver
	if deps.Observer == nil {
		deps.Observer = noopCapObserver{}
	}
	return generation.NewService(generation.Config{
		Enabled:  cfg.Variables.GenerationEnabled,
		DailyCap: cfg.Variables.GenerationDailyCap,
	}, deps, time.Now)
}
