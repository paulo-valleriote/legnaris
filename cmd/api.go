package main

import (
	"context"
	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"log/slog"
	"strings"
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
	mockMongoTemplate := template.FantasyV1Seed

	fantasyDAG, err := template.BuildDAG("fantasy_v1", mockMongoTemplate.Fields, formulaEngine)
	if err != nil {
		log.Fatal(ctx, "error generating dag", logger.Attribute{Key: "err", Value: err.Error()})
	}

	dagRegistry := map[string]*template.DAG{
		"fantasy_v1": fantasyDAG,
	}

	modules := app.NewModules(database, dagRegistry)

	seedIDs, err := seedTemplates(ctx, *modules, []template.Template{mockMongoTemplate})
	if err != nil {
		log.Fatal(ctx, "error while seed templates", logger.Attribute{Key: "err", Value: err.Error()})
	}
	log.Info(ctx, "templates succesfully seeded", logger.Attribute{
		Key:   "ids",
		Value: strings.Join(seedIDs, ", "),
	})

	e := app.NewRouter(env, modules)
	if err := e.Start(":8080"); err != nil {
		log.Error(
			ctx,
			"failed to start server",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}
}

func seedTemplates(ctx context.Context, modules app.Modules, templates []template.Template) ([]string, error) {
	return modules.Template.Service.SeedDefaultTemplates(ctx, templates)
}
