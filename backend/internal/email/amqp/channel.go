package amqp

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type confirmation interface {
	WaitContext(ctx context.Context) (bool, error)
}

type channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Confirm(noWait bool) error
	NotifyReturn(c chan amqp.Return) chan amqp.Return
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (confirmation, error)
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Cancel(consumer string, noWait bool) error
	Close() error
}

type connection interface {
	Channel() (channel, error)
	NotifyStateChange(recv chan *amqp.StateChanged)
	NotifyBlocked(recv chan amqp.Blocking) chan amqp.Blocking
	Close() error
}

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

func (c *amqpChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	return c.ch.Qos(prefetchCount, prefetchSize, global)
}

func (c *amqpChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	return c.ch.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
}

func (c *amqpChannel) Cancel(consumer string, noWait bool) error {
	return c.ch.Cancel(consumer, noWait)
}

func (c *amqpChannel) Close() error {
	return c.ch.Close()
}

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

type dialer func(url string, cfg amqp.Config) (connection, error)

func defaultDialer(url string, cfg amqp.Config) (connection, error) {
	conn, err := amqp.DialConfig(url, cfg)
	if err != nil {
		return nil, err
	}
	return &amqpConnection{conn: conn}, nil
}
