// Command canonicalize-emails repairs legacy user documents whose stored address
// is not in canonical form. It is a one-shot operator tool: the API boot path
// never imports it (PRQ-11).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

const usageLine = "usage: canonicalize-emails [-apply]"

// printLine writes one report line. A failure to write stdout cannot change the
// outcome of an already-decided run, so the write error is deliberately ignored.
func printLine(out io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}

func main() {
	// Usage is resolved before any configuration or connection, so `-h` and a
	// bad flag answer with the documented usage instead of a database error.
	apply, ok := parseArgs(os.Args[1:], os.Stdout)
	if !ok {
		os.Exit(2)
	}

	ctx := context.Background()
	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	database, err := db.InitMongo(cfg)
	if err != nil {
		log.Error(ctx, "database initialization failed", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
		defer cancel()
		if err := database.Close(closeCtx); err != nil {
			log.Error(ctx, "error while disconnecting from mongodb", logger.Attribute{Key: "error", Value: err.Error()})
		}
	}()

	store := user.NewEmailMigrationStore(database.Client.Database(cfg.Variables.DatabaseName), log)
	os.Exit(execute(ctx, apply, os.Stdout, store))
}

// run is the testable command body: it parses the flags, prints the documented
// report and returns the process exit code. A dry run is the default, so an
// accidental invocation writes nothing (PRQ-11, SC-008).
func run(ctx context.Context, args []string, stdout io.Writer, store user.EmailMigrationStore) int {
	apply, ok := parseArgs(args, stdout)
	if !ok {
		return 2
	}
	return execute(ctx, apply, stdout, store)
}

// parseArgs resolves the only flag and the no-argument contract. A usage error
// prints the usage line and reports ok=false.
func parseArgs(args []string, stdout io.Writer) (bool, bool) {
	flags := flag.NewFlagSet("canonicalize-emails", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "write the planned changes")

	if err := flags.Parse(args); err != nil {
		printLine(stdout, "%s\n", usageLine)
		return false, false
	}
	if flags.NArg() > 0 {
		printLine(stdout, "%s\n", usageLine)
		return false, false
	}
	return *apply, true
}

// execute reads the stored identities, prints the report and performs the
// validated plan. A blocked plan and any store failure return exit code 1.
func execute(ctx context.Context, apply bool, stdout io.Writer, store user.EmailMigrationStore) int {
	rows, err := store.ListEmailIdentities(ctx)
	if err != nil {
		printLine(stdout, "error: list email identities: %v\n", err)
		return 1
	}

	plan := user.PlanEmailCanonicalization(rows)
	printLine(stdout, "scanned: %d\n", len(rows))
	for _, change := range plan.Changes {
		printLine(stdout, "change: %s %s -> %s\n", change.ID, change.From, change.To)
	}
	for _, collision := range plan.Collisions {
		printLine(stdout, "collision: %s %s\n", collision.Canonical, strings.Join(collision.IDs, " "))
	}
	for _, invalid := range plan.Invalid {
		printLine(stdout, "invalid: %s %s (%s)\n", invalid.ID, invalid.Raw, invalid.Reason)
	}

	if plan.Blocked() {
		return 1
	}

	written, err := plan.Apply(ctx, store, time.Now().UTC(), !apply)
	if err != nil {
		printLine(stdout, "error: %v\n", err)
		return 1
	}

	if apply {
		printLine(stdout, "written: %d\n", written)
	} else {
		printLine(stdout, "dry run: %d changes not written\n", len(plan.Changes))
	}
	return 0
}
