package main

import (
	"context"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/accountdeletion"
	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/amqp"
	"legnaris/backend/internal/email/outbox"
	outboxmocks "legnaris/backend/internal/email/outbox/mocks"
	eventsoutbox "legnaris/backend/internal/events/outbox"
	eventsoutboxmocks "legnaris/backend/internal/events/outbox/mocks"
	"legnaris/backend/internal/generation"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeBroker struct {
	declared bool
	closed   bool
}

func (b *fakeBroker) Publish(context.Context, email.Message) error { return nil }
func (b *fakeBroker) DeclareTopology() error                       { b.declared = true; return nil }
func (b *fakeBroker) Close() error                                 { b.closed = true; return nil }

type fakeEventBroker struct {
	closed bool
}

func (b *fakeEventBroker) Publish(context.Context, eventsoutbox.Entry) error { return nil }
func (b *fakeEventBroker) Close() error                                      { b.closed = true; return nil }

type fakeDatabase struct {
	closed bool
}

func (d *fakeDatabase) Close(context.Context) error { d.closed = true; return nil }

func noopServeMetrics(string, http.Handler) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

func testEnv() env.Env {
	log := logger.NewLogger(slog.LevelError)
	cfg := env.NewEnv(log)
	cfg.Variables.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.Variables.DatabaseName = "legnaris"
	cfg.Variables.EmailRelayPollInterval = time.Hour
	return cfg
}

func TestRun_FailsFastOnBadBrokerURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "Error: empty URL", url: ""},
		{name: "Error: blank URL", url: "   "},
		{name: "Error: unparseable URL", url: "not-a-uri"},
		{name: "Error: a foreign scheme", url: "http://localhost:5672/"},
		{name: "Error: a missing host", url: "amqp://"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testEnv()
			cfg.Variables.RabbitMQURL = tc.url

			dialed := false
			opened := false
			indexesEnsured := false
			deps := dependencies{
				openDatabase: func(env.Env) (databaseHandle, error) {
					opened = true
					return &fakeDatabase{}, nil
				},
				ensureIndexes: func(context.Context, env.Env, databaseHandle) error {
					indexesEnsured = true
					return nil
				},
				newRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (outbox.Repository, error) {
					return outboxmocks.NewMockRepository(t), nil
				},
				dialBroker: func(env.Env) (brokerClient, error) {
					dialed = true
					return &fakeBroker{}, nil
				},
				serveMetrics: noopServeMetrics,
			}

			err := run(context.Background(), cfg, deps)

			require.Error(t, err)
			assert.ErrorIs(t, err, amqp.ErrBrokerUnavailable)
			assert.False(t, dialed, "the broker must not be dialed on a bad URL")
			assert.False(t, opened, "the database must not be opened on a bad URL")
			assert.False(t, indexesEnsured, "the indexes must not be ensured on a bad URL")
		})
	}
}

func TestValidateBrokerURL_AcceptsValidURLs(t *testing.T) {
	for _, raw := range []string{
		"amqp://guest:guest@rabbitmq:5672/",
		"amqps://user:pass@broker.example:5671/vhost",
	} {
		assert.NoErrorf(t, validateBrokerURL(raw), "URL %q must be accepted", raw)
	}
}

func TestRun_CancelledContextDrainsAndReturns(t *testing.T) {
	cfg := testEnv()

	repository := outboxmocks.NewMockRepository(t)
	repository.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()
	repository.EXPECT().Backlog(mock.Anything).Return(0, nil).Once()

	eventRepository := eventsoutboxmocks.NewMockRepository(t)
	eventRepository.EXPECT().OldestPending(mock.Anything).Return(time.Time{}, false, nil).Once()
	eventRepository.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	broker := &fakeBroker{}
	eventBroker := &fakeEventBroker{}
	database := &fakeDatabase{}
	indexesEnsured := false

	billingBuilt := false
	deps := dependencies{
		openDatabase: func(env.Env) (databaseHandle, error) { return database, nil },
		ensureIndexes: func(context.Context, env.Env, databaseHandle) error {
			indexesEnsured = true
			return nil
		},
		newRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (outbox.Repository, error) {
			return repository, nil
		},
		dialBroker: func(env.Env) (brokerClient, error) { return broker, nil },
		dialEventBroker: func(env.Env) (eventBrokerClient, error) {
			return eventBroker, nil
		},
		newEventRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (eventsoutbox.Repository, error) {
			return eventRepository, nil
		},
		serveMetrics: noopServeMetrics,
		dialGenerationBroker: func(env.Env) (generationBrokerClient, error) {
			return &fakeGenerationBroker{}, nil
		},
		newGenerationRepublish: func(_ env.Env, _ databaseHandle, publisher generation.Publisher) (generation.RepublishDeps, error) {
			deps := idleGenerationDeps(t)
			deps.Publisher = publisher
			return deps, nil
		},
		newGenerationReap: func(env.Env, databaseHandle) (generation.ReapDeps, error) {
			return idleReapDeps(t), nil
		},
		newAccountDeletion: func(env.Env, databaseHandle, eventsoutbox.Repository) (accountdeletion.Deps, error) {
			return idleDeletionDeps(t), nil
		},
		newBilling: func(_ env.Env, _ databaseHandle, outboxRepo eventsoutbox.Repository) (billing.ConsumerDeps, error) {
			billingBuilt = true
			ledger := billingtest.NewLedger()
			return billing.ConsumerDeps{
				Provider: billingtest.NewProvider(),
				Ledger:   ledger,
				Tx:       billingtest.NewTransactor(ledger, nil),
				Outbox:   outboxRepo,
				Now:      time.Now,
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, run(ctx, cfg, deps))

	assert.True(t, indexesEnsured, "the relay must ensure the outbox indexes")
	assert.True(t, billingBuilt, "the relay must register the billing consumer queue")
	assert.True(t, broker.declared, "the topology must be declared before publishing")
	assert.True(t, broker.closed, "the broker must be closed on shutdown")
	assert.True(t, eventBroker.closed, "the event broker must be closed on shutdown")
	assert.True(t, database.closed, "the database must be closed on shutdown")
}
