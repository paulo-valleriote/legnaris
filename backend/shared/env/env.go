package env

import (
	"context"
	"errors"
	"fmt"
	"legnaris/backend/shared/logger"
	"math"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Variables struct {
	Port                              string
	AppName                           string
	DatabaseURI                       string
	DatabaseName                      string
	DatabasePingTimeout               time.Duration
	DatabaseServerSelectionTimeout    time.Duration
	DatabaseConnectTimeout            time.Duration
	DatabaseCloseTimeout              time.Duration
	DatabaseMaxPoolSize               uint64
	DatabaseMinPoolSize               uint64
	ShutdownTimeout                   time.Duration
	CORSOrigins                       []string
	JWTSecret                         string
	JWTAccessTTL                      time.Duration
	AuthRefreshTTL                    time.Duration
	AuthBcryptCost                    int
	AuthRateLimitAttempts             int
	AuthRateLimitWindow               time.Duration
	RegisterIPRateLimitAttempts       int
	RegisterIPRateLimitWindow         time.Duration
	RegisterEmailRateLimitAttempts    int
	RegisterEmailRateLimitWindow      time.Duration
	CSPReportRateLimitAttempts        int
	CSPReportRateLimitWindow          time.Duration
	UserRateLimitAttempts             int
	UserRateLimitWindow               time.Duration
	TrustedProxyCIDRs                 []string
	RedisURL                          string
	AppEnv                            string
	EmailProvider                     string
	ResendAPIKey                      string
	StripeSecretKey                   string
	StripeWebhookSecret               string
	StripePriceIDUSD                  string
	BillingSuccessURL                 string
	BillingCancelURL                  string
	EmailFrom                         string
	EmailSendTimeout                  time.Duration
	RabbitMQURL                       string
	EmailRelayPollInterval            time.Duration
	EmailRelayLease                   time.Duration
	EmailRelayBatch                   int
	EmailRelayConfirmTimeout          time.Duration
	EmailRelayMaxAttempts             int
	EmailOutboxRetention              time.Duration
	EmailRelayMetricsAddr             string
	EmailRetryDelayFirst              time.Duration
	EmailRetryDelaySecond             time.Duration
	EmailRetryDelayLater              time.Duration
	EmailWorkerPrefetch               int
	EmailWorkerMaxAttempts            int
	EmailWorkerMetricsAddr            string
	EmailCodeSecret                   string
	AuthOTPTTL                        time.Duration
	AuthOTPCeiling                    time.Duration
	AuthOTPResendCooldown             time.Duration
	AuthOTPMaxAttempts                int
	AuthOTPDailySendCap               int
	AuthOTPResendCapRatio             float64
	AuthOTPUserWindow                 time.Duration
	AuthRegisterMinDuration           time.Duration
	AuthOTPUserLimit                  int
	AuthDisposableEmailCheck          bool
	LLMProvider                       string
	LLMBaseURL                        string
	LLMAPIKey                         string
	LLMModel                          string
	LLMTimeout                        time.Duration
	GenerationEnabled                 bool
	GenerationDailyCap                int
	GenerationCreateRateLimitAttempts int
	GenerationCreateRateLimitWindow   time.Duration
	GenerationReadRateLimitAttempts   int
	CampaignJoinIPRateLimitAttempts   int
	CampaignJoinIPRateLimitWindow     time.Duration
	CampaignJoinRateLimitAttempts     int
	CampaignJoinRateLimitWindow       time.Duration
	GenerationReadRateLimitWindow     time.Duration
}

const (
	defaultPort                           = "8080"
	defaultAppName                        = "legnaris"
	defaultDatabaseName                   = "legnaris"
	defaultDatabasePingTimeout            = 1 * time.Second
	defaultDatabaseServerSelectionTimeout = 2 * time.Second
	defaultDatabaseConnectTimeout         = 2 * time.Second
	defaultDatabaseCloseTimeout           = 5 * time.Second
	defaultShutdownTimeout                = 5 * time.Second
	defaultDatabaseMaxPoolSize            = 20
	defaultDatabaseMinPoolSize            = 0
	defaultJWTAccessTTL                   = 15 * time.Minute
	defaultAuthRefreshTTL                 = 720 * time.Hour
	defaultAuthBcryptCost                 = 12
	defaultAuthRateLimitAttempts          = 10
	defaultAuthRateLimitWindow            = 1 * time.Minute
	defaultRegisterIPRateLimitAttempts    = 5
	defaultRegisterIPRateLimitWindow      = 1 * time.Hour
	defaultRegisterEmailRateLimitAttempts = 3
	defaultRegisterEmailRateLimitWindow   = 1 * time.Hour
	defaultCSPReportRateLimitAttempts     = 30
	defaultCSPReportRateLimitWindow       = 1 * time.Minute
	defaultUserRateLimitAttempts          = 80
	defaultUserRateLimitWindow            = 1 * time.Minute
	defaultCampaignJoinIPAttempts         = 20
	defaultCampaignJoinIPWindow           = 10 * time.Minute
	defaultCampaignJoinAttempts           = 10
	defaultCampaignJoinWindow             = 10 * time.Minute
	defaultEmailProvider                  = "log"
	defaultEmailSendTimeout               = 10 * time.Second
	defaultEmailRelayPollInterval         = 1 * time.Second
	defaultEmailRelayLease                = 30 * time.Second
	defaultEmailRelayBatch                = 25
	defaultEmailRelayConfirmTimeout       = 5 * time.Second
	defaultEmailRelayMaxAttempts          = 5
	defaultEmailRelayMetricsAddr          = ":9100"

	defaultEmailWorkerPrefetch     = 10
	defaultEmailWorkerMaxAttempts  = 5
	defaultEmailWorkerMetricsAddr  = ":9101"
	defaultAuthOTPTTL              = 10 * time.Minute
	defaultAuthOTPCeiling          = 30 * time.Minute
	defaultAuthOTPResendCooldown   = 60 * time.Second
	defaultAuthOTPMaxAttempts      = 5
	defaultAuthOTPDailySendCap     = 80
	defaultAuthOTPResendCapRatio   = 0.30
	defaultAuthOTPUserWindow       = time.Hour
	defaultAuthRegisterMinDuration = 500 * time.Millisecond
	defaultAuthOTPUserLimit        = 10
	defaultDisposableEmailCheck    = true
	defaultLLMProvider             = "openrouter"
	defaultLLMBaseURL              = "https://openrouter.ai/api/v1"
	defaultLLMModel                = "deepseek/deepseek-v4.1-flash"
	defaultLLMTimeout              = 60 * time.Second

	defaultGenerationEnabled         = false
	defaultGenerationDailyCap        = 200
	defaultGenerationCreateAttempts  = 5
	defaultGenerationReadAttempts    = 30
	defaultGenerationRateLimitWindow = 1 * time.Minute
)

const appEnvProduction = "production"

const DefaultEmailOutboxRetention = 168 * time.Hour

const attrKey = "key"

const (
	attrValue   = "value"
	attrDefault = "default"
)

const (
	keyDatabaseURI       = "DATABASE_URI"
	keyJWTSecret         = "JWT_SECRET"
	keyEmailCodeSecret   = "EMAIL_CODE_SECRET"
	keyTrustedProxyCIDRs = "TRUSTED_PROXY_CIDRS"
	keyRedisURL          = "REDIS_URL"
	keyAppEnv            = "APP_ENV"
	keyDisposableEmail   = "AUTH_DISPOSABLE_EMAIL_CHECK"
)

func RequiredKeys() []string {
	return []string{keyDatabaseURI, keyJWTSecret, keyEmailCodeSecret}
}

var defaultCORSOrigins = []string{"http://localhost:3000", "http://localhost:5173"}

type Env struct {
	Logger    logger.StructuredLogger
	Variables Variables
	invalid   []error
}

func (e Env) Validate() error {
	errs := slices.Clone(e.invalid)
	if e.Variables.DatabaseMinPoolSize > e.Variables.DatabaseMaxPoolSize {
		errs = append(errs, fmt.Errorf(
			"DATABASE_MIN_POOL_SIZE (%d) must not exceed DATABASE_MAX_POOL_SIZE (%d)",
			e.Variables.DatabaseMinPoolSize, e.Variables.DatabaseMaxPoolSize))
	}
	if err := validateRedisURL(e.Variables.RedisURL); err != nil {
		errs = append(errs, err)
	}
	if err := validateLLMModel(e.Variables.LLMModel); err != nil {
		errs = append(errs, err)
	}
	if e.Variables.GenerationDailyCap < 0 {
		errs = append(errs, errors.New("GENERATION_DAILY_CAP must not be negative"))
	}
	return errors.Join(errs...)
}

func validateLLMModel(model string) error {
	if strings.HasSuffix(model, ":free") {
		return errors.New("LLM_MODEL must not be a :free model variant (ADR 0008)")
	}
	return nil
}

func validateRedisURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "redis" && parsed.Scheme != "rediss") || parsed.Host == "" {
		return fmt.Errorf("%s must be a redis:// or rediss:// URL with a host", keyRedisURL)
	}
	return nil
}

