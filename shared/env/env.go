package env

import (
	"context"
	"legnaris/shared/logger"
	"os"
	"slices"
	"strings"
)

type Variables struct {
	Port        string
	AppName     string
	DatabaseURI string
	CORSOrigins []string
}

const (
	defaultPort    = "8080"
	defaultAppName = "legnaris"
)

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
		Port:        e.lookupStringEnv("PORT", defaultPort),
		AppName:     e.lookupStringEnv("APP_NAME", defaultAppName),
		DatabaseURI: e.lookupStringEnv("DATABASE_URI", "mongodb://root:password@localhost:27017/"),
		CORSOrigins: e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins),
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
			logger.Attribute{Key: "key", Value: key},
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
