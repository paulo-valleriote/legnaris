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
	Port                   string
	AppName                string
	DatabaseURL            string
	DatabaseMaxConnections int
	DatabaseMinConnections int
	DatabaseConnLifetime   time.Duration
	DatabaseCheckPeriod    time.Duration
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
	port := e.lookupStringEnv("Port", defaultPort)
	appName := e.lookupStringEnv("AppName", defaultAppName)
	databaseURL := e.lookupStringEnv("DatabaseURL", "")
	databaseMaxConnections := e.lookupIntEnv("DatabaseMaxConnections", 10)
	databaseMinConnections := e.lookupIntEnv("DatabaseMinConnections", 5)
	databaseConnLifetime := e.lookupTimeEnv("DatabaseConnLifetime", time.Minute*5)
	databaseCheckPeriod := e.lookupTimeEnv("DatabaseCheckPeriod", time.Minute*1)

	return Variables{
		Port:                   port,
		AppName:                appName,
		DatabaseURL:            databaseURL,
		DatabaseMaxConnections: databaseMaxConnections,
		DatabaseMinConnections: databaseMinConnections,
		DatabaseConnLifetime:   databaseConnLifetime,
		DatabaseCheckPeriod:    databaseCheckPeriod,
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