func NewEnv(logger logger.StructuredLogger) Env {
	env := Env{
		Logger: logger,
	}

	env.Variables = env.initVariables()
	return env
}

func (e *Env) initVariables() Variables {
	vars := e.buildVariables()
	vars.AuthDisposableEmailCheck = e.resolveDisposableEmailCheck(vars.AppEnv)
	return vars
}

func (e *Env) buildVariables() Variables {
	return Variables{
		Port:                              e.lookupStringEnv("PORT", defaultPort),
		AppName:                           e.lookupStringEnv("APP_NAME", defaultAppName),
		DatabaseURI:                       e.lookupRequiredStringEnv(keyDatabaseURI),
		DatabaseName:                      e.lookupStringEnv("DATABASE_NAME", defaultDatabaseName),
		DatabasePingTimeout:               e.lookupDurationEnv("DATABASE_PING_TIMEOUT", defaultDatabasePingTimeout),
		DatabaseServerSelectionTimeout:    e.lookupDurationEnv("DATABASE_SERVER_SELECTION_TIMEOUT", defaultDatabaseServerSelectionTimeout),
		DatabaseConnectTimeout:            e.lookupDurationEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimeout),
		DatabaseCloseTimeout:              e.lookupDurationEnv("DATABASE_CLOSE_TIMEOUT", defaultDatabaseCloseTimeout),
		DatabaseMaxPoolSize:               e.lookupPoolSizeEnv("DATABASE_MAX_POOL_SIZE", defaultDatabaseMaxPoolSize, 1),
		DatabaseMinPoolSize:               e.lookupPoolSizeEnv("DATABASE_MIN_POOL_SIZE", defaultDatabaseMinPoolSize, 0),
		ShutdownTimeout:                   e.lookupDurationEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
		CORSOrigins:                       e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
		JWTSecret:                         e.lookupRequiredStringEnv(keyJWTSecret),
		JWTAccessTTL:                      e.lookupDurationEnv("JWT_ACCESS_TTL", defaultJWTAccessTTL),
		AuthRefreshTTL:                    e.lookupDurationEnv("AUTH_REFRESH_TTL", defaultAuthRefreshTTL),
		AuthBcryptCost:                    e.lookupIntEnv("AUTH_BCRYPT_COST", defaultAuthBcryptCost),
		AuthRateLimitAttempts:             e.lookupIntEnv("AUTH_RATE_LIMIT_ATTEMPTS", defaultAuthRateLimitAttempts),
		AuthRateLimitWindow:               e.lookupDurationEnv("AUTH_RATE_LIMIT_WINDOW", defaultAuthRateLimitWindow),
		RegisterIPRateLimitAttempts:       e.lookupIntEnv("AUTH_REGISTER_IP_RATE_LIMIT_ATTEMPTS", defaultRegisterIPRateLimitAttempts),
		RegisterIPRateLimitWindow:         e.lookupDurationEnv("AUTH_REGISTER_IP_RATE_LIMIT_WINDOW", defaultRegisterIPRateLimitWindow),
		RegisterEmailRateLimitAttempts:    e.lookupIntEnv("AUTH_REGISTER_EMAIL_RATE_LIMIT_ATTEMPTS", defaultRegisterEmailRateLimitAttempts),
		RegisterEmailRateLimitWindow:      e.lookupDurationEnv("AUTH_REGISTER_EMAIL_RATE_LIMIT_WINDOW", defaultRegisterEmailRateLimitWindow),
		CSPReportRateLimitAttempts:        e.lookupIntEnv("CSP_REPORT_RATE_LIMIT_ATTEMPTS", defaultCSPReportRateLimitAttempts),
		CSPReportRateLimitWindow:          e.lookupDurationEnv("CSP_REPORT_RATE_LIMIT_WINDOW", defaultCSPReportRateLimitWindow),
		UserRateLimitAttempts:             e.lookupIntEnv("USER_RATE_LIMIT_ATTEMPTS", defaultUserRateLimitAttempts),
		UserRateLimitWindow:               e.lookupDurationEnv("USER_RATE_LIMIT_WINDOW", defaultUserRateLimitWindow),
		CampaignJoinIPRateLimitAttempts:   e.lookupIntEnv("CAMPAIGN_JOIN_IP_RATE_LIMIT_ATTEMPTS", defaultCampaignJoinIPAttempts),
		CampaignJoinIPRateLimitWindow:     e.lookupDurationEnv("CAMPAIGN_JOIN_IP_RATE_LIMIT_WINDOW", defaultCampaignJoinIPWindow),
		CampaignJoinRateLimitAttempts:     e.lookupIntEnv("CAMPAIGN_JOIN_RATE_LIMIT_ATTEMPTS", defaultCampaignJoinAttempts),
		CampaignJoinRateLimitWindow:       e.lookupDurationEnv("CAMPAIGN_JOIN_RATE_LIMIT_WINDOW", defaultCampaignJoinWindow),
		TrustedProxyCIDRs:                 e.lookupTrustedProxyCIDRs(),
		RedisURL:                          e.lookupStringEnv(keyRedisURL, ""),
		AppEnv:                            e.lookupStringEnv(keyAppEnv, ""),
		EmailProvider:                     e.lookupStringEnv("EMAIL_PROVIDER", defaultEmailProvider),
		ResendAPIKey:                      e.lookupStringEnv("RESEND_API_KEY", ""),
		StripeSecretKey:                   e.lookupStringEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:               e.lookupStringEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceIDUSD:                  e.lookupStringEnv("STRIPE_PRICE_ID_USD", ""),
		BillingSuccessURL:                 e.lookupStringEnv("BILLING_SUCCESS_URL", ""),
		BillingCancelURL:                  e.lookupStringEnv("BILLING_CANCEL_URL", ""),
		EmailFrom:                         e.lookupStringEnv("EMAIL_FROM", ""),
		EmailSendTimeout:                  e.lookupDurationEnv("EMAIL_SEND_TIMEOUT", defaultEmailSendTimeout),
		RabbitMQURL:                       e.lookupStringEnv("RABBITMQ_URL", ""),
		EmailRelayPollInterval:            e.lookupDurationEnv("EMAIL_RELAY_POLL_INTERVAL", defaultEmailRelayPollInterval),
		EmailRelayLease:                   e.lookupDurationEnv("EMAIL_RELAY_LEASE", defaultEmailRelayLease),
		EmailRelayBatch:                   e.lookupIntEnv("EMAIL_RELAY_BATCH", defaultEmailRelayBatch),
		EmailRelayConfirmTimeout:          e.lookupDurationEnv("EMAIL_RELAY_CONFIRM_TIMEOUT", defaultEmailRelayConfirmTimeout),
		EmailRelayMaxAttempts:             e.lookupIntEnv("EMAIL_RELAY_MAX_ATTEMPTS", defaultEmailRelayMaxAttempts),
		EmailOutboxRetention:              e.lookupDurationEnv("EMAIL_OUTBOX_RETENTION", DefaultEmailOutboxRetention),
		EmailRelayMetricsAddr:             e.lookupStringEnv("EMAIL_RELAY_METRICS_ADDR", defaultEmailRelayMetricsAddr),
		EmailRetryDelayFirst:              e.lookupDurationEnv("EMAIL_RETRY_DELAY_FIRST", 0),
		EmailRetryDelaySecond:             e.lookupDurationEnv("EMAIL_RETRY_DELAY_SECOND", 0),
		EmailRetryDelayLater:              e.lookupDurationEnv("EMAIL_RETRY_DELAY_LATER", 0),
		EmailWorkerPrefetch:               e.lookupIntEnv("EMAIL_WORKER_PREFETCH", defaultEmailWorkerPrefetch),
		EmailWorkerMaxAttempts:            e.lookupIntEnv("EMAIL_WORKER_MAX_ATTEMPTS", defaultEmailWorkerMaxAttempts),
		EmailWorkerMetricsAddr:            e.lookupStringEnv("EMAIL_WORKER_METRICS_ADDR", defaultEmailWorkerMetricsAddr),
		EmailCodeSecret:                   e.lookupRequiredStringEnv(keyEmailCodeSecret),
		AuthOTPTTL:                        e.lookupDurationEnv("AUTH_OTP_TTL", defaultAuthOTPTTL),
		AuthOTPCeiling:                    e.lookupDurationEnv("AUTH_OTP_CEILING", defaultAuthOTPCeiling),
		AuthOTPResendCooldown:             e.lookupDurationEnv("AUTH_OTP_RESEND_COOLDOWN", defaultAuthOTPResendCooldown),
		AuthOTPMaxAttempts:                e.lookupIntEnv("AUTH_OTP_MAX_ATTEMPTS", defaultAuthOTPMaxAttempts),
		AuthOTPDailySendCap:               e.lookupIntEnv("AUTH_OTP_DAILY_SEND_CAP", defaultAuthOTPDailySendCap),
		AuthOTPResendCapRatio:             e.lookupRatioEnv("AUTH_OTP_RESEND_CAP_RATIO", defaultAuthOTPResendCapRatio),
		AuthOTPUserWindow:                 e.lookupDurationEnv("AUTH_OTP_USER_WINDOW", defaultAuthOTPUserWindow),
		AuthRegisterMinDuration:           e.lookupDurationEnv("AUTH_REGISTER_MIN_DURATION", defaultAuthRegisterMinDuration),
		AuthOTPUserLimit:                  e.lookupIntEnv("AUTH_OTP_USER_LIMIT", defaultAuthOTPUserLimit),
		LLMProvider:                       e.lookupStringEnv("LLM_PROVIDER", defaultLLMProvider),
		LLMBaseURL:                        e.lookupStringEnv("LLM_BASE_URL", defaultLLMBaseURL),
		LLMAPIKey:                         e.lookupStringEnv("LLM_API_KEY", ""),
		LLMModel:                          e.lookupStringEnv("LLM_MODEL", defaultLLMModel),
		LLMTimeout:                        e.lookupDurationEnv("LLM_TIMEOUT", defaultLLMTimeout),
		GenerationEnabled:                 e.lookupBoolEnv("GENERATION_ENABLED", defaultGenerationEnabled),
		GenerationDailyCap:                e.lookupIntEnv("GENERATION_DAILY_CAP", defaultGenerationDailyCap),
		GenerationCreateRateLimitAttempts: e.lookupIntEnv("GENERATION_CREATE_RATE_LIMIT_ATTEMPTS", defaultGenerationCreateAttempts),
		GenerationCreateRateLimitWindow:   e.lookupDurationEnv("GENERATION_CREATE_RATE_LIMIT_WINDOW", defaultGenerationRateLimitWindow),
		GenerationReadRateLimitAttempts:   e.lookupIntEnv("GENERATION_READ_RATE_LIMIT_ATTEMPTS", defaultGenerationReadAttempts),
		GenerationReadRateLimitWindow:     e.lookupDurationEnv("GENERATION_READ_RATE_LIMIT_WINDOW", defaultGenerationRateLimitWindow),
	}
}

