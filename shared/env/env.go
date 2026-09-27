package env

import (
	"context"
	"legnaris/shared/logger"
	"os"
	"slices"
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
}

const (
	defaultPort                           = "8080"
	defaultAppName                        = "legnaris"
	defaultDatabaseName                   = "legnaris"
	defaultDatabasePingTimeout            = 1 * time.Second
	defaultDatabaseServerSelectionTimeout = 2 * time.Second
	defaultDatabaseConnectTimeout         = 2 * time.Second
	defaultDatabaseCloseTimeout           = 5 * time.Second
)

const attrKey = "key"

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
		DatabaseURI:                    e.lookupRequiredStringEnv("DATABASE_URI"),
		DatabaseName:                   e.lookupStringEnv("DATABASE_NAME", defaultDatabaseName),
		DatabasePingTimeout:            e.lookupDurationEnv("DATABASE_PING_TIMEOUT", defaultDatabasePingTimeout),
		DatabaseServerSelectionTimeout: e.lookupDurationEnv("DATABASE_SERVER_SELECTION_TIMEOUT", defaultDatabaseServerSelectionTimeout),
		DatabaseConnectTimeout:         e.lookupDurationEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimeout),
		DatabaseCloseTimeout:           e.lookupDurationEnv("DATABASE_CLOSE_TIMEOUT", defaultDatabaseCloseTimeout),
		CORSOrigins:                    e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
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
