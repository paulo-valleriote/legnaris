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
	Port                           string
	AppName                        string
	DatabaseURI                    string
	DatabaseName                   string
	DatabasePingTimeout            time.Duration
	DatabaseServerSelectionTimeout time.Duration
	DatabaseConnectTimeout         time.Duration
	DatabaseCloseTimeout           time.Duration
	// DatabaseMaxPoolSize and DatabaseMinPoolSize bound the Mongo driver's
	// connection pool per process. Unlike the lenient readers, invalid values
	// are reported by Env.Validate so a mis-sized pool fails the boot.
	DatabaseMaxPoolSize        uint64
	DatabaseMinPoolSize        uint64
	ShutdownTimeout            time.Duration
	CORSOrigins                []string
	JWTSecret                  string
	JWTAccessTTL               time.Duration
	AuthRefreshTTL             time.Duration
	AuthBcryptCost             int
	AuthRateLimitAttempts      int
	AuthRateLimitWindow        time.Duration
	CSPReportRateLimitAttempts int
	CSPReportRateLimitWindow   time.Duration
	UserRateLimitAttempts      int
	UserRateLimitWindow        time.Duration
	// TrustedProxyCIDRs is the operator's trusted proxy set. It is nil when
	// unset or blank, which selects the documented loopback/private/link-local
	// default; a configured value replaces the non-loopback portion (PRQ-03).
	TrustedProxyCIDRs []string
	// RedisURL selects the shared rate-limit store. Empty keeps the in-process
	// limiter, whose effective cap is the configured value per replica; it is
	// validated by Env.Validate so a malformed URL fails the boot (ADR 0003).
	RedisURL string
	// Email and broker settings are all optional with documented defaults, so
	// the request-serving process starts when none of them is set and the
	// deployment's required-variable list is unchanged. The relay — not the
	// API — validates the broker URL it needs (EML-14, EML-15, EML-16).
	EmailProvider            string
	ResendAPIKey             string
	EmailFrom                string
	EmailSendTimeout         time.Duration
	RabbitMQURL              string
	EmailRelayPollInterval   time.Duration
	EmailRelayLease          time.Duration
	EmailRelayBatch          int
	EmailRelayConfirmTimeout time.Duration
	EmailRelayMaxAttempts    int
	EmailOutboxRetention     time.Duration
	EmailRelayMetricsAddr    string
	// The worker's settings are optional too: the worker validates the broker
	// and the provider itself, so the required list stays unchanged (WKR-14,
	// WKR-16, WKR-19, WKR-20).
	// The retry delay tiers are shared by every process that declares the
	// topology; zero selects the tier's default (WKR-14).
	EmailRetryDelayFirst   time.Duration
	EmailRetryDelaySecond  time.Duration
	EmailRetryDelayLater   time.Duration
	EmailWorkerPrefetch    int
	EmailWorkerMaxAttempts int
	EmailWorkerMetricsAddr string
	// EmailCodeSecret keys the OTP digest and is required: the API itself
	// digests codes, so an absent, blank or too-short value must fail the boot
	// rather than silently weaken every code (F-09, OTP-18, SC-008). The
	// challenge settings are optional with documented defaults.
	EmailCodeSecret       string
	AuthOTPTTL            time.Duration
	AuthOTPCeiling        time.Duration
	AuthOTPResendCooldown time.Duration
	AuthOTPMaxAttempts    int
	AuthOTPDailySendCap   int
	AuthOTPUserWindow     time.Duration
	AuthOTPUserLimit      int
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
	// A single page can emit several CSP reports in a row, so the report cap is
	// deliberately looser than the auth cap's 10/min while staying bounded
	// (POL-05, contract csp-report.contract.md).
	defaultCSPReportRateLimitAttempts = 30
	defaultCSPReportRateLimitWindow   = 1 * time.Minute
	// Authenticated routes are capped per user and the remaining public probes
	// (health, metrics) per address (LGN-30).
	defaultUserRateLimitAttempts = 80
	defaultUserRateLimitWindow   = 1 * time.Minute
	// Email-delivery and relay defaults (EML-14).
	defaultEmailProvider            = "log"
	defaultEmailSendTimeout         = 10 * time.Second
	defaultEmailRelayPollInterval   = 1 * time.Second
	defaultEmailRelayLease          = 30 * time.Second
	defaultEmailRelayBatch          = 25
	defaultEmailRelayConfirmTimeout = 5 * time.Second
	defaultEmailRelayMaxAttempts    = 5
	defaultEmailRelayMetricsAddr    = ":9100"

	// Email-worker defaults (WKR-14, WKR-16, WKR-20).
	defaultEmailWorkerPrefetch    = 10
	defaultEmailWorkerMaxAttempts = 5
	defaultEmailWorkerMetricsAddr = ":9101"
	// Challenge defaults (OTP-18): the code lifetime, its absolute ceiling, the
	// resend cooldown, the attempt cap, the daily send cap (deliberately below
	// the provider's free-tier 100) and the durable per-account bound.
	defaultAuthOTPTTL            = 10 * time.Minute
	defaultAuthOTPCeiling        = 30 * time.Minute
	defaultAuthOTPResendCooldown = 60 * time.Second
	defaultAuthOTPMaxAttempts    = 5
	defaultAuthOTPDailySendCap   = 80
	defaultAuthOTPUserWindow     = time.Hour
	defaultAuthOTPUserLimit      = 10
)

