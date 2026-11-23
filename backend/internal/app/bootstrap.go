package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/billing"
	billingstripe "legnaris/backend/internal/billing/stripe"
	"legnaris/backend/internal/campaign"
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

type BillingModule struct {
	Controller *BillingController
	Service    *billing.Service
	Webhook    *BillingWebhookController
}

type LLMConsentModule struct {
	Controller *LLMConsentController
	Service    llmconsent.Service
}

type GenerationModule struct {
	Controller    *GenerationController
	Service       *generation.Service
	CreateLimiter ratelimit.Allower
	ReadLimiter   ratelimit.Allower
}

type moduleOptions struct {
	generationPublisher generation.Publisher
	capObserver         generation.CapObserver
}

type ModuleOption func(*moduleOptions)

func WithGenerationPublisher(publisher generation.Publisher) ModuleOption {
	return func(o *moduleOptions) {
		o.generationPublisher = publisher
	}
}

func WithGenerationObserver(observer generation.CapObserver) ModuleOption {
	return func(o *moduleOptions) {
		o.capObserver = observer
	}
}

type unconfiguredPublisher struct{}

func (unconfiguredPublisher) Publish(context.Context, generation.Message) error {
	return generation.ErrPublisherNotConfigured
}

type noopCapObserver struct{}

func (noopCapObserver) GenerationCapUsage(int, int) {}

type AuthModule struct {
	Controller           *AuthController
	Service              auth.Service
	TokenManager         auth.TokenManager
	RateLimiter          ratelimit.Allower
	SessionLimiter       ratelimit.Allower
	AccountLimiter       ratelimit.Allower
	RegisterIPLimiter    ratelimit.Allower
	RegisterEmailLimiter ratelimit.Allower
}

type Modules struct {
	Template       TemplateModule
	Sheet          SheetModule
	Campaign       CampaignModule
	LLMConsent     LLMConsentModule
	Auth           AuthModule
	Generation     GenerationModule
	Billing        BillingModule
	Plan           *PlanController
	ClientIP       func(*http.Request) string
	ActiveAccounts auth.ActiveAccountChecker
	UserLimiter    ratelimit.Allower
	PublicLimiter  ratelimit.Allower
	Readiness      health.Pinger
	closers        []io.Closer
}