func (e *Env) resolveDisposableEmailCheck(appEnv string) bool {
	enabled := e.lookupBoolEnv(keyDisposableEmail, defaultDisposableEmailCheck)
	if enabled || appEnv != appEnvProduction {
		return enabled
	}
	e.Logger.Warn(
		context.Background(),
		"disposable e-mail check cannot be disabled in production; keeping it enabled",
		logger.Attribute{Key: attrKey, Value: keyDisposableEmail},
	)
	return true
}

func (e *Env) lookupBoolEnv(key string, defaultValue bool) bool {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		e.Logger.Warn(
			context.Background(),
			"invalid boolean value for environment variable; using default",
			logger.Attribute{Key: attrKey, Value: key},
			logger.Attribute{Key: attrValue, Value: value},
			logger.Attribute{Key: attrDefault, Value: strconv.FormatBool(defaultValue)},
		)
		return defaultValue
	}
	return parsed
}

func (e *Env) lookupEnv(key string) (string, bool) {
	value, found := os.LookupEnv(key)
	if !found || value == "" {
		e.Logger.Warn(
			context.Background(),
			"environment variable not set; using default",
			logger.Attribute{Key: attrKey, Value: key},
		)
		return "", false
	}
	return value, true
}

func (e *Env) lookupStringEnv(key string, defaultValue string) string {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}
	return value
}

