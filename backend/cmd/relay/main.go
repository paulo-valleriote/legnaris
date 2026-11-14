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

	"errors"

	"legnaris/backend/internal/accountdeletion"
	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/amqp"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/internal/email/relay"
	eventsamqp "legnaris/backend/internal/events/amqp"
	eventsoutbox "legnaris/backend/internal/events/outbox"
	eventsrelay "legnaris/backend/internal/events/relay"
	"legnaris/backend/internal/generation"
	generationamqp "legnaris/backend/internal/generation/amqp"
	"legnaris/backend/internal/platform/metrics"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/internal/user"
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

// eventBrokerClient is the broker adapter the event relay publishes through.
type eventBrokerClient interface {
	eventsrelay.Publisher
	Close() error
}

// generationBrokerClient is the broker adapter the generation republish sweep
// publishes through.
type generationBrokerClient interface {
	generation.Publisher
	Close() error
}

// dependencies are the process's wiring seams, overridable in tests so the
// startup logic is exercised without Mongo or a broker (EML-15).
type dependencies struct {
	dialGenerationBroker   func(cfg env.Env) (generationBrokerClient, error)
	newGenerationRepublish func(cfg env.Env, database databaseHandle, publisher generation.Publisher) (generation.RepublishDeps, error)
	newGenerationReap      func(cfg env.Env, database databaseHandle) (generation.ReapDeps, error)
	newAccountDeletion     func(cfg env.Env, database databaseHandle, outboxRepo eventsoutbox.Repository) (accountdeletion.Deps, error)
	openDatabase           func(cfg env.Env) (databaseHandle, error)
	ensureIndexes          func(ctx context.Context, cfg env.Env, database databaseHandle) error
	newRepository          func(cfg env.Env, database databaseHandle, log logger.StructuredLogger) (outbox.Repository, error)
	dialBroker             func(cfg env.Env) (brokerClient, error)
	dialEventBroker        func(cfg env.Env) (eventBrokerClient, error)
	newEventRepository     func(cfg env.Env, database databaseHandle, log logger.StructuredLogger) (eventsoutbox.Repository, error)
	serveMetrics           func(addr string, handler http.Handler) (func(ctx context.Context) error, error)
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

	eventBroker, err := deps.dialEventBroker(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = eventBroker.Close() }()

	eventRepository, err := deps.newEventRepository(cfg, database, cfg.Logger)
	if err != nil {
		return err
	}
	generationBroker, err := deps.dialGenerationBroker(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = generationBroker.Close() }()

	sweep, err := newSweeper(cfg, deps, database, eventRepository, eventBroker, generationBroker, relayMetrics)
	if err != nil {
		return err
	}

	sweepDone := make(chan error, 1)
	go func() { sweepDone <- sweep.Run(ctx) }()

	publisher := relay.New(repository, broker, relayMetrics, relayConfig(cfg), cfg.Logger)
	emailErr := publisher.Run(ctx)
	sweepErr := <-sweepDone
	return errors.Join(emailErr, sweepErr)
}

// newSweeper wires every sweeper queue from the opened collaborators and builds the
// sweeper that drives them (ADR 0013). It only constructs: run starts it.
func newSweeper(cfg env.Env, deps dependencies, database databaseHandle, eventRepository eventsoutbox.Repository, eventBroker eventsrelay.Publisher, generationBroker generation.Publisher, observer sweeper.Observer) (*sweeper.Sweeper, error) {
	generationDeps, err := deps.newGenerationRepublish(cfg, database, generationBroker)
	if err != nil {
		return nil, err
	}
	reapDeps, err := deps.newGenerationReap(cfg, database)
	if err != nil {
		return nil, err
	}
	deletionDeps, err := deps.newAccountDeletion(cfg, database, eventRepository)
	if err != nil {
		return nil, err
	}
	queues, err := sweepQueues(cfg, eventRepository, eventBroker, generationDeps, reapDeps, deletionDeps)
	if err != nil {
		return nil, err
	}
	return sweeper.New(queues, relayClaimerID(), observer, cfg.Logger)
}

// eventsOutboxInterval is how often the sweeper claims the event outbox.
const eventsOutboxInterval = 5 * time.Minute

// generationRepublishAttemptCap is how many publish attempts a queued generation
// gets before the sweep gives up on it and refunds the quota (LGN-186).
const generationRepublishAttemptCap = 5

