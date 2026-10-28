package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/amqp"
	"legnaris/backend/internal/email/emailtest"
	"legnaris/backend/internal/email/factory"
	"legnaris/backend/internal/email/resend"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/internal/email/worker/mocks"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeBroker is the broker the worker runs against: mockery doubles for the
// republisher, a feedable delivery stream and flags for the lifecycle.
type fakeBroker struct {
	*mocks.MockRepublisher
	stream     chan worker.Delivery
	declareErr error
	declared   atomic.Bool
	closed     bool
}

func newFakeBroker(t *testing.T) *fakeBroker {
	t.Helper()
	return &fakeBroker{MockRepublisher: mocks.NewMockRepublisher(t), stream: make(chan worker.Delivery, 4)}
}

func (b *fakeBroker) Consume(ctx context.Context) (<-chan worker.Delivery, error) {
	return b.stream, nil
}

func (b *fakeBroker) DeclareTopology() error { b.declared.Store(true); return b.declareErr }
func (b *fakeBroker) Close() error           { b.closed = true; return nil }

func testEnv() env.Env {
	cfg := env.NewEnv(logger.NewLogger(slog.LevelError))
	cfg.Variables.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.Variables.EmailProvider = factory.ProviderLog
	return cfg
}

// probe records which dependencies the process touched.
type probe struct {
	senderBuilt bool
	dialed      bool
	served      bool
	shutdown    bool
}

func (p *probe) deps(broker *fakeBroker, sender email.Sender) dependencies {
	return dependencies{
		newSender: func(cfg env.Env) (email.Sender, error) {
			p.senderBuilt = true
			if sender != nil {
				return sender, nil
			}
			return senderFromConfig(cfg)
		},
		dialBroker: func(env.Env) (brokerClient, error) {
			p.dialed = true
			return broker, nil
		},
		serveMetrics: func(string, http.Handler) (func(context.Context) error, error) {
			p.served = true
			return func(context.Context) error { p.shutdown = true; return nil }, nil
		},
	}
}

// TestRun_FailsFastOnUnusableConfiguration proves a configuration that cannot
// work stops the process with a typed error before it dials the broker or
// consumes anything (WKR-19).
func TestRun_FailsFastOnUnusableConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*env.Env)
		wantErr error
	}{
		{name: "Error: a blank broker URL", mutate: func(c *env.Env) { c.Variables.RabbitMQURL = "" }, wantErr: amqp.ErrBrokerUnavailable},
		{name: "Error: a whitespace broker URL", mutate: func(c *env.Env) { c.Variables.RabbitMQURL = "   " }, wantErr: amqp.ErrBrokerUnavailable},
		{name: "Error: a malformed broker URL", mutate: func(c *env.Env) { c.Variables.RabbitMQURL = "not-a-uri" }, wantErr: amqp.ErrBrokerUnavailable},
		{name: "Error: a foreign broker scheme", mutate: func(c *env.Env) { c.Variables.RabbitMQURL = "http://localhost:5672/" }, wantErr: amqp.ErrBrokerUnavailable},
		{name: "Error: an unknown provider", mutate: func(c *env.Env) { c.Variables.EmailProvider = "carrier-pigeon" }, wantErr: factory.ErrUnknownProvider},
		{name: "Error: a blank provider", mutate: func(c *env.Env) { c.Variables.EmailProvider = "" }, wantErr: factory.ErrUnknownProvider},
		{name: "Error: resend without an API key", mutate: func(c *env.Env) {
			c.Variables.EmailProvider = factory.ProviderResend
			c.Variables.EmailFrom = "no-reply@example.com"
		}, wantErr: resend.ErrMissingConfiguration},
		{name: "Error: resend without a sender address", mutate: func(c *env.Env) {
			c.Variables.EmailProvider = factory.ProviderResend
			c.Variables.ResendAPIKey = "re_test"
		}, wantErr: resend.ErrMissingConfiguration},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testEnv()
			tc.mutate(&cfg)
			p := &probe{}
			broker := newFakeBroker(t)

			err := run(context.Background(), cfg, p.deps(broker, nil))

			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.False(t, p.dialed, "the broker must not be dialed on a bad configuration")
			assert.False(t, p.served)
			assert.False(t, broker.declared.Load())
		})
	}
}

