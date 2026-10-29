package amqp

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/worker"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ worker.DeliverySource = (*Broker)(nil)
	_ worker.Republisher    = (*Broker)(nil)
)

// recordingAcknowledger captures how a broker delivery is settled.
type recordingAcknowledger struct {
	acked    []uint64
	nacked   []uint64
	rejected []uint64
	ackErr   error
}

func (r *recordingAcknowledger) Ack(tag uint64, _ bool) error {
	r.acked = append(r.acked, tag)
	return r.ackErr
}

func (r *recordingAcknowledger) Nack(tag uint64, _, _ bool) error {
	r.nacked = append(r.nacked, tag)
	return nil
}

func (r *recordingAcknowledger) Reject(tag uint64, _ bool) error {
	r.rejected = append(r.rejected, tag)
	return nil
}

// newConsumingBroker builds a broker whose consumer gets a channel of its own.
func newConsumingBroker(t *testing.T, cfg Config) (*Broker, *fakeChannel, *fakeChannel, *fakeConnection) {
	t.Helper()

	publisher := &fakeChannel{}
	consumer := &fakeChannel{deliveries: make(chan amqp.Delivery, 4)}
	conn := &fakeConnection{channel: publisher, consumers: []*fakeChannel{consumer}}

	dial := func(string, amqp.Config) (connection, error) { return conn, nil }
	if cfg.ConfirmTimeout == 0 {
		cfg.ConfirmTimeout = time.Second
	}
	cfg.Logger = logger.NewLogger(slog.LevelError)

	broker, err := dialWith(dial, "amqp://guest:guest@localhost:5672/", cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })
	return broker, publisher, consumer, conn
}

func receive(t *testing.T, stream <-chan worker.Delivery) (worker.Delivery, bool) {
	t.Helper()

	select {
	case delivery, ok := <-stream:
		return delivery, ok
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the delivery stream")
		return worker.Delivery{}, false
	}
}

// TestBroker_ConsumeRegistersTheConsumer pins how the consumer is registered:
// on a channel of its own, prefetch first, manual acknowledgement (WKR-02,
// WKR-16).
func TestBroker_ConsumeRegistersTheConsumer(t *testing.T) {
	t.Run("Happy path: the work queue is consumed with manual acknowledgement", func(t *testing.T) {
		broker, publisher, consumer, _ := newConsumingBroker(t, Config{Prefetch: 7})

		_, err := broker.Consume(context.Background())
		require.NoError(t, err)

		assert.Equal(t, queueWork, consumer.consumeQueue)
		assert.False(t, consumer.consumeAutoAck, "acknowledgement must be manual")
		assert.True(t, consumer.qosCalled)
		assert.Equal(t, 7, consumer.qosCount)
		assert.Empty(t, publisher.consumeQueue, "the publisher channel must never consume")
	})

	t.Run("Edge: a zero prefetch leaves the broker's default", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})

		_, err := broker.Consume(context.Background())
		require.NoError(t, err)

		assert.False(t, consumer.qosCalled)
	})

	t.Run("Edge: each registration gets a unique tag", func(t *testing.T) {
		broker, _, first, conn := newConsumingBroker(t, Config{})
		second := &fakeChannel{deliveries: make(chan amqp.Delivery)}
		conn.consumers = append(conn.consumers, second)

		_, err := broker.Consume(context.Background())
		require.NoError(t, err)
		_, err = broker.Consume(context.Background())
		require.NoError(t, err)

		assert.NotEmpty(t, first.consumeTag)
		assert.NotEqual(t, first.consumeTag, second.consumeTag)
		assert.True(t, first.closed, "registering again closes the previous consumer channel")
	})
}

// TestBroker_ConsumeFailures proves every registration failure is the typed
// broker error and leaves no channel open (WKR-17).
func TestBroker_ConsumeFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(consumer *fakeChannel, conn *fakeConnection)
		closed bool
	}{
		{name: "Error: the consumer channel cannot be opened", mutate: func(_ *fakeChannel, conn *fakeConnection) { conn.channelErr = errors.New("connection closed") }},
		{name: "Error: the prefetch is refused", mutate: func(c *fakeChannel, _ *fakeConnection) { c.qosErr = errors.New("channel closed") }, closed: true},
		{name: "Error: the consumer is refused", mutate: func(c *fakeChannel, _ *fakeConnection) { c.consumeErr = errors.New("queue missing") }, closed: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			broker, _, consumer, conn := newConsumingBroker(t, Config{Prefetch: 3})
			tc.mutate(consumer, conn)

			stream, err := broker.Consume(context.Background())

			assert.ErrorIs(t, err, ErrBrokerUnavailable)
			assert.Nil(t, stream)
			assert.Equal(t, tc.closed, consumer.closed)
		})
	}
}

