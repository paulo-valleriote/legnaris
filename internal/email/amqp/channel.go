// Package amqp is the RabbitMQ adapter: it declares the broker topology
// idempotently and publishes the message contract with publisher confirms and
// mandatory routing. The broker specifics sit behind a small channel seam (an
// interface plus an injectable dial function) so the publish, nack, timeout,
// unroutable and blocked paths are unit-tested without a broker (EML-06,
// EML-07, EML-19).
package amqp

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

// confirmation is the subset of the deferred publisher confirm the adapter
// waits on, so a fake can drive ack/nack/timeout without a broker.
type confirmation interface {
	WaitContext(ctx context.Context) (bool, error)
}

// channel is the subset of the AMQP channel operations the adapter uses.
type channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Confirm(noWait bool) error
	NotifyReturn(c chan amqp.Return) chan amqp.Return
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (confirmation, error)
	Close() error
}

// connection is the subset of the AMQP connection operations the adapter uses.
type connection interface {
	Channel() (channel, error)
	NotifyStateChange(recv chan *amqp.StateChanged)
	NotifyBlocked(recv chan amqp.Blocking) chan amqp.Blocking
	Close() error
}

// amqpChannel adapts a live *amqp.Channel to the seam, narrowing the deferred
// confirmation to the confirmation interface (Go has no covariant returns).
type amqpChannel struct {
	ch *amqp.Channel
}

func (c *amqpChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	return c.ch.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (c *amqpChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return c.ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (c *amqpChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	return c.ch.QueueBind(name, key, exchange, noWait, args)
}

func (c *amqpChannel) Confirm(noWait bool) error {
	return c.ch.Confirm(noWait)
}

func (c *amqpChannel) NotifyReturn(recv chan amqp.Return) chan amqp.Return {
	return c.ch.NotifyReturn(recv)
}

func (c *amqpChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (confirmation, error) {
	conf, err := c.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, immediate, msg)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (c *amqpChannel) Close() error {
	return c.ch.Close()
}

// amqpConnection adapts a live *amqp.Connection to the connection seam.
type amqpConnection struct {
	conn *amqp.Connection
}

func (c *amqpConnection) Channel() (channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return &amqpChannel{ch: ch}, nil
}

func (c *amqpConnection) NotifyStateChange(recv chan *amqp.StateChanged) {
	c.conn.NotifyStateChange(recv)
}

func (c *amqpConnection) NotifyBlocked(recv chan amqp.Blocking) chan amqp.Blocking {
	return c.conn.NotifyBlocked(recv)
}

func (c *amqpConnection) Close() error {
	return c.conn.Close()
}

// dialer opens a connection to the broker. It is a function value so a test can
// substitute a fake connection with no broker running (EML-07).
type dialer func(url string, cfg amqp.Config) (connection, error)

// defaultDialer opens a real connection with automatic reconnection and
// topology recovery enabled (EML-06).
func defaultDialer(url string, cfg amqp.Config) (connection, error) {
	conn, err := amqp.DialConfig(url, cfg)
	if err != nil {
		return nil, err
	}
	return &amqpConnection{conn: conn}, nil
}
