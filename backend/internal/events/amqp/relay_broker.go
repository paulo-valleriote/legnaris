package amqp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/events/outbox"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// ErrNotConfirmed reports a publish the broker rejected with a nack. The relay
// reschedules the entry.
var ErrNotConfirmed = errors.New("broker did not confirm the event")

// RelayBrokerConfig configures a RelayBroker.
type RelayBrokerConfig struct {
	// URL is the AMQP URI of the broker.
	URL string
	// ConfirmTimeout bounds each wait for a publisher confirm; zero uses DefaultConfirmTimeout.
	ConfirmTimeout time.Duration

	dial dialer
}

// RelayBroker publishes guaranteed events synchronously with publisher confirms
// and reports each outcome to the relay, which owns the retries. It connects on
// first use and reconnects after any failure, so the relay starts while the
// broker is down. A RelayBroker is not safe for concurrent use: the relay is its
// only caller.
type RelayBroker struct {
	url            string
	dial           dialer
	confirmTimeout time.Duration

	conn     connection
	ch       channel
	confirms chan amqp091.Confirmation
}

// NewRelayBroker returns a RelayBroker. It does not dial.
func NewRelayBroker(cfg RelayBrokerConfig) *RelayBroker {
	broker := &RelayBroker{
		url:            cfg.URL,
		dial:           cfg.dial,
		confirmTimeout: cfg.ConfirmTimeout,
	}
	if broker.dial == nil {
		broker.dial = dialAMQP
	}
	if broker.confirmTimeout <= 0 {
		broker.confirmTimeout = DefaultConfirmTimeout
	}
	return broker
}

// Publish sends one entry to ExchangeEvents, keyed by its event type, persistent
// and with the envelope id as MessageId. It returns nil only after the broker
// confirms. Any failure resets the connection so the next call reconnects.
func (b *RelayBroker) Publish(ctx context.Context, entry outbox.Entry) error {
	if b.ch == nil {
		if err := b.open(); err != nil {
			return err
		}
	}

	props := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    entry.ID,
		Type:         entry.EventType,
		Body:         entry.Body,
	}
	if err := b.ch.PublishWithContext(ctx, ExchangeEvents, entry.EventType, false, false, props); err != nil {
		b.reset()
		return fmt.Errorf("publish event: %w", err)
	}

	return b.awaitConfirm(ctx)
}

// awaitConfirm waits for the confirmation of the message just published.
func (b *RelayBroker) awaitConfirm(ctx context.Context) error {
	waitCtx, cancel := context.WithTimeout(ctx, b.confirmTimeout)
	defer cancel()

	select {
	case confirmation, ok := <-b.confirms:
		if !ok {
			b.reset()
			return fmt.Errorf("publish event: confirm channel closed")
		}
		if !confirmation.Ack {
			return ErrNotConfirmed
		}
		return nil
	case <-waitCtx.Done():
		b.reset()
		return fmt.Errorf("publish event: confirm wait: %w", waitCtx.Err())
	}
}

// Close releases the connection. It is safe to call when nothing is open.
func (b *RelayBroker) Close() error {
	b.reset()
	return nil
}

// open dials, enables confirms and declares the event topology. A failure closes
// what it opened, so no half-open connection is kept.
func (b *RelayBroker) open() error {
	if b.url == "" {
		return errURLNotSet
	}

	conn, ch, err := openConfirmedChannel(b.dial, b.url)
	if err != nil {
		return err
	}

	b.conn = conn
	b.ch = ch
	b.confirms = ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	return nil
}

// reset closes the channel and connection and forgets them.
func (b *RelayBroker) reset() {
	if b.ch != nil {
		_ = b.ch.Close()
	}
	if b.conn != nil {
		_ = b.conn.Close()
	}
	b.ch = nil
	b.conn = nil
	b.confirms = nil
}
