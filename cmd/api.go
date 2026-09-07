package main

import (
	"context"
	"legnaris/internal/app"
	"legnaris/shared/env"
	"legnaris/shared/logger"
)

func main() {
	log := logger.NewLogger()
	env := env.NewEnv(log)

	e := app.NewRouter(env)

	if err := e.Start(":8080"); err != nil {
		log.Error(
			context.Background(),
			"failed to start server",
			logger.Attribute{Key: "error", Value: err.Error()},
		)
	}
}
