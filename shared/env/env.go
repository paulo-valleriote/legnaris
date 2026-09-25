package env

import (
	"context"
	"fmt"
	"legnaris/shared/logger"
	"os"
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
	port := e.lookupStringEnv("PORT", defaultPort)
	appName := e.lookupStringEnv("APP_NAME", defaultAppName)
	databaseURI := e.lookupStringEnv("DATABASE_URI", "mongodb://root:password@localhost:27017/")
	corsOrigins := e.lookupStringSliceEnv("CORS_ALLOWED_ORIGINS", defaultCORSOrigins)

	return Variables{
		Port:        port,
		AppName:     appName,
		DatabaseURI: databaseURI,
		CORSOrigins: corsOrigins,
	}
}

func (e *Env) lookupStringEnv(key string, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if !found && value == "" {
		e.Logger.Warn(
			context.Background(),
			fmt.Sprintf("env variable [%s] not found using default value", key),
		)
		return defaultValue
	}

	return value
}

// lookupStringSliceEnv reads a comma-separated env var into a trimmed slice.
func (e *Env) lookupStringSliceEnv(key string, defaultValue []string) []string {
	value, found := os.LookupEnv(key)
	if !found || value == "" {
		e.Logger.Warn(
			context.Background(),
			fmt.Sprintf("env variable [%s] not found using default value", key),
		)
		return defaultValue
	}

	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