func (m *Modules) Close() error {
	var errs []error
	for _, closer := range m.closers {
		errs = append(errs, closer.Close())
	}
	return errors.Join(errs...)
}

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

	clientIPPolicy, err := clientip.New(cfg.Variables.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("build client IP policy: %w", err)
	}

	templateService := template.NewService(registry)
	templateController := NewTemplateController(templateService)

	userRepository := user.NewCachedRepository(user.NewRepository(mdb, log))

	sheetRepository := sheet.NewRepository(mdb, log)
	linkStore := campaign.NewLinkStore(mdb)
	scopeGate := newCampaignScopeGate()
	sheetService := sheet.NewService(sheetRepository, templateService, registry, log, publisher,
		sheet.WithPlanLimits(user.NewPlanSource(userRepository), db.NewTransactor(database.Client, nil)),
		sheet.WithLinkCleaner(campaign.NewSheetLinkCleaner(linkStore, publisher)),
		sheetDraftTemplateOption(registry),
		sheet.WithCampaignScope(scopeGate),
	)
	sheetController := NewSheetController(sheetService)

	ownedCampaigns := campaign.NewOwnershipLimiter(user.NewPlanSource(userRepository), campaign.NewOwnershipCounter(mdb))
	refreshTokenRepository := auth.NewRefreshTokenRepository(mdb)

	hasher, err := auth.NewPasswordHasher(cfg.Variables.AuthBcryptCost)
	if err != nil {
		return nil, fmt.Errorf("build password hasher: %w", err)
	}

	now := auth.Clock(time.Now)

	tokenManager, err := auth.NewTokenManager(cfg.Variables.JWTSecret, cfg.Variables.JWTAccessTTL, now)
	if err != nil {
		return nil, fmt.Errorf("build token manager: %w", err)
	}

	csrfManager := auth.NewCSRFTokenManager([]byte(cfg.Variables.JWTSecret))

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
			Notices:    auth.NewNoticeRepository(mdb),
			Observer:   challengeObserver,
			Policy:     challengePolicy,
		},
		publisher,
	)
	authController := NewAuthController(authService)

	quotaService := quota.NewService(quotaStore, nil)
	llmConsentService := llmconsent.NewService(
		llmconsent.NewAccountConsentStore(userRepository, llmconsent.NewConsentStore(mdb)),
		user.NewPlanSource(userRepository),
		llmconsent.NewGenerationStore(mdb),
		generationQuotaRefunder{quota: quotaService},
		db.NewTransactor(database.Client, nil),
		time.Now,
	)
	generationService := newGenerationService(cfg, registry, options, generation.Deps{
		Store:     generation.NewMongoStore(mdb),
		Consent:   llmConsentService,
		Sheets:    sheetService,
		Plans:     user.NewPlanSource(userRepository),
		Templates: templateService,
		Quota:     quotaService,
		Tx:        db.NewTransactor(database.Client, nil),
		Logger:    log,
	})
	billingProvider, err := newBillingProvider(cfg.Variables)
	if err != nil {
		return nil, fmt.Errorf("build billing provider: %w", err)
	}
	billingStore := billing.NewMongoStore(mdb)
	billingService := newBillingService(billingProvider, billingStore, cfg.Variables)
	billingInbox := billing.NewInbox(billingProvider, billingStore, time.Now)
	planController := NewPlanController(quotaService, PlanResolverFromSource(user.NewPlanSource(userRepository)),
		WithSheetUsage(sheetService),
		WithOwnedCampaigns(ownedCampaigns),
		WithBillingSummary(billing.NewStatusReader(billingStore)),
		WithGenerationStatus(llmConsentService, generationService),
	)

	authLimiter := limiter("auth", cfg.Variables.AuthRateLimitAttempts, cfg.Variables.AuthRateLimitWindow)
	sessionLimiter := limiter("session", cfg.Variables.SessionRateLimitAttempts, cfg.Variables.SessionRateLimitWindow)
	accountLimiter := limiter("auth_account", cfg.Variables.AuthAccountRateLimitAttempts, cfg.Variables.AuthAccountRateLimitWindow)
	userLimiter := limiter("user", cfg.Variables.UserRateLimitAttempts, cfg.Variables.UserRateLimitWindow)
	generationCreateLimiter := limiter("generation_create", cfg.Variables.GenerationCreateRateLimitAttempts, cfg.Variables.GenerationCreateRateLimitWindow)
	generationReadLimiter := limiter("generation_read", cfg.Variables.GenerationReadRateLimitAttempts, cfg.Variables.GenerationReadRateLimitWindow)
	publicLimiter := limiter("public", cfg.Variables.CSPReportRateLimitAttempts, cfg.Variables.CSPReportRateLimitWindow)
	registerIPLimiter := limiter("register_ip", cfg.Variables.RegisterIPRateLimitAttempts, cfg.Variables.RegisterIPRateLimitWindow)
	registerEmailLimiter := limiter("register_email", cfg.Variables.RegisterEmailRateLimitAttempts, cfg.Variables.RegisterEmailRateLimitWindow)
	campaignHasher, err := campaign.NewArgon2Hasher()
	if err != nil {
		return nil, fmt.Errorf("init campaign password hasher: %w", err)
	}
	campaignJoinIPLimiter := limiter("campaign_join_ip", cfg.Variables.CampaignJoinIPRateLimitAttempts, cfg.Variables.CampaignJoinIPRateLimitWindow)
	campaignJoinLimiter := limiter("campaign_join_campaign", cfg.Variables.CampaignJoinRateLimitAttempts, cfg.Variables.CampaignJoinRateLimitWindow)
	campaignService := campaign.NewService(campaign.NewRepository(mdb), db.NewTransactor(database.Client, nil), publisher,
		campaign.WithNameLookup(campaignNameLookup{users: userRepository}),
		campaign.WithCreateGuard(ownedCampaigns),
		campaign.WithOwnershipReleaser(ownedCampaigns),
		campaign.WithCharacterStore(sheetRepository),
		campaign.WithCharacterUnlinker(campaignCharacterUnlinker{sheets: sheetRepository}),
		campaign.WithLinkStore(linkStore),
		campaign.WithPasswordHasher(campaignHasher),
		campaign.WithJoinLimiter(campaignJoinLimiter),
		campaign.WithScopeResolver(newCampaignScopeResolver(registry)),
	)
	scopeGate.bind(campaignService)
	campaignController := NewCampaignController(campaignService)

	return &Modules{
		Template: TemplateModule{
			Controller: templateController,
			Service:    templateService,
		},
		Sheet: SheetModule{
			Controller: sheetController,
			Service:    sheetService,
		},
		Campaign: CampaignModule{
			Controller:    campaignController,
			Service:       campaignService,
			JoinIPLimiter: campaignJoinIPLimiter,
		},
		LLMConsent: LLMConsentModule{
			Controller: NewLLMConsentController(llmConsentService),
			Service:    llmConsentService,
		},
		Auth: AuthModule{
			Controller:     authController,
			Service:        authService,
			TokenManager:   tokenManager,
			RateLimiter:    authLimiter,
			SessionLimiter: sessionLimiter,
			AccountLimiter: accountLimiter,

			RegisterIPLimiter:    registerIPLimiter,
			RegisterEmailLimiter: registerEmailLimiter,
		},
		Generation: GenerationModule{
			Controller:    NewGenerationController(generationService),
			Service:       generationService,
			CreateLimiter: generationCreateLimiter,
			ReadLimiter:   generationReadLimiter,
		},
		Billing: BillingModule{
			Controller: NewBillingController(billingService),
			Service:    billingService,
			Webhook:    NewBillingWebhookController(billingInbox),
		},
		Plan:          planController,
		ClientIP:      clientIPPolicy.Resolve,
		UserLimiter:   userLimiter,
		PublicLimiter: publicLimiter,
		Readiness:     database,
		closers:       closers,
	}, nil
}

