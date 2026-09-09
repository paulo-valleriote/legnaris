package db

import (
	"context"
	"legnaris/shared/env"
	loggerMocks "legnaris/shared/logger/mocks"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var _ DatabaseLifecycle = (*Database)(nil)

func databaseTestEnv(structuredLogger *loggerMocks.MockStructuredLogger, databaseURL string) env.Env {
	return env.Env{
		Logger: structuredLogger,
		Variables: env.Variables{
			DatabaseURL:            databaseURL,
			DatabaseMaxConnections: 20,
			DatabaseMinConnections: 4,
			DatabaseConnLifetime:   30 * time.Minute,
			DatabaseCheckPeriod:    45 * time.Second,
		},
	}
}

func TestNewDatabase(t *testing.T) {
	t.Run("returns an error when database URL is missing", func(t *testing.T) {
		structuredLogger := loggerMocks.NewMockStructuredLogger(t)
		structuredLogger.On("Error", mock.Anything, "database url isnt configured").Once()

		database, err := NewDatabase(databaseTestEnv(structuredLogger, ""))

		assert.Error(t, err)
		assert.Nil(t, database)
	})

	t.Run("returns an error when database URL is invalid", func(t *testing.T) {
		structuredLogger := loggerMocks.NewMockStructuredLogger(t)
		structuredLogger.On("Error", mock.Anything, "failed to parse database url").Once()

		database, err := NewDatabase(databaseTestEnv(structuredLogger, "://invalid"))

		assert.Error(t, err)
		assert.Nil(t, database)
	})

	t.Run("creates a pool with configured values", func(t *testing.T) {
		maxConnections := 20
		minConnections := 4
		connectionLifetime := 30 * time.Minute
		checkPeriod := 45 * time.Second

		structuredLogger := loggerMocks.NewMockStructuredLogger(t)
		database, err := NewDatabase(databaseTestEnv(structuredLogger, "postgres://user:password@localhost:5432/database?sslmode=disable"))

		assert.NoError(t, err)
		if assert.NotNil(t, database) {
			config := database.Pool.Config()
			assert.Equal(t, int32(maxConnections), config.MaxConns)
			assert.Equal(t, int32(minConnections), config.MinConns)
			assert.Equal(t, connectionLifetime, config.MaxConnLifetime)
			assert.Equal(t, checkPeriod, config.HealthCheckPeriod)
			database.Close()
		}
	})
}

func TestDatabasePing(t *testing.T) {
	structuredLogger := loggerMocks.NewMockStructuredLogger(t)
	structuredLogger.On("Error", mock.Anything, "failed to ping database").Once()

	config, err := pgxpool.ParseConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	assert.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	assert.NoError(t, err)
	defer pool.Close()

	database := &Database{Pool: pool, logger: structuredLogger}

	err = database.Ping(context.Background())

	assert.Error(t, err)
}

func TestDatabaseClose(t *testing.T) {
	t.Run("closes the configured pool", func(t *testing.T) {
		pool, err := pgxpool.New(context.Background(), "postgres://user:password@localhost:5432/database?sslmode=disable")
		assert.NoError(t, err)

		database := &Database{Pool: pool}

		database.Close()
	})

	t.Run("does nothing when pool is nil", func(t *testing.T) {
		database := &Database{}

		database.Close()
	})
}
