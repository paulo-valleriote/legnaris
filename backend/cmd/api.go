package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

func main() {
	ctx := context.Background()
	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	if err := run(ctx, log, cfg); err != nil {
		log.Error(ctx, "application terminated with error", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
}

func run(ctx context.Context, log logger.StructuredLogger, cfg env.Env) error {
	database, err := db.InitMongo(cfg)
	if err != nil {
		return fmt.Errorf("db initialization error: %w", err)
	}

	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
		defer cancel()
		if err := database.Close(closeCtx); err != nil {
			log.Error(
				ctx,
				"error while disconnecting from mongodb",
				logger.Attribute{Key: "error", Value: err.Error()},
			)
		}
	}()

	if err := db.EnsureIndexesWithRetention(ctx, database.Client.Database(cfg.Variables.DatabaseName), cfg.Variables.EmailOutboxRetention); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	formulaEngine := engine.NewFormulaEngine()

	registry, err := template.BuildRegistry(formulaEngine)
	if err != nil {
		return fmt.Errorf("build template registry: %w", err)
	}

	httpMetrics := metrics.New()

	modules, err := app.NewModules(database, registry, cfg, httpMetrics, httpMetrics)
	if err != nil {
		return fmt.Errorf("build modules: %w", err)
	}

	router := app.NewRouter(cfg, modules, httpMetrics)
	if err := router.Start(":" + cfg.Variables.Port); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}