// sweepQueues is the sweeper's queue table: one entry per queue, and the only
// place a queue is registered (ADR 0013). The event outbox shares the email
// relay's batch, confirm timeout and attempt cap. The generation republish and the
// generation lease reaper run every 10 s, because a player waits on the screen
// (LGN-186, LGN-230). The deleting-accounts queue removes an account and its data
// in one transaction, and writes its user.deleted event to the same outbox (LGN-190).
func sweepQueues(cfg env.Env, eventRepository eventsoutbox.Repository, eventBroker eventsrelay.Publisher, generationDeps generation.RepublishDeps, reapDeps generation.ReapDeps, deletionDeps accountdeletion.Deps) ([]sweeper.Queue, error) {
	vars := cfg.Variables
	eventQueue, err := eventsrelay.NewQueue(eventRepository, eventBroker, eventsrelay.Config{
		Interval:       eventsOutboxInterval,
		Batch:          vars.EmailRelayBatch,
		ConfirmTimeout: vars.EmailRelayConfirmTimeout,
		AttemptCap:     vars.EmailRelayMaxAttempts,
	})
	if err != nil {
		return nil, err
	}
	generationQueue, err := generation.NewRepublishQueue(generationDeps, generationRepublishAttemptCap, vars.EmailRelayBatch)
	if err != nil {
		return nil, err
	}
	reapQueue, err := generation.NewReapQueue(reapDeps, generationRepublishAttemptCap, vars.EmailRelayBatch)
	if err != nil {
		return nil, err
	}
	deletionQueue, err := accountdeletion.NewQueue(deletionDeps, vars.EmailRelayMaxAttempts, vars.EmailRelayBatch)
	if err != nil {
		return nil, err
	}
	return []sweeper.Queue{eventQueue, generationQueue, reapQueue, deletionQueue}, nil
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
		dialEventBroker: func(cfg env.Env) (eventBrokerClient, error) {
			return eventsamqp.NewRelayBroker(eventsamqp.RelayBrokerConfig{
				URL:            cfg.Variables.RabbitMQURL,
				ConfirmTimeout: cfg.Variables.EmailRelayConfirmTimeout,
			}), nil
		},
		newEventRepository: func(cfg env.Env, database databaseHandle, log logger.StructuredLogger) (eventsoutbox.Repository, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return nil, fmt.Errorf("unexpected database handle type %T", database)
			}
			return eventsoutbox.NewRepository(mongo.Client.Database(cfg.Variables.DatabaseName), log), nil
		},
		serveMetrics: serveMetrics,
		dialGenerationBroker: func(cfg env.Env) (generationBrokerClient, error) {
			return generationamqp.New(generationamqp.Config{
				URL:            cfg.Variables.RabbitMQURL,
				ConfirmTimeout: cfg.Variables.EmailRelayConfirmTimeout,
			}), nil
		},
		newGenerationRepublish: func(cfg env.Env, database databaseHandle, publisher generation.Publisher) (generation.RepublishDeps, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return generation.RepublishDeps{}, fmt.Errorf("unexpected database handle type %T", database)
			}
			mdb := mongo.Client.Database(cfg.Variables.DatabaseName)
			return generation.RepublishDeps{
				Store:     generation.NewMongoStore(mdb),
				Publisher: publisher,
				Quota:     quota.NewService(quota.NewMongoStore(mdb), nil),
				Tx:        db.NewTransactor(mongo.Client, nil),
				Now:       time.Now,
			}, nil
		},
		newAccountDeletion: func(cfg env.Env, database databaseHandle, outboxRepo eventsoutbox.Repository) (accountdeletion.Deps, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return accountdeletion.Deps{}, fmt.Errorf("unexpected database handle type %T", database)
			}
			mdb := mongo.Client.Database(cfg.Variables.DatabaseName)
			return accountdeletion.Deps{
				Store:   user.NewDeletionStore(mdb, cfg.Logger),
				Cascade: accountdeletion.NewCascade(mdb, outboxRepo, time.Now),
				Tx:      db.NewTransactor(mongo.Client, nil),
			}, nil
		},
		newGenerationReap: func(cfg env.Env, database databaseHandle) (generation.ReapDeps, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return generation.ReapDeps{}, fmt.Errorf("unexpected database handle type %T", database)
			}
			mdb := mongo.Client.Database(cfg.Variables.DatabaseName)
			return generation.ReapDeps{
				Store: generation.NewMongoStore(mdb),
				Quota: quota.NewService(quota.NewMongoStore(mdb), nil),
				Tx:    db.NewTransactor(mongo.Client, nil),
			}, nil
		},
	}
}