// TestBroker_ConsumeStream proves deliveries reach the worker with their body,
// that the handle acknowledges and never rejects, and that the stream ends when
// the broker ends it (WKR-02, WKR-17).
func TestBroker_ConsumeStream(t *testing.T) {
	t.Run("Happy path: a delivery is handed over and acknowledged once", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		acker := &recordingAcknowledger{}
		stream, err := broker.Consume(context.Background())
		require.NoError(t, err)

		consumer.deliveries <- amqp.Delivery{Acknowledger: acker, DeliveryTag: 42, Body: []byte(`{"id":"x"}`)}
		delivery, ok := receive(t, stream)
		require.True(t, ok)

		assert.Equal(t, []byte(`{"id":"x"}`), delivery.Body)
		require.NoError(t, delivery.Handle.Ack())
		assert.Equal(t, []uint64{42}, acker.acked)
		assert.Empty(t, acker.nacked)
		assert.Empty(t, acker.rejected)
	})

	t.Run("Error: a failed acknowledgement is reported", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		acker := &recordingAcknowledger{ackErr: errors.New("channel closed")}
		stream, err := broker.Consume(context.Background())
		require.NoError(t, err)

		consumer.deliveries <- amqp.Delivery{Acknowledger: acker, DeliveryTag: 1}
		delivery, _ := receive(t, stream)

		assert.Error(t, delivery.Handle.Ack())
	})

	t.Run("Edge: the stream ends when the broker cancels the consumer", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		stream, err := broker.Consume(context.Background())
		require.NoError(t, err)

		close(consumer.deliveries)
		_, ok := receive(t, stream)

		assert.False(t, ok, "the stream must end so the worker registers again")
	})

	t.Run("Edge: cancelling the context cancels the consumer and ends the stream", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := broker.Consume(ctx)
		require.NoError(t, err)

		cancel()
		_, ok := receive(t, stream)

		assert.False(t, ok)
		consumer.mu.Lock()
		defer consumer.mu.Unlock()
		assert.Equal(t, []string{consumer.consumeTag}, consumer.cancelled)
	})

	t.Run("Edge: a failed cancel does not stop the stream from ending", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		consumer.cancelErr = errors.New("channel closed")
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := broker.Consume(ctx)
		require.NoError(t, err)

		cancel()
		_, ok := receive(t, stream)

		assert.False(t, ok)
	})

	t.Run("Edge: closing the broker ends the stream and closes the consumer channel", func(t *testing.T) {
		broker, _, consumer, _ := newConsumingBroker(t, Config{})
		stream, err := broker.Consume(context.Background())
		require.NoError(t, err)

		require.NoError(t, broker.Close())
		_, ok := receive(t, stream)

		assert.False(t, ok)
		consumer.mu.Lock()
		defer consumer.mu.Unlock()
		assert.True(t, consumer.closed)
	})
}

// TestBroker_PublishTo pins the destination mapping and the publish guarantees:
// each destination reaches its exchange and routing key, the publish is
// mandatory, persistent and confirmed (WKR-09, WKR-11, WKR-13).
func TestBroker_PublishTo(t *testing.T) {
	tests := []struct {
		name         string
		destination  email.Destination
		wantExchange string
		wantKey      string
	}{
		{name: "Happy path: the first retry tier", destination: email.DestinationRetry30s, wantExchange: exchangeRetry, wantKey: "30s"},
		{name: "Happy path: the second retry tier", destination: email.DestinationRetry2m, wantExchange: exchangeRetry, wantKey: "2m"},
		{name: "Happy path: the last retry tier", destination: email.DestinationRetry5m, wantExchange: exchangeRetry, wantKey: "5m"},
		{name: "Happy path: the dead-letter route", destination: email.DestinationDeadLetter, wantExchange: exchangeMain, wantKey: "email.dead"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeChannel{}
			broker, _ := newFakeBroker(t, fake, Config{})
			message := testMessage()
			message.Attempt = 2

			require.NoError(t, broker.PublishTo(context.Background(), tc.destination, message))

			assert.Equal(t, tc.wantExchange, fake.lastExchange)
			assert.Equal(t, tc.wantKey, fake.lastKey)
			assert.True(t, fake.lastMandatory)
			require.Len(t, fake.published, 1)
			assert.Equal(t, amqp.Persistent, fake.published[0].DeliveryMode)
			assert.Equal(t, message.ID, fake.published[0].MessageId)
		})
	}
}