// DefaultEmailOutboxRetention is the retention window after which a published
// send intention becomes eligible for retirement; it backs the email_outbox
// published_at TTL index (EML-17). It is exported so backend/shared/db's index
// declaration and this variable's default share one source of truth.
const DefaultEmailOutboxRetention = 168 * time.Hour

const attrKey = "key"

// Structured-log field keys shared by the readers that report an unusable
// value: the raw value and the default that replaced it.
const (
	attrValue   = "value"
	attrDefault = "default"
)

// Environment variables the application cannot start without. Kept as
// constants so RequiredKeys and initVariables share one source of truth.
const (
	keyDatabaseURI       = "DATABASE_URI"
	keyJWTSecret         = "JWT_SECRET"
	keyEmailCodeSecret   = "EMAIL_CODE_SECRET"
	keyTrustedProxyCIDRs = "TRUSTED_PROXY_CIDRS"
	keyRedisURL          = "REDIS_URL"
)

// RequiredKeys returns the environment variables the application cannot start
// without. Deployment configuration (e.g. deploy/docker-compose.yml) must
// provide every one of them; exposing the set here lets that be validated
// against this single source of truth instead of drifting silently.
func RequiredKeys() []string {
	return []string{keyDatabaseURI, keyJWTSecret, keyEmailCodeSecret}
}

// defaultCORSOrigins are the allowed origins during local development. Set
// CORS_ALLOWED_ORIGINS (comma-separated) to override, e.g. with the production
// frontend origin.
var defaultCORSOrigins = []string{"http://localhost:3000", "http://localhost:5173"}

type Env struct {
	Logger    logger.StructuredLogger
	Variables Variables
	invalid   []error
}

// Validate reports configuration that must abort startup: unusable pool sizes
// and a minimum pool larger than the maximum. It returns nil when valid.
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
	return errors.Join(errs...)
}