func newBillingProvider(vars env.Variables) (billing.Provider, error) {
	if vars.StripeSecretKey == "" || vars.StripePriceIDUSD == "" {
		return billing.UnconfiguredProvider{}, nil
	}
	return billingstripe.New(billingstripe.Config{
		SecretKey:     vars.StripeSecretKey,
		WebhookSecret: vars.StripeWebhookSecret,
	})
}

func newBillingService(provider billing.Provider, store billing.Store, vars env.Variables) *billing.Service {
	return billing.NewService(store, provider, billing.Config{
		PriceID:    vars.StripePriceIDUSD,
		SuccessURL: vars.BillingSuccessURL,
		CancelURL:  vars.BillingCancelURL,
		Now:        time.Now,
	})
}

func newGenerationService(cfg env.Env, registry *template.Registry, options moduleOptions, deps generation.Deps) *generation.Service {
	deps.Publisher = options.generationPublisher
	if deps.Publisher == nil {
		deps.Publisher = unconfiguredPublisher{}
	}
	deps.Observer = options.capObserver
	if deps.Observer == nil {
		deps.Observer = noopCapObserver{}
	}
	return generation.NewService(generation.Config{
		Enabled:        cfg.Variables.GenerationEnabled,
		DailyCap:       cfg.Variables.GenerationDailyCap,
		DraftTemplates: registry.DraftsAllowed(),
	}, deps, time.Now)
}

func sheetDraftTemplateOption(registry *template.Registry) sheet.ServiceOption {
	return sheet.WithDraftTemplates(registry.DraftsAllowed())
}
