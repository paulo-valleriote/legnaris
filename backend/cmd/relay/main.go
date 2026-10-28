// Command relay is the email publication process. It polls the durable outbox,
// claims a batch with a lease, publishes each intention to RabbitMQ with
// publisher confirms, and applies a bounded backoff on failure. It is a
// separate binary, so a broker outage can never reach the HTTP API and the API
// never depends on the broker to start (O2, EML-05, EML-15, EML-16).
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
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/internal/email/relay"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"
)

// brokerClient is the broker adapter the relay runs against: it publishes
// through the relay Publisher port, declares the topology before the first
// publish, and can be closed.
type brokerClient interface {
	Publish(ctx context.Context, msg email.Message) error
	DeclareTopology() error
	Close() error
}

// databaseHandle is the open database the relay closes on shutdown.
type databaseHandle interface {
	Close(ctx context.Context) error
}

// dependencies are the process's wiring seams, overridable in tests so the
// startup logic is exercised without Mongo or a broker (EML-15).
type dependencies struct {
	openDatabase  func(cfg env.Env) (databaseHandle, error)
	ensureIndexes func(ctx context.Context, cfg env.Env, database databaseHandle) error
	newRepository func(cfg env.Env, database databaseHandle, log logger.StructuredLogger) (outbox.Repository, error)
	dialBroker    func(cfg env.Env) (brokerClient, error)
	serveMetrics  func(addr string, handler http.Handler) (func(ctx context.Context) error, error)
}

func main() {
	log := logger.NewLogger(slog.LevelInfo)
	cfg := env.NewEnv(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, defaultDependencies()); err != nil {
		log.Error(ctx, "relay terminated with error", logger.Attribute{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
}

// run validates the broker settings, opens the database and the broker, declares
// the topology, serves the metrics listener and runs the publication loop until
// ctx is cancelled (EML-05, EML-15, EML-16).
func run(ctx context.Context, cfg env.Env, deps dependencies) error {
	vars := cfg.Variables

	// The broker is validated here, by the one process that needs it: a blank or
	// unusable URL fails fast before anything is opened or polled (EML-15).
	if err := validateBrokerURL(vars.RabbitMQURL); err != nil {
		return err
	}

	database, err := deps.openDatabase(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer closeDatabase(cfg, database, cfg.Logger)

	// The relay is self-sufficient: it ensures the outbox indexes itself, so a
	// relay started before (or without) the API still has the claim index and
	// the retention TTL (EML-17).
	if err := deps.ensureIndexes(ctx, cfg, database); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	broker, err := deps.dialBroker(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = broker.Close() }()

	// The topology exists before the first publish; a declaration failure stops
	// startup (EML-06).
	if err := broker.DeclareTopology(); err != nil {
		return err
	}

	repository, err := deps.newRepository(cfg, database, cfg.Logger)
	if err != nil {
		return err
	}

	relayMetrics := metrics.New()
	shutdown, err := deps.serveMetrics(vars.EmailRelayMetricsAddr, relayMetrics.Handler())
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	publisher := relay.New(repository, broker, relayMetrics, relayConfig(cfg), cfg.Logger)
	return publisher.Run(ctx)
}

// validateBrokerURL rejects an absent or unusable broker URL with the typed
// startup error, so the relay never begins polling without a broker (EML-15).
func validateBrokerURL(raw string) error {
	return amqp.ValidateURL(raw)
}

// relayConfig maps the optional configuration onto the relay's knobs; every
// value has a documented default (EML-04, EML-05).
func relayConfig(cfg env.Env) relay.Config {
	vars := cfg.Variables
	return relay.Config{
		PollInterval:   vars.EmailRelayPollInterval,
		Lease:          vars.EmailRelayLease,
		Batch:          vars.EmailRelayBatch,
		ConfirmTimeout: vars.EmailRelayConfirmTimeout,
		MaxAttempts:    vars.EmailRelayMaxAttempts,
		ClaimerID:      relayClaimerID(),
	}
}

// relayClaimerID identifies this relay instance in a claim's lease, so an
// operator can tell which process held a row (EML-03).
func relayClaimerID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "relay"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

func closeDatabase(cfg env.Env, database databaseHandle, log logger.StructuredLogger) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Variables.DatabaseCloseTimeout)
	defer cancel()
	if err := database.Close(ctx); err != nil {
		log.Error(ctx, "error while disconnecting from mongodb", logger.Attribute{Key: "error", Value: err.Error()})
	}
}

// serveMetrics starts a plain http server on addr for the relay's metrics
// listener; it returns the shutdown function (EML-16, EML-23).
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

// defaultDependencies wires the real Mongo, RabbitMQ and metrics listener.
func defaultDependencies() dependencies {
	return dependencies{
		openDatabase: func(cfg env.Env) (databaseHandle, error) {
			return db.InitMongo(cfg)
		},
		ensureIndexes: func(ctx context.Context, cfg env.Env, database databaseHandle) error {
			mongo, ok := database.(*db.Database)
			if !ok {
				return fmt.Errorf("unexpected database handle type %T", database)
			}
			return db.EnsureIndexesWithRetention(ctx, mongo.Client.Database(cfg.Variables.DatabaseName), cfg.Variables.EmailOutboxRetention)
		},
		newRepository: func(cfg env.Env, database databaseHandle, log logger.StructuredLogger) (outbox.Repository, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return nil, fmt.Errorf("unexpected database handle type %T", database)
			}
			return outbox.NewRepository(mongo.Client.Database(cfg.Variables.DatabaseName), log), nil
		},
		dialBroker: func(cfg env.Env) (brokerClient, error) {
			broker, err := amqp.Dial(cfg.Variables.RabbitMQURL, amqp.Config{
				ConfirmTimeout: cfg.Variables.EmailRelayConfirmTimeout,
				Logger:         cfg.Logger,
			})
			if err != nil {
				return nil, err
			}
			return broker, nil
		},
		serveMetrics: serveMetrics,
	}
}
