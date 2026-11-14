package amqp

import (
	"context"
	"fmt"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// defaultDialTimeout bounds one dial attempt so a black-holed broker cannot
// stall the publisher worker for longer than this.
const defaultDialTimeout = 2 * time.Second

// channel is the subset of the AMQP channel the publisher uses. It exists so
// the connect, confirm, nack and timeout paths are tested without a broker.
type channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error
	Confirm(noWait bool) error
	NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
	Close() error
}

// connection is the subset of the AMQP connection the publisher uses.
type connection interface {
	Channel() (channel, error)
	Close() error
}

// dialer opens a connection to the broker at url.
type dialer func(url string) (connection, error)

// liveConnection adapts a *amqp091.Connection to the connection seam.
type liveConnection struct {
	conn *amqp091.Connection
}

// Channel opens a channel on the live connection. A failed open returns a nil
// channel so the caller never holds a typed-nil interface.
func (c liveConnection) Channel() (channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// Close closes the live connection.
func (c liveConnection) Close() error {
	return c.conn.Close()
}

// openConfirmedChannel dials url, opens a channel with publisher confirms enabled
// and declares the event topology. Every failure closes what it opened, so a
// half-open connection is never returned.
func openConfirmedChannel(dial dialer, url string) (connection, channel, error) {
	conn, err := dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("dial broker: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, ch, nil
}

// dialAMQP is the production dialer. It enables no automatic recovery: the
// publisher reconnects on its own schedule so a down broker never blocks it.
func dialAMQP(url string) (connection, error) {
	conn, err := amqp091.DialConfig(url, amqp091.Config{Dial: amqp091.DefaultDial(defaultDialTimeout)})
	if err != nil {
		return nil, err
	}
	return liveConnection{conn: conn}, nil
}
