package env

import (
	"context"
	"fmt"
	"legnaris/shared/logger"
	"os"
)

type Variables struct {
	Port        string
	AppName     string
	DatabaseURI string
}

const (
	defaultPort    = "8080"
	defaultAppName = "legnaris"
)

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

	return Variables{
		Port:        port,
		AppName:     appName,
		DatabaseURI: databaseURI,
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
