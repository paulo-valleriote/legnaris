package amqp

import (
	"context"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

type confirmation interface {
	WaitContext(ctx context.Context) (bool, error)
}

type channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error
	Confirm(noWait bool) error
	NotifyReturn(c chan amqp091.Return) chan amqp091.Return
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) (confirmation, error)
	Close() error
}

type connection interface {
	Channel() (channel, error)
	Close() error
}

type dialer func(url string) (connection, error)

func defaultDialer(url string) (connection, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, err
	}
	return amqpConnection{conn: conn}, nil
}

type amqpConnection struct {
	conn *amqp091.Connection
}

func (c amqpConnection) Channel() (channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return amqpChannel{ch: ch}, nil
}

func (c amqpConnection) Close() error {
	return c.conn.Close()
}

type amqpChannel struct {
	ch *amqp091.Channel
}

func (c amqpChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error {
	return c.ch.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (c amqpChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error) {
	return c.ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (c amqpChannel) QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error {
	return c.ch.QueueBind(name, key, exchange, noWait, args)
}

func (c amqpChannel) Confirm(noWait bool) error {
	return c.ch.Confirm(noWait)
}

func (c amqpChannel) NotifyReturn(recv chan amqp091.Return) chan amqp091.Return {
	return c.ch.NotifyReturn(recv)
}

func (c amqpChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) (confirmation, error) {
	conf, err := c.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, immediate, msg)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (c amqpChannel) Close() error {
	return c.ch.Close()
}