// TestBroker_PublishToFailures proves every unconfirmed outcome is a failure,
// and an unknown destination publishes nothing (WKR-13).
func TestBroker_PublishToFailures(t *testing.T) {
	tests := []struct {
		name  string
		fake  *fakeChannel
		block bool
		cfg   Config
	}{
		{name: "Error: a nack", fake: &fakeChannel{confirm: &fakeConfirmation{acked: false}}},
		{name: "Error: a confirm timeout", fake: &fakeChannel{confirm: &fakeConfirmation{wait: func(ctx context.Context) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		}}}, cfg: Config{ConfirmTimeout: 10 * time.Millisecond}},
		{name: "Error: a transport error", fake: &fakeChannel{publishErr: errors.New("connection reset")}},
		{name: "Error: an unroutable message", fake: &fakeChannel{returnBeforeConfirm: true}},
		{name: "Error: a blocked connection", fake: &fakeChannel{}, block: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			broker, _ := newFakeBroker(t, tc.fake, tc.cfg)
			if tc.block {
				broker.setBlocked(true)
			}

			err := broker.PublishTo(context.Background(), email.DestinationRetry30s, testMessage())

			assert.ErrorIs(t, err, email.ErrTransient)
		})
	}

	unknown := []email.Destination{"", "email.otp", "retry_1h", "DEAD_LETTER"}
	for _, destination := range unknown {
		t.Run("Edge: unknown destination "+string(destination), func(t *testing.T) {
			fake := &fakeChannel{}
			broker, _ := newFakeBroker(t, fake, Config{})

			err := broker.PublishTo(context.Background(), destination, testMessage())

			assert.ErrorIs(t, err, ErrUnknownDestination)
			assert.Empty(t, fake.published, "nothing is published to an unknown destination")
		})
	}
}

// TestBroker_PublishBodyTo proves an undecodable body is parked byte for byte,
// under a fresh canonical identity, with the same guarantees (WKR-04, WKR-13).
func TestBroker_PublishBodyTo(t *testing.T) {
	t.Run("Happy path: the body is published unchanged to the dead-letter route", func(t *testing.T) {
		fake := &fakeChannel{}
		broker, _ := newFakeBroker(t, fake, Config{})
		raw := []byte("not json \x00\xff")

		require.NoError(t, broker.PublishBodyTo(context.Background(), email.DestinationDeadLetter, raw))

		require.Len(t, fake.published, 1)
		assert.Equal(t, raw, fake.published[0].Body)
		assert.Equal(t, exchangeMain, fake.lastExchange)
		assert.Equal(t, routingKeyDead, fake.lastKey)
		assert.True(t, fake.lastMandatory)
		assert.NoError(t, idgen.Validate(fake.published[0].MessageId))
	})

	t.Run("Error: an unroutable body is a failure", func(t *testing.T) {
		broker, _ := newFakeBroker(t, &fakeChannel{returnBeforeConfirm: true}, Config{})

		err := broker.PublishBodyTo(context.Background(), email.DestinationDeadLetter, []byte("x"))

		assert.ErrorIs(t, err, email.ErrTransient)
	})

	t.Run("Edge: an unknown destination publishes nothing", func(t *testing.T) {
		fake := &fakeChannel{}
		broker, _ := newFakeBroker(t, fake, Config{})

		err := broker.PublishBodyTo(context.Background(), "nowhere", []byte("x"))

		assert.ErrorIs(t, err, ErrUnknownDestination)
		assert.Empty(t, fake.published)
	})

	t.Run("Edge: an empty body is still published", func(t *testing.T) {
		fake := &fakeChannel{}
		broker, _ := newFakeBroker(t, fake, Config{})

		require.NoError(t, broker.PublishBodyTo(context.Background(), email.DestinationDeadLetter, nil))
		assert.Len(t, fake.published, 1)
	})
}
