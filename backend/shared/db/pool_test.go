package db_test

import (
	"log/slog"
	"testing"

	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitMongo_RejectsInvalidPoolConfig(t *testing.T) {
	tests := []struct {
		name string
		max  string
		min  string
	}{
		{name: "Error: zero max pool size", max: "0"},
		{name: "Error: negative max pool size", max: "-5"},
		{name: "Error: non-numeric max pool size", max: "lots"},
		{name: "Error: negative min pool size", min: "-1"},
		{name: "Error: min greater than max", max: "2", min: "3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URI", "mongodb://root:pw@127.0.0.1:1/?replicaSet=rs0&authSource=admin&serverSelectionTimeoutMS=50")
			if tc.max != "" {
				t.Setenv("DATABASE_MAX_POOL_SIZE", tc.max)
			}
			if tc.min != "" {
				t.Setenv("DATABASE_MIN_POOL_SIZE", tc.min)
			}
			cfg := env.NewEnv(logger.NewLogger(slog.LevelError))

			database, err := db.InitMongo(cfg)

			require.Error(t, err)
			assert.Nil(t, database)
			assert.ErrorIs(t, err, db.ErrInvalidConfig)
			assert.Contains(t, err.Error(), "POOL_SIZE")
		})
	}
}

func TestInitMongo_ValidPoolConfigReachesConnection(t *testing.T) {
	t.Run("Happy path: valid pool config passes validation and fails only on ping", func(t *testing.T) {
		t.Setenv("DATABASE_URI", "mongodb://root:pw@127.0.0.1:1/?replicaSet=rs0&authSource=admin&serverSelectionTimeoutMS=50")
		t.Setenv("DATABASE_MAX_POOL_SIZE", "10")
		t.Setenv("DATABASE_MIN_POOL_SIZE", "2")
		cfg := env.NewEnv(logger.NewLogger(slog.LevelError))

		_, err := db.InitMongo(cfg)

		require.Error(t, err)
		assert.NotErrorIs(t, err, db.ErrInvalidConfig)
		assert.Contains(t, err.Error(), "ping mongodb")
	})
}

func TestInitMongo_RequiresReplicaSet(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "Error: standalone URI without replicaSet", uri: "mongodb://root:pw@127.0.0.1:1/?serverSelectionTimeoutMS=50"},
		{name: "Edge: empty replicaSet value", uri: "mongodb://root:pw@127.0.0.1:1/?replicaSet=&serverSelectionTimeoutMS=50"},
		{name: "Edge: URI with credentials but no replicaSet", uri: "mongodb://root:pw@127.0.0.1:1/?authSource=admin&serverSelectionTimeoutMS=50"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URI", tc.uri)
			cfg := env.NewEnv(logger.NewLogger(slog.LevelError))

			database, err := db.InitMongo(cfg)

			assert.Nil(t, database)
			assert.ErrorIs(t, err, db.ErrMissingReplicaSet)
			assert.ErrorIs(t, err, db.ErrInvalidConfig)
		})
	}
}

func TestInitMongo_RejectsUnparsableURIWithoutLeakingIt(t *testing.T) {
	t.Run("Error: malformed URI is invalid config and is never echoed", func(t *testing.T) {
		t.Setenv("DATABASE_URI", "mongodb://:secret@127.0.0.1:1/?replicaSet=rs0")
		cfg := env.NewEnv(logger.NewLogger(slog.LevelError))

		database, err := db.InitMongo(cfg)

		assert.Nil(t, database)
		assert.ErrorIs(t, err, db.ErrInvalidConfig)
		assert.NotContains(t, err.Error(), "secret")
	})
}

func TestInitMongo_RequiresCredentials(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "Error: URI without credentials", uri: "mongodb://127.0.0.1:1/?replicaSet=rs0&serverSelectionTimeoutMS=50"},
		{name: "Edge: username without password", uri: "mongodb://root@127.0.0.1:1/?replicaSet=rs0&serverSelectionTimeoutMS=50"},
		{name: "Edge: empty password", uri: "mongodb://root:@127.0.0.1:1/?replicaSet=rs0&serverSelectionTimeoutMS=50"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URI", tc.uri)
			cfg := env.NewEnv(logger.NewLogger(slog.LevelError))

			database, err := db.InitMongo(cfg)

			assert.Nil(t, database)
			assert.ErrorIs(t, err, db.ErrMissingCredentials)
			assert.ErrorIs(t, err, db.ErrInvalidConfig)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}
