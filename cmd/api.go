package main

import (
	"context"
	"log/slog"

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
	env := env.NewEnv(log)

	database, err := db.InitMongo(env)
	if err != nil {
		log.Fatal(ctx, "db initialization error: %w", logger.Attribute{Key: "err", Value: err.Error()})
	}

	defer func(ctx context.Context) {
		err := database.Close(ctx)
		log.Fatal(
			ctx,
			"fatal error while disconnecting from mongodb",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}(ctx)

	formulaEngine := engine.NewFormulaEngine()

	registry, err := template.BuildRegistry(formulaEngine)
	if err != nil {
		log.Fatal(ctx, "error building template registry", logger.Attribute{Key: "err", Value: err.Error()})
	}

	modules := app.NewModules(database, registry)

	e := app.NewRouter(env, modules)
	if err := e.Start(":" + env.Variables.Port); err != nil {
		log.Error(
			ctx,
			"failed to start server",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}
}
