package db

import (
	"context"
	"errors"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseLifecycle interface {
	Ping(ctx context.Context) error
	Close()
}

type Database struct {
	Pool   *pgxpool.Pool
	logger logger.StructuredLogger
}

func NewDatabase(env env.Env) (*Database, error) {
	ctx := context.Background()

	dsn := env.Variables.DatabaseURL
	if dsn == "" {
		env.Logger.Error(ctx, "database url isnt configured")
		return nil, errors.New("database url isnt configured")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		env.Logger.Error(ctx, "failed to parse database url")
		return nil, err
	}

	cfg.MaxConns = int32(env.Variables.DatabaseMaxConnections)
	cfg.MinConns = int32(env.Variables.DatabaseMinConnections)
	cfg.MaxConnLifetime = env.Variables.DatabaseConnLifetime
	cfg.HealthCheckPeriod = env.Variables.DatabaseCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		env.Logger.Error(ctx, "failed to create database connection pool")
		return nil, err
	}

	return &Database{
		Pool:   pool,
		logger: env.Logger,
	}, nil
}

func (d *Database) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}

func (d *Database) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := d.Pool.Ping(pingCtx); err != nil {
		d.logger.Error(ctx, "failed to ping database")
		return err
	}

	d.logger.Info(ctx, "database connection established successfully")
	return nil
}
