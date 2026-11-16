package amqp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/events/outbox"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

var ErrNotConfirmed = errors.New("broker did not confirm the event")

type RelayBrokerConfig struct {
	URL            string
	ConfirmTimeout time.Duration

	dial dialer
}

type RelayBroker struct {
	url            string
	dial           dialer
	confirmTimeout time.Duration

	conn     connection
	ch       channel
	confirms chan amqp091.Confirmation
}

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

func (b *RelayBroker) Close() error {
	b.reset()
	return nil
}

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
