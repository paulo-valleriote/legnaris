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
	"legnaris/backend/internal/billing"
	billingstripe "legnaris/backend/internal/billing/stripe"
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

type brokerClient interface {
	Publish(ctx context.Context, msg email.Message) error
	DeclareTopology() error
	Close() error
}

type databaseHandle interface {
	Close(ctx context.Context) error
}

type eventBrokerClient interface {
	eventsrelay.Publisher
	Close() error
}

type generationBrokerClient interface {
	generation.Publisher
	Close() error
}

type dependencies struct {
	dialGenerationBroker   func(cfg env.Env) (generationBrokerClient, error)
	newGenerationRepublish func(cfg env.Env, database databaseHandle, publisher generation.Publisher) (generation.RepublishDeps, error)
	newGenerationReap      func(cfg env.Env, database databaseHandle) (generation.ReapDeps, error)
	newAccountDeletion     func(cfg env.Env, database databaseHandle, outboxRepo eventsoutbox.Repository) (accountdeletion.Deps, error)
	newBilling             func(cfg env.Env, database databaseHandle, outboxRepo eventsoutbox.Repository) (billing.ConsumerDeps, error)
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

func run(ctx context.Context, cfg env.Env, deps dependencies) error {
	vars := cfg.Variables

	if err := validateBrokerURL(vars.RabbitMQURL); err != nil {
		return err
	}

	database, err := deps.openDatabase(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer closeDatabase(cfg, database, cfg.Logger)

	if err := deps.ensureIndexes(ctx, cfg, database); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	broker, err := deps.dialBroker(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = broker.Close() }()

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

	reconciler, err := newReconciler(cfg, deps, database, eventRepository, relayMetrics)
	if err != nil {
		return err
	}
	reconcileDone := make(chan error, 1)
	go func() { reconcileDone <- reconciler.Run(ctx) }()

	publisher := relay.New(repository, broker, relayMetrics, relayConfig(cfg), cfg.Logger)
	emailErr := publisher.Run(ctx)
	sweepErr := <-sweepDone
	reconcileErr := <-reconcileDone
	return errors.Join(emailErr, sweepErr, reconcileErr)
}

func newReconciler(cfg env.Env, deps dependencies, database databaseHandle, eventRepository eventsoutbox.Repository, observer billing.ReconcileObserver) (*billing.Reconciler, error) {
	billingDeps, err := deps.newBilling(cfg, database, eventRepository)
	if err != nil {
		return nil, err
	}
	return billing.NewReconciler(billing.ReconcileDeps{
		ConsumerDeps: billingDeps,
		Claimer:      relayClaimerID() + "-reconcile",
		Observer:     observer,
		Log:          cfg.Logger,
	}), nil
}

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
	billingDeps, err := deps.newBilling(cfg, database, eventRepository)
	if err != nil {
		return nil, err
	}
	queues, err := sweepQueues(cfg, eventRepository, eventBroker, generationDeps, reapDeps, deletionDeps, billingDeps)
	if err != nil {
		return nil, err
	}
	return sweeper.New(queues, relayClaimerID(), observer, cfg.Logger)
}

const eventsOutboxInterval = 30 * time.Second

const generationRepublishAttemptCap = 5

func sweepQueues(cfg env.Env, eventRepository eventsoutbox.Repository, eventBroker eventsrelay.Publisher, generationDeps generation.RepublishDeps, reapDeps generation.ReapDeps, deletionDeps accountdeletion.Deps, billingDeps billing.ConsumerDeps) ([]sweeper.Queue, error) {
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
	billingQueue, err := billing.NewConsumerQueue(billingDeps, vars.EmailRelayMaxAttempts, vars.EmailRelayBatch)
	if err != nil {
		return nil, err
	}
	return []sweeper.Queue{eventQueue, generationQueue, reapQueue, deletionQueue, billingQueue}, nil
}

func validateBrokerURL(raw string) error {
	return amqp.ValidateURL(raw)
}

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
				Billing: billing.NewDeletion(billing.DeletionDeps{
					Provider: newRelayBillingProvider(cfg.Variables),
					Ledger:   billing.NewMongoStore(mdb),
				}),
			}, nil
		},
		newBilling: func(cfg env.Env, database databaseHandle, outboxRepo eventsoutbox.Repository) (billing.ConsumerDeps, error) {
			mongo, ok := database.(*db.Database)
			if !ok {
				return billing.ConsumerDeps{}, fmt.Errorf("unexpected database handle type %T", database)
			}
			mdb := mongo.Client.Database(cfg.Variables.DatabaseName)
			return billing.ConsumerDeps{
				Provider: newRelayBillingProvider(cfg.Variables),
				Ledger:   billing.NewMongoStore(mdb),
				Tx:       db.NewTransactor(mongo.Client, nil),
				Outbox:   outboxRepo,
				Now:      time.Now,
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

func newRelayBillingProvider(vars env.Variables) billing.Provider {
	if vars.StripeSecretKey == "" {
		return billing.UnconfiguredProvider{}
	}
	provider, err := billingstripe.New(billingstripe.Config{SecretKey: vars.StripeSecretKey, WebhookSecret: vars.StripeWebhookSecret})
	if err != nil {
		return billing.UnconfiguredProvider{}
	}
	return provider
}
