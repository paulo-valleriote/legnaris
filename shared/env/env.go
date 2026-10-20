package env

import (
	"context"
	"legnaris/shared/logger"
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
	CORSOrigins                    []string
	JWTSecret                      string
	JWTAccessTTL                   time.Duration
	AuthRefreshTTL                 time.Duration
	AuthBcryptCost                 int
	AuthRateLimitAttempts          int
	AuthRateLimitWindow            time.Duration
	CSPReportRateLimitAttempts     int
	CSPReportRateLimitWindow       time.Duration
	// TrustedProxyCIDRs is the operator's trusted proxy set. It is nil when
	// unset or blank, which selects the documented loopback/private/link-local
	// default; a configured value replaces the non-loopback portion (PRQ-03).
	TrustedProxyCIDRs []string
}

const (
	defaultPort                           = "8080"
	defaultAppName                        = "legnaris"
	defaultDatabaseName                   = "legnaris"
	defaultDatabasePingTimeout            = 1 * time.Second
	defaultDatabaseServerSelectionTimeout = 2 * time.Second
	defaultDatabaseConnectTimeout         = 2 * time.Second
	defaultDatabaseCloseTimeout           = 5 * time.Second
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
)

const attrKey = "key"

// Environment variables the application cannot start without. Kept as
// constants so RequiredKeys and initVariables share one source of truth.
const (
	keyDatabaseURI       = "DATABASE_URI"
	keyJWTSecret         = "JWT_SECRET"
	keyTrustedProxyCIDRs = "TRUSTED_PROXY_CIDRS"
)

// RequiredKeys returns the environment variables the application cannot start
// without. Deployment configuration (e.g. deploy/docker-compose.yml) must
// provide every one of them; exposing the set here lets that be validated
// against this single source of truth instead of drifting silently.
func RequiredKeys() []string {
	return []string{keyDatabaseURI, keyJWTSecret}
}

// defaultCORSOrigins are the allowed origins during local development. Set
// CORS_ALLOWED_ORIGINS (comma-separated) to override, e.g. with the production
// frontend origin.
var defaultCORSOrigins = []string{"http://localhost:3000", "http://localhost:5173"}

type Env struct {
	Logger    logger.StructuredLogger
	Variables Variables
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
		CORSOrigins:                    e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
		JWTSecret:                      e.lookupRequiredStringEnv(keyJWTSecret),
		JWTAccessTTL:                   e.lookupDurationEnv("JWT_ACCESS_TTL", defaultJWTAccessTTL),
		AuthRefreshTTL:                 e.lookupDurationEnv("AUTH_REFRESH_TTL", defaultAuthRefreshTTL),
		AuthBcryptCost:                 e.lookupIntEnv("AUTH_BCRYPT_COST", defaultAuthBcryptCost),
		AuthRateLimitAttempts:          e.lookupIntEnv("AUTH_RATE_LIMIT_ATTEMPTS", defaultAuthRateLimitAttempts),
		AuthRateLimitWindow:            e.lookupDurationEnv("AUTH_RATE_LIMIT_WINDOW", defaultAuthRateLimitWindow),
		CSPReportRateLimitAttempts:     e.lookupIntEnv("CSP_REPORT_RATE_LIMIT_ATTEMPTS", defaultCSPReportRateLimitAttempts),
		CSPReportRateLimitWindow:       e.lookupDurationEnv("CSP_REPORT_RATE_LIMIT_WINDOW", defaultCSPReportRateLimitWindow),
		TrustedProxyCIDRs:              e.lookupTrustedProxyCIDRs(),
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
			logger.Attribute{Key: "value", Value: value},
			logger.Attribute{Key: "default", Value: defaultValue.String()},
		)
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
			logger.Attribute{Key: "value", Value: value},
			logger.Attribute{Key: "default", Value: strconv.Itoa(defaultValue)},
		)
		return defaultValue
	}
	return parsed
}
