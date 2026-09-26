package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
)

func main() {
	ctx := context.Background()
	log := logger.NewLogger(slog.LevelInfo)
	applicationEnv := env.NewEnv(log)

	if err := run(ctx, log, applicationEnv); err != nil {
		log.Error(ctx, "application terminated with error", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
}

func run(ctx context.Context, log logger.StructuredLogger, applicationEnv env.Env) error {
	database, err := db.InitMongo(applicationEnv)
	if err != nil {
		return fmt.Errorf("db initialization error: %w", err)
	}

	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), applicationEnv.Variables.DatabaseCloseTimeout)
		defer cancel()
		if err := database.Close(closeCtx); err != nil {
			log.Error(
				ctx,
				"error while disconnecting from mongodb",
				logger.Attribute{Key: "error", Value: err.Error()},
			)
		}
	}()

	formulaEngine := engine.NewFormulaEngine()

	registry, err := template.BuildRegistry(formulaEngine)
	if err != nil {
		return fmt.Errorf("build template registry: %w", err)
	}

	modules := app.NewModules(database, registry, applicationEnv.Variables.DatabaseName)

	e := app.NewRouter(applicationEnv, modules)
	if err := e.Start(":" + applicationEnv.Variables.Port); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}
