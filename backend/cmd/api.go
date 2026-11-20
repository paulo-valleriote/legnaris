package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/engine"
	eventsamqp "legnaris/backend/internal/events/amqp"
	"legnaris/backend/internal/funnel"
	generationamqp "legnaris/backend/internal/generation/amqp"
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
		log.Error(ctx, "application terminated with error", logger.Attribute{Key: logErrorKey, Value: err.Error()})
		stop()
		os.Exit(1)
	}
}

const logErrorKey = "error"

func startFunnelRefresher(ctx context.Context, refresher *funnel.Refresher, log logger.StructuredLogger) func() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := refresher.Run(ctx); err != nil {
			log.Error(ctx, "funnel stats refresher stopped", logger.Attribute{Key: logErrorKey, Value: err.Error()})
		}
	}()
	return wg.Wait
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

	funnelRefresher := funnel.NewRefresher(
		funnel.NewMongoStore(database.Client.Database(cfg.Variables.DatabaseName)),
		httpMetrics,
		log,
		funnel.RefreshInterval,
	)
	defer startFunnelRefresher(ctx, funnelRefresher, log)()

	eventPublisher := eventsamqp.New(eventsamqp.Config{
		URL:           cfg.Variables.RabbitMQURL,
		Recorder:      httpMetrics,
		Logger:        log,
		RetryInterval: eventsamqp.DefaultRetryInterval,
	})
	defer func() {
		drainCtx, cancel := context.WithTimeout(context.Background(), eventsamqp.DefaultDrainTimeout)
		defer cancel()
		eventPublisher.Shutdown(drainCtx)
	}()

	generationPublisher := generationamqp.New(generationamqp.Config{URL: cfg.Variables.RabbitMQURL})
	defer func() {
		if err := generationPublisher.Close(); err != nil {
			log.Error(ctx, "error while closing the generation publisher", logger.Attribute{Key: logErrorKey, Value: err.Error()})
		}
	}()

	modules, err := app.NewModules(database, registry, cfg, httpMetrics, httpMetrics, eventPublisher,
		app.WithGenerationPublisher(generationPublisher),
		app.WithGenerationObserver(httpMetrics),
	)
	if err != nil {
		return fmt.Errorf("build modules: %w", err)
	}

	defer func() {
		if err := modules.Close(); err != nil {
			log.Error(ctx, "error while closing the rate limit store", logger.Attribute{Key: logErrorKey, Value: err.Error()})
		}
	}()

	router := app.NewRouter(cfg, modules, httpMetrics)

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", ":"+cfg.Variables.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Info(ctx, "http server listening", logger.Attribute{Key: "address", Value: listener.Addr().String()})
	return serve(ctx, router, listener, cfg.Variables.ShutdownTimeout)
}

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
