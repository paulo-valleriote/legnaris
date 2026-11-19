package amqp

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/events/outbox"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRelayEntry(t *testing.T, eventType string) outbox.Entry {
	t.Helper()
	ev := sampleEvent(eventType)
	entry, err := outbox.NewEntry(ev, time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return entry
}

func TestRelayBrokerPublish(t *testing.T) {
	t.Run("Happy path: an acked entry is published to the events exchange with its identity", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: healthyDialer(ch).dial})
		entry := newRelayEntry(t, "user.deleted")

		require.NoError(t, broker.Publish(context.Background(), entry))

		msgs := ch.publishedMessages()
		require.Len(t, msgs, 1)
		assert.Equal(t, ExchangeEvents, msgs[0].exchange)
		assert.Equal(t, "user.deleted", msgs[0].key, "the routing key is the event type")
		assert.Equal(t, entry.ID, msgs[0].msg.MessageId, "the envelope id is the AMQP MessageId")
		assert.Equal(t, "user.deleted", msgs[0].msg.Type)
		assert.Equal(t, uint8(amqp091.Persistent), msgs[0].msg.DeliveryMode)
		assert.Equal(t, "application/json", msgs[0].msg.ContentType)
		assert.Equal(t, entry.Body, msgs[0].msg.Body, "the body is published unchanged")
	})

	t.Run("Happy path: the topology is declared before the first publish", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: healthyDialer(ch).dial})

		require.NoError(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))

		assert.Contains(t, ch.declared, "exchange:"+ExchangeEvents+":topic")
	})

	t.Run("Happy path: a second publish reuses the open channel and dials once", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		d := healthyDialer(ch)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: d.dial})

		require.NoError(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))
		require.NoError(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))

		assert.Equal(t, 1, d.dialCount())
		assert.Len(t, ch.publishedMessages(), 2)
	})

	t.Run("Error: a nacked entry is not reported as published", func(t *testing.T) {
		ch := newFakeChannel(confirmNack)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: healthyDialer(ch).dial})

		err := broker.Publish(context.Background(), newRelayEntry(t, "user.deleted"))
		assert.ErrorIs(t, err, ErrNotConfirmed, "want ErrNotConfirmed, got %v", err)
	})

	t.Run("Error: a missing confirm times out and the channel is reset", func(t *testing.T) {
		ch := newFakeChannel(confirmSilent)
		d := healthyDialer(ch)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", ConfirmTimeout: 10 * time.Millisecond, dial: d.dial})

		err := broker.Publish(context.Background(), newRelayEntry(t, "user.deleted"))
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.True(t, ch.closed, "a timed-out channel must be closed so the next publish reconnects")
	})

	t.Run("Error: a cancelled context fails the publish without a confirm", func(t *testing.T) {
		ch := newFakeChannel(confirmSilent)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", ConfirmTimeout: time.Minute, dial: healthyDialer(ch).dial})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := broker.Publish(ctx, newRelayEntry(t, "user.deleted"))
		assert.ErrorIs(t, err, context.Canceled, "want context.Canceled, got %v", err)
	})

	t.Run("Error: a publish failure resets the channel and the next call reconnects", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		ch.publishErr = errFake
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: healthyDialer(ch).dial})

		require.Error(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))
		assert.True(t, ch.closed)
	})

	t.Run("Edge: a broker that is down fails with a dial error and no channel", func(t *testing.T) {
		d := &fakeDialer{}
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: d.dial})

		err := broker.Publish(context.Background(), newRelayEntry(t, "user.deleted"))
		require.Error(t, err)
		assert.Equal(t, 1, d.dialCount())
	})

	t.Run("Edge: a down broker is retried on the next publish, once the broker returns", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		d := &fakeDialer{results: []dialResult{{err: errFake}, {conn: &fakeConn{ch: ch}}}}
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: d.dial})

		require.Error(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))
		require.NoError(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))
		assert.Equal(t, 2, d.dialCount())
	})

	t.Run("Edge: a channel open failure is reported and no channel is kept", func(t *testing.T) {
		d := &fakeDialer{results: []dialResult{{conn: &fakeConn{chErr: errFake}}}}
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: d.dial})

		require.Error(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))
		assert.Nil(t, broker.ch)
	})

	t.Run("Edge: an empty URL fails without dialing", func(t *testing.T) {
		d := &fakeDialer{}
		broker := NewRelayBroker(RelayBrokerConfig{URL: "", dial: d.dial})

		require.ErrorIs(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")), errURLNotSet)
		assert.Equal(t, 0, d.dialCount())
	})
}

func TestRelayBrokerClose(t *testing.T) {
	t.Run("Edge: closing a broker that never connected is safe", func(t *testing.T) {
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x"})
		assert.NoError(t, broker.Close())
		assert.NoError(t, broker.Close())
	})

	t.Run("Happy path: closing an open broker closes its channel", func(t *testing.T) {
		ch := newFakeChannel(confirmAck)
		broker := NewRelayBroker(RelayBrokerConfig{URL: "amqp://x", dial: healthyDialer(ch).dial})
		require.NoError(t, broker.Publish(context.Background(), newRelayEntry(t, "user.deleted")))

		require.NoError(t, broker.Close())
		assert.True(t, ch.closed)
	})
}
