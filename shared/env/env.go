package env

import (
	"context"
	"fmt"
	"legnaris/shared/logger"
	"os"
	"strconv"
	"time"
)

type EnvWire interface {
	lookupEnv(key string, defaultValue string) (string, error)
}

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

func (e *Env) lookupTimeEnv(key string, defaultValue time.Duration) time.Duration {
	value, found := os.LookupEnv(key)
	if !found && value == "" {
		e.Logger.Warn(
			context.Background(),
			fmt.Sprintf("env variable [%s] not found using default value", key),
		)
		return defaultValue
	}

	timeValue, err := parseTime(value)
	if err != nil {
		e.Logger.Fatal(context.Background(), fmt.Sprintf("invalid time duration value %q: %v", value, err))
	}

	return timeValue
}

func (e *Env) lookupIntEnv(key string, defaultValue int) int {
	value, found := os.LookupEnv(key)
	if !found && value == "" {
		e.Logger.Warn(
			context.Background(),
			fmt.Sprintf("env variable [%s] not found using default value", key),
		)
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		e.Logger.Fatal(context.Background(), fmt.Sprintf("invalid integer value %q: %v", value, err))
	}

	return intValue
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

func parseTime(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return time.Duration(0), fmt.Errorf("invalid time duration %q: %w", value, err)
	}

	return duration, nil
}