func (e *Env) lookupStringSliceEnv(key string, defaultValue []string) []string {
	value, found := e.lookupEnv(key)
	if !found {
		return slices.Clone(defaultValue)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSuffix(strings.TrimSpace(part), "/")
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (e *Env) lookupTrustedProxyCIDRs() []string {
	value, found := os.LookupEnv(keyTrustedProxyCIDRs)
	if !found || value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (e *Env) lookupDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		e.Logger.Warn(
			context.Background(),
			"invalid duration value for environment variable; using default",
			logger.Attribute{Key: attrKey, Value: key},
			logger.Attribute{Key: attrValue, Value: value},
			logger.Attribute{Key: attrDefault, Value: defaultValue.String()},
		)
		return defaultValue
	}
	return parsed
}

func (e *Env) lookupPoolSizeEnv(key string, defaultValue, minimum uint64) uint64 {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed < minimum {
		e.invalid = append(e.invalid, fmt.Errorf(
			"invalid %s %q: must be an integer between %d and %d", key, value, minimum, math.MaxUint32))
		return defaultValue
	}
	return parsed
}

func (e *Env) lookupRequiredStringEnv(key string) string {
	value, found := e.lookupEnv(key)
	if !found {
		e.Logger.Warn(
			context.Background(),
			"required environment variable not set",
			logger.Attribute{Key: attrKey, Value: key},
		)
		return ""
	}
	return value
}

func (e *Env) lookupRatioEnv(key string, defaultValue float64) float64 {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || parsed <= 0 || parsed > 1 {
		e.Logger.Warn(
			context.Background(),
			"invalid ratio value for environment variable; using default",
			logger.Attribute{Key: attrKey, Value: key},
			logger.Attribute{Key: attrValue, Value: value},
			logger.Attribute{Key: attrDefault, Value: strconv.FormatFloat(defaultValue, 'f', -1, 64)},
		)
		return defaultValue
	}
	return parsed
}

func (e *Env) lookupIntEnv(key string, defaultValue int) int {
	value, found := e.lookupEnv(key)
	if !found {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		e.Logger.Warn(
			context.Background(),
			"invalid integer value for environment variable; using default",
			logger.Attribute{Key: attrKey, Value: key},
			logger.Attribute{Key: attrValue, Value: value},
			logger.Attribute{Key: attrDefault, Value: strconv.Itoa(defaultValue)},
		)
		return defaultValue
	}
	return parsed
}