// validateRedisURL accepts an empty value (in-process limiter) or a redis://
// or rediss:// URL with a host.
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
	return Variables{
		Port:                           e.lookupStringEnv("PORT", defaultPort),
		AppName:                        e.lookupStringEnv("APP_NAME", defaultAppName),
		DatabaseURI:                    e.lookupRequiredStringEnv(keyDatabaseURI),
		DatabaseName:                   e.lookupStringEnv("DATABASE_NAME", defaultDatabaseName),
		DatabasePingTimeout:            e.lookupDurationEnv("DATABASE_PING_TIMEOUT", defaultDatabasePingTimeout),
		DatabaseServerSelectionTimeout: e.lookupDurationEnv("DATABASE_SERVER_SELECTION_TIMEOUT", defaultDatabaseServerSelectionTimeout),
		DatabaseConnectTimeout:         e.lookupDurationEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimeout),
		DatabaseCloseTimeout:           e.lookupDurationEnv("DATABASE_CLOSE_TIMEOUT", defaultDatabaseCloseTimeout),
		DatabaseMaxPoolSize:            e.lookupPoolSizeEnv("DATABASE_MAX_POOL_SIZE", defaultDatabaseMaxPoolSize, 1),
		DatabaseMinPoolSize:            e.lookupPoolSizeEnv("DATABASE_MIN_POOL_SIZE", defaultDatabaseMinPoolSize, 0),
		ShutdownTimeout:                e.lookupDurationEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout),
		CORSOrigins:                    e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
		JWTSecret:                      e.lookupRequiredStringEnv(keyJWTSecret),
		JWTAccessTTL:                   e.lookupDurationEnv("JWT_ACCESS_TTL", defaultJWTAccessTTL),
		AuthRefreshTTL:                 e.lookupDurationEnv("AUTH_REFRESH_TTL", defaultAuthRefreshTTL),
		AuthBcryptCost:                 e.lookupIntEnv("AUTH_BCRYPT_COST", defaultAuthBcryptCost),
		AuthRateLimitAttempts:          e.lookupIntEnv("AUTH_RATE_LIMIT_ATTEMPTS", defaultAuthRateLimitAttempts),
		AuthRateLimitWindow:            e.lookupDurationEnv("AUTH_RATE_LIMIT_WINDOW", defaultAuthRateLimitWindow),
		CSPReportRateLimitAttempts:     e.lookupIntEnv("CSP_REPORT_RATE_LIMIT_ATTEMPTS", defaultCSPReportRateLimitAttempts),
		CSPReportRateLimitWindow:       e.lookupDurationEnv("CSP_REPORT_RATE_LIMIT_WINDOW", defaultCSPReportRateLimitWindow),
		UserRateLimitAttempts:          e.lookupIntEnv("USER_RATE_LIMIT_ATTEMPTS", defaultUserRateLimitAttempts),
		UserRateLimitWindow:            e.lookupDurationEnv("USER_RATE_LIMIT_WINDOW", defaultUserRateLimitWindow),
		TrustedProxyCIDRs:              e.lookupTrustedProxyCIDRs(),
		RedisURL:                       e.lookupStringEnv(keyRedisURL, ""),
		EmailProvider:                  e.lookupStringEnv("EMAIL_PROVIDER", defaultEmailProvider),
		ResendAPIKey:                   e.lookupStringEnv("RESEND_API_KEY", ""),
		EmailFrom:                      e.lookupStringEnv("EMAIL_FROM", ""),
		EmailSendTimeout:               e.lookupDurationEnv("EMAIL_SEND_TIMEOUT", defaultEmailSendTimeout),
		RabbitMQURL:                    e.lookupStringEnv("RABBITMQ_URL", ""),
		EmailRelayPollInterval:         e.lookupDurationEnv("EMAIL_RELAY_POLL_INTERVAL", defaultEmailRelayPollInterval),
		EmailRelayLease:                e.lookupDurationEnv("EMAIL_RELAY_LEASE", defaultEmailRelayLease),
		EmailRelayBatch:                e.lookupIntEnv("EMAIL_RELAY_BATCH", defaultEmailRelayBatch),
		EmailRelayConfirmTimeout:       e.lookupDurationEnv("EMAIL_RELAY_CONFIRM_TIMEOUT", defaultEmailRelayConfirmTimeout),
		EmailRelayMaxAttempts:          e.lookupIntEnv("EMAIL_RELAY_MAX_ATTEMPTS", defaultEmailRelayMaxAttempts),
		EmailOutboxRetention:           e.lookupDurationEnv("EMAIL_OUTBOX_RETENTION", DefaultEmailOutboxRetention),
		EmailRelayMetricsAddr:          e.lookupStringEnv("EMAIL_RELAY_METRICS_ADDR", defaultEmailRelayMetricsAddr),
		EmailRetryDelayFirst:           e.lookupDurationEnv("EMAIL_RETRY_DELAY_FIRST", 0),
		EmailRetryDelaySecond:          e.lookupDurationEnv("EMAIL_RETRY_DELAY_SECOND", 0),
		EmailRetryDelayLater:           e.lookupDurationEnv("EMAIL_RETRY_DELAY_LATER", 0),
		EmailWorkerPrefetch:            e.lookupIntEnv("EMAIL_WORKER_PREFETCH", defaultEmailWorkerPrefetch),
		EmailWorkerMaxAttempts:         e.lookupIntEnv("EMAIL_WORKER_MAX_ATTEMPTS", defaultEmailWorkerMaxAttempts),
		EmailWorkerMetricsAddr:         e.lookupStringEnv("EMAIL_WORKER_METRICS_ADDR", defaultEmailWorkerMetricsAddr),
		EmailCodeSecret:                e.lookupRequiredStringEnv(keyEmailCodeSecret),
		AuthOTPTTL:                     e.lookupDurationEnv("AUTH_OTP_TTL", defaultAuthOTPTTL),
		AuthOTPCeiling:                 e.lookupDurationEnv("AUTH_OTP_CEILING", defaultAuthOTPCeiling),
		AuthOTPResendCooldown:          e.lookupDurationEnv("AUTH_OTP_RESEND_COOLDOWN", defaultAuthOTPResendCooldown),
		AuthOTPMaxAttempts:             e.lookupIntEnv("AUTH_OTP_MAX_ATTEMPTS", defaultAuthOTPMaxAttempts),
		AuthOTPDailySendCap:            e.lookupIntEnv("AUTH_OTP_DAILY_SEND_CAP", defaultAuthOTPDailySendCap),
		AuthOTPUserWindow:              e.lookupDurationEnv("AUTH_OTP_USER_WINDOW", defaultAuthOTPUserWindow),
		AuthOTPUserLimit:               e.lookupIntEnv("AUTH_OTP_USER_LIMIT", defaultAuthOTPUserLimit),
	}
}

