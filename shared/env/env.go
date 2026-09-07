package env

import (
	"context"
	"legnaris/shared/logger"
	"os"
)

type EnvWire interface {
	lookupEnv(key string, defaultValue string) (string, error)
}

type Variables struct {
	PORT     string
	APP_NAME string
}

const (
	defaultPort    = "8080"
	defaultAppName = "legnaris"
)

type Env struct {
	logger    logger.StructuredLogger
	Variables Variables
}

func NewEnv(logger logger.StructuredLogger) Env {
	env := Env{
		logger: logger,
	}

	env.Variables = env.initVariables()
	return env
}

func (e *Env) initVariables() Variables {
	port := e.lookupEnv("PORT", defaultPort)
	appName := e.lookupEnv("APP_NAME", defaultAppName)

	return Variables{
		PORT:     port,
		APP_NAME: appName,
	}
}

func (e *Env) lookupEnv(key string, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if !found && value == "" {
		e.logger.Warn(
			context.Background(),
			"env variable [%s] not found using default value",
		)
		return defaultValue
	}

	return value
}
