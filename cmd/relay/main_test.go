package main

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"legnaris/internal/email"
	"legnaris/internal/email/amqp"
	"legnaris/internal/email/outbox"
	outboxmocks "legnaris/internal/email/outbox/mocks"
	"legnaris/shared/env"
	"legnaris/shared/logger"

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

// TestRun_FailsFastOnBadBrokerURL proves a blank or unusable broker URL stops the
// relay with the typed error before the database is opened or anything is polled
// (EML-15, EML-19).
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
			deps := dependencies{
				openDatabase: func(env.Env) (databaseHandle, error) {
					opened = true
					return &fakeDatabase{}, nil
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
		})
	}
}

// TestValidateBrokerURL_AcceptsValidURLs guards against over-rejecting a valid
// AMQP URI (EML-15).
func TestValidateBrokerURL_AcceptsValidURLs(t *testing.T) {
	for _, raw := range []string{
		"amqp://guest:guest@rabbitmq:5672/",
		"amqps://user:pass@broker.example:5671/vhost",
	} {
		assert.NoErrorf(t, validateBrokerURL(raw), "URL %q must be accepted", raw)
	}
}

// TestRun_CancelledContextDrainsAndReturns proves a cancelled context lets the
// in-flight batch finish and returns nil, closing the broker and database
// (EML-05).
func TestRun_CancelledContextDrainsAndReturns(t *testing.T) {
	cfg := testEnv()

	repository := outboxmocks.NewMockRepository(t)
	repository.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()
	repository.EXPECT().Backlog(mock.Anything).Return(0, nil).Once()

	broker := &fakeBroker{}
	database := &fakeDatabase{}

	deps := dependencies{
		openDatabase: func(env.Env) (databaseHandle, error) { return database, nil },
		newRepository: func(env.Env, databaseHandle, logger.StructuredLogger) (outbox.Repository, error) {
			return repository, nil
		},
		dialBroker:   func(env.Env) (brokerClient, error) { return broker, nil },
		serveMetrics: noopServeMetrics,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, run(ctx, cfg, deps))

	assert.True(t, broker.declared, "the topology must be declared before publishing")
	assert.True(t, broker.closed, "the broker must be closed on shutdown")
	assert.True(t, database.closed, "the database must be closed on shutdown")
}
