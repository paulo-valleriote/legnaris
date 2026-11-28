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

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/amqp"
	"legnaris/backend/internal/email/factory"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

const drainMargin = 5 * time.Second

type brokerClient interface {
	worker.DeliverySource
	worker.Republisher
	DeclareTopology() error
	Close() error
}

type dependencies struct {
	newSender     func(cfg env.Env) (email.Sender, error)
	dialBroker    func(cfg env.Env) (brokerClient, error)
	serveMetrics  func(addr string, handler http.Handler) (func(ctx context.Context) error, error)
	newGeneration func(cfg env.Env, observer generation.WorkerObserver) (generationRunner, func(), error)
}

func main() {
	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, defaultDependencies()); err != nil {
		log.Error(ctx, "worker terminated with error", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg env.Env, deps dependencies) error {
	if err := amqp.ValidateURL(cfg.Variables.RabbitMQURL); err != nil {
		return err
	}

	sender, err := deps.newSender(cfg)
	if err != nil {
		return err
	}

	workerMetrics := metrics.New()

	var generation generationRunner
	if cfg.Variables.GenerationEnabled {
		runner, cleanup, err := deps.newGeneration(cfg, workerMetrics)
		if err != nil {
			return err
		}
		defer cleanup()
		generation = runner
	}

	renderer, err := email.NewRenderer()
	if err != nil {
		return fmt.Errorf("build email renderer: %w", err)
	}

	broker, err := deps.dialBroker(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = broker.Close() }()

	if err := broker.DeclareTopology(); err != nil {
		return err
	}

	shutdown, err := deps.serveMetrics(cfg.Variables.EmailWorkerMetricsAddr, workerMetrics.Handler())
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	delivery := worker.New(broker, broker, sender, renderer, workerMetrics, workerConfig(cfg), cfg.Logger)
	if generation == nil {
		return delivery.Run(ctx)
	}
	return runConcurrently(ctx, delivery.Run, generation.Run)
}

func workerConfig(cfg env.Env) worker.Config {
	vars := cfg.Variables
	return worker.Config{
		Prefetch:     vars.EmailWorkerPrefetch,
		MaxAttempts:  vars.EmailWorkerMaxAttempts,
		DrainTimeout: vars.EmailSendTimeout + drainMargin,
	}
}

func senderFromConfig(cfg env.Env) (email.Sender, error) {
	vars := cfg.Variables
	return factory.New(factory.Config{
		Provider: vars.EmailProvider,
		AppEnv:   vars.AppEnv,
		APIKey:   vars.ResendAPIKey,
		From:     vars.EmailFrom,
		Timeout:  vars.EmailSendTimeout,
		Logger:   cfg.Logger,
	})
}

func serveMetrics(addr string, handler http.Handler) (func(ctx context.Context) error, error) {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen metrics on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()

	return server.Shutdown, nil
}

func defaultDependencies() dependencies {
	return dependencies{
		newSender: senderFromConfig,
		dialBroker: func(cfg env.Env) (brokerClient, error) {
			broker, err := amqp.Dial(cfg.Variables.RabbitMQURL, amqp.Config{
				Prefetch: cfg.Variables.EmailWorkerPrefetch,
				RetryDelays: amqp.RetryDelays{
					First:  cfg.Variables.EmailRetryDelayFirst,
					Second: cfg.Variables.EmailRetryDelaySecond,
					Later:  cfg.Variables.EmailRetryDelayLater,
				},
				Logger: cfg.Logger,
			})
			if err != nil {
				return nil, err
			}
			return broker, nil
		},
		serveMetrics:  serveMetrics,
		newGeneration: buildGeneration,
	}
}
