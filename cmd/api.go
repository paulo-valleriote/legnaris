package main

import (
	"context"
	"legnaris/internal/app"
	"legnaris/shared/db"
	"legnaris/shared/env"
	"legnaris/shared/logger"
	"log/slog"
)

func main() {
	ctx := context.Background()
	log := logger.NewLogger(slog.LevelInfo)
	env := env.NewEnv(log)

	database, err := db.NewDatabase(env)
	if err != nil {
		log.Error(
			ctx,
			"failed to initialize database",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return
	}
	defer database.Close()

	err = database.Ping(ctx)
	if err != nil {
		log.Error(
			ctx,
			"failed to ping database",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return
	}

	e := app.NewRouter(env)
	if err := e.Start(":8080"); err != nil {
		log.Error(
			ctx,
			"failed to start server",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}
}
