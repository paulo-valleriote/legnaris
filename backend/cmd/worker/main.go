// Command worker is the background process. It delivers the queued OTP messages
// from RabbitMQ, renders and sends each through the configured provider, retries
// transient failures through the broker's delay tiers and parks the ones that can
// never succeed. When GENERATION_ENABLED is set it also consumes character
// generation jobs (LGN-230), in the same process. It is a separate binary, so a broker or
// provider outage never reaches the HTTP API, and it needs no database: the
// code's lifetime travels on the message (O2, WKR-01, WKR-19).
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
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

// drainMargin is added to the send timeout to size the wait for in-flight
// messages on stop, so a send that is about to time out can still finish.
const drainMargin = 5 * time.Second

// brokerClient is the broker adapter the worker runs against: it is the
// delivery source and the republisher, declares the topology before the first
// delivery, and can be closed.
type brokerClient interface {
	worker.DeliverySource
	worker.Republisher
	DeclareTopology() error
	Close() error
}

// dependencies are the process's wiring seams, overridable in tests so the
// startup logic is exercised without a broker or a provider (WKR-19, WKR-27).
type dependencies struct {
	newSender    func(cfg env.Env) (email.Sender, error)
	dialBroker   func(cfg env.Env) (brokerClient, error)
	serveMetrics func(addr string, handler http.Handler) (func(ctx context.Context) error, error)
	// newGeneration builds the generation consumer. It is called only when
	// GENERATION_ENABLED is set, and its failure stops the process before consuming.
	newGeneration func(cfg env.Env) (generationRunner, func(), error)
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

// run validates the broker and provider settings, opens the broker, declares the
// topology, serves the metrics listener and consumes until ctx is cancelled.
// Everything that can fail does so before the first delivery is taken
// (WKR-01, WKR-18, WKR-19, WKR-20).
func run(ctx context.Context, cfg env.Env, deps dependencies) error {
	if err := amqp.ValidateURL(cfg.Variables.RabbitMQURL); err != nil {
		return err
	}

	sender, err := deps.newSender(cfg)
	if err != nil {
		return err
	}

	var generation generationRunner
	if cfg.Variables.GenerationEnabled {
		runner, cleanup, err := deps.newGeneration(cfg)
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

	workerMetrics := metrics.New()
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

// workerConfig maps the optional configuration onto the worker's knobs; every
// value has a documented default (WKR-14, WKR-16).
func workerConfig(cfg env.Env) worker.Config {
	vars := cfg.Variables
	return worker.Config{
		Prefetch:     vars.EmailWorkerPrefetch,
		MaxAttempts:  vars.EmailWorkerMaxAttempts,
		DrainTimeout: vars.EmailSendTimeout + drainMargin,
	}
}

// senderFromConfig builds the sending port the provider setting selects.
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

// serveMetrics starts a plain http server on addr for the worker's own metrics
// listener and returns its shutdown function (WKR-20).
func serveMetrics(addr string, handler http.Handler) (func(ctx context.Context) error, error) {
	listener, err := net.Listen("tcp", addr)
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

// defaultDependencies wires the real RabbitMQ broker, provider and metrics
// listener.
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
