// Command migrate-sheet-kinds moves a database created before Sheet kinds to the
// kinds model: legacy Sheets become characters, the per-owner counters are
// rebuilt and the one-sheet-per-user index is replaced (LGN-183). It is a
// one-shot operator tool; the API boot path never imports it.
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

// closeDatabase releases the connection, logging a failure instead of hiding it.
func closeDatabase(ctx context.Context, log logger.StructuredLogger, database *db.Database, timeout time.Duration) {
	closeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := database.Close(closeCtx); err != nil {
		log.Error(ctx, "error while disconnecting from mongodb", logger.Attribute{Key: "error", Value: err.Error()})
	}
}

// run executes the migration and prints the report. It returns the process exit
// code, so the command body stays testable without a database.
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

// printLine writes one report line. A failed write to stdout cannot change the
// outcome of an already-decided run, so the error is deliberately ignored.
func printLine(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}