// TestRun_FailsBeforeConsuming proves a topology or listener failure stops the
// process and releases what was opened (WKR-19).
func TestRun_FailsBeforeConsuming(t *testing.T) {
	t.Run("Error: the broker cannot be dialed", func(t *testing.T) {
		p := &probe{}
		deps := p.deps(newFakeBroker(t), nil)
		deps.dialBroker = func(env.Env) (brokerClient, error) { return nil, amqp.ErrBrokerUnavailable }

		err := run(context.Background(), testEnv(), deps)

		assert.ErrorIs(t, err, amqp.ErrBrokerUnavailable)
		assert.False(t, p.served)
	})

	t.Run("Error: the topology cannot be declared", func(t *testing.T) {
		p := &probe{}
		broker := newFakeBroker(t)
		broker.declareErr = errors.New("channel closed")

		err := run(context.Background(), testEnv(), p.deps(broker, nil))

		require.Error(t, err)
		assert.True(t, broker.closed, "the broker is released")
		assert.False(t, p.served)
	})

	t.Run("Error: the metrics listener cannot bind", func(t *testing.T) {
		p := &probe{}
		broker := newFakeBroker(t)
		deps := p.deps(broker, nil)
		deps.serveMetrics = func(string, http.Handler) (func(context.Context) error, error) {
			return nil, errors.New("address in use")
		}

		err := run(context.Background(), testEnv(), deps)

		require.Error(t, err)
		assert.True(t, broker.closed, "the broker is released")
	})
}

// TestRun_DeliversAndStopsGracefully drives a message through the real wiring
// and proves a stop releases the broker and the listener (WKR-01, WKR-18,
// SC-008).
func TestRun_DeliversAndStopsGracefully(t *testing.T) {
	cfg := testEnv()
	p := &probe{}
	broker := newFakeBroker(t)
	sender := emailtest.NewRecorder()

	msg := emailtest.Message()
	id, err := idgen.New()
	require.NoError(t, err)
	msg.ID = id
	msg.CreatedAt = time.Now()
	handle := mocks.NewMockDeliveryHandle(t)
	handle.EXPECT().Ack().Return(nil).Once()
	broker.stream <- worker.Delivery{Body: mustMarshal(t, msg), Handle: handle}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, p.deps(broker, sender)) }()

	require.Eventually(t, func() bool { return sender.Count() == 1 }, 5*time.Second, 5*time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the process did not stop")
	}

	assert.True(t, broker.declared.Load(), "the topology is declared before consuming")
	assert.True(t, broker.closed, "the broker is released on stop")
	assert.True(t, p.shutdown, "the metrics listener is released on stop")
	assert.Equal(t, id, sender.Sent()[0].IdempotencyKey)
}

// TestRun_StopWithoutTraffic proves an idle worker stops cleanly.
func TestRun_StopWithoutTraffic(t *testing.T) {
	p := &probe{}
	broker := newFakeBroker(t)
	broker.MockRepublisher.EXPECT().PublishTo(mock.Anything, mock.Anything, mock.Anything).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, testEnv(), p.deps(broker, nil)) }()

	require.Eventually(t, func() bool { return broker.declared.Load() }, 5*time.Second, 5*time.Millisecond)
	cancel()

	require.NoError(t, <-done)
	assert.True(t, broker.closed)
}

func TestWorkerConfig_MapsTheSettings(t *testing.T) {
	cfg := testEnv()
	cfg.Variables.EmailWorkerPrefetch = 3
	cfg.Variables.EmailWorkerMaxAttempts = 2
	cfg.Variables.EmailSendTimeout = 4 * time.Second

	got := workerConfig(cfg)

	assert.Equal(t, 3, got.Prefetch)
	assert.Equal(t, 2, got.MaxAttempts)
	assert.Greater(t, got.DrainTimeout, 4*time.Second, "the drain budget outlasts one send")
}

func mustMarshal(t *testing.T, msg email.Message) []byte {
	t.Helper()
	encoded, err := json.Marshal(msg)
	require.NoError(t, err)
	return encoded
}
