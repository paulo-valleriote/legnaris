package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"legnaris/backend/internal/sheet"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

func main() {
	ctx := context.Background()
	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	database, err := db.InitMongo(cfg)
	if err != nil {
		log.Error(ctx, "database initialization failed", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
	defer closeDatabase(ctx, log, database, cfg.Variables.DatabaseCloseTimeout)

	mdb := database.Client.Database(cfg.Variables.DatabaseName)
	os.Exit(run(ctx, os.Stdout, func(ctx context.Context) (sheet.MigrationReport, error) {
		return sheet.MigrateKinds(ctx, mdb)
	}))
}

func closeDatabase(ctx context.Context, log logger.StructuredLogger, database *db.Database, timeout time.Duration) {
	closeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := database.Close(closeCtx); err != nil {
		log.Error(ctx, "error while disconnecting from mongodb", logger.Attribute{Key: "error", Value: err.Error()})
	}
}

func run(ctx context.Context, out io.Writer, migrate func(context.Context) (sheet.MigrationReport, error)) int {
	report, err := migrate(ctx)
	if err != nil {
		printLine(out, "migration failed: %v\n", err)
		return 1
	}
	printLine(out, "sheets backfilled to kind character: %d\n", report.BackfilledSheets)
	printLine(out, "counters rebuilt: %d\n", report.Counters)
	return 0
}

func printLine(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}