// lookupEnv reads key from the environment and reports whether it was set to a
// non-empty value. When absent, it logs a structured warning and returns false.
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

// lookupStringSliceEnv reads a comma-separated env var into a slice, trimming
// whitespace, dropping empty entries, and normalizing trailing slashes so
// origins match at runtime. The fallback default is cloned so callers never
// alias package-level state.
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

// lookupTrustedProxyCIDRs reads the optional trusted proxy set. It preserves the
// distinction the resolver needs: unset and blank mean "use the documented
// default" (nil), while a configured value such as "," means "trust exactly what
// I listed" (empty, non-nil). CIDRs are not validated here; an invalid entry is
// a boot error when the policy is built (PRQ-03, PRQ-18).
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

// lookupDurationEnv reads a duration from the environment using Go's
// time.ParseDuration format (e.g. "2s", "500ms", "1m30s"). When the variable
// is unset, empty, invalid, or parses to a non-positive duration, it logs a
// structured warning and returns the provided default.
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

// lookupPoolSizeEnv reads an unsigned integer of at least minimum. Unset or
// empty returns the default; an unusable value is recorded for Validate and
// the default is returned meanwhile.
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

// lookupRequiredStringEnv reads a required string from the environment. When
// the variable is unset or empty, it logs a structured warning and returns an
// empty string (the caller must handle the empty case).
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

// lookupIntEnv reads a positive integer from the environment. When the variable
// is unset, empty, non-numeric, or not positive, it logs a structured warning
// and returns the provided default.
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
