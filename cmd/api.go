package main

import (
	"context"
	"encoding/json"
	"fmt"
	"legnaris/internal/app"
	"legnaris/internal/engine"
	"legnaris/internal/sheet/template"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"log"
	"log/slog"
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

	controllers := app.NewControllers(database)

	e := app.NewRouter(env, controllers)
	if err := e.Start(":8080"); err != nil {
		log.Error(
			ctx,
			"failed to start server",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}
}

// EXAMPLE
func initFormulaEngine() {
	// PHASE 1: Server Startup
	formulaEngine := engine.NewFormulaEngine()

	mockMongoTemplate := template.FantasyV1Seed

	fantasyDAG, err := template.BuildDAG("fantasy_v1", mockMongoTemplate, formulaEngine)
	if err != nil {
		log.Fatalf("Fatal error while startuping: %v", err)
	}
	fmt.Println("Server initialized. DAG builded up and formulas are cached")

	// PHASE 2: DURING REQUESTS (HTTP PATCH)

	userSheetData := map[string]any{
		"stats.strength":         10.0,
		"modifiers.strength_mod": 0.0,
		"base_attack_bonus":      2.0,
		"derived.melee_damage":   2.0,
	}

	userPatchRequest := map[string]any{
		"stats.strength": 15.0,
	}

	fmt.Println("\n--- Recalc ---")
	cascadingEffects, err := fantasyDAG.Recalculate(userSheetData, userPatchRequest)
	if err != nil {
		log.Printf("Erro HTTP 500: falha no recalculo: %v", err)
		return
	}

	responseJSON, _ := json.MarshalIndent(cascadingEffects, "", "  ")
	fmt.Printf("Efeitos Cascata (Para UX do Frontend e $set no Mongo):\n%s\n", responseJSON)
}
