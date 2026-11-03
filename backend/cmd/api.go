package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/platform/health"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/labstack/echo/v5"
)

const healthcheckTimeout = 3 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) == 3 && os.Args[1] == "healthcheck" {
		if err := health.Probe(ctx, os.Args[2], healthcheckTimeout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	go func() {
		<-ctx.Done()
		stop()
	}()

	if err := run(ctx, log, cfg); err != nil {
		log.Error(ctx, "application terminated with error", logger.Attribute{Key: "error", Value: err.Error()})
		stop()
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

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", ":"+cfg.Variables.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Info(ctx, "http server listening", logger.Attribute{Key: "address", Value: listener.Addr().String()})
	return serve(ctx, router, listener, cfg.Variables.ShutdownTimeout)
}

// serve runs handler on ln until ctx is cancelled, then stops accepting
// connections and waits up to timeout for in-flight requests. When the drain
// exceeds timeout the remaining connections are closed forcibly and an error
// is returned so the process exits non-zero.
func serve(ctx context.Context, handler http.Handler, ln net.Listener, timeout time.Duration) error {
	var (
		server      *http.Server
		shutdownErr error
	)

	starter := echo.StartConfig{
		Listener:        ln,
		GracefulTimeout: timeout,
		HideBanner:      true,
		HidePort:        true,
		BeforeServeFunc: func(s *http.Server) error {
			server = s
			return nil
		},
		OnShutdownError: func(err error) {
			shutdownErr = err
			_ = server.Close()
		},
	}

	if err := starter.Start(ctx, handler); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	if shutdownErr != nil {
		return fmt.Errorf("graceful shutdown: %w", shutdownErr)
	}
	return nil
}
