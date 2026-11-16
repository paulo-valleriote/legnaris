package amqp

import (
	"context"
	"fmt"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

const defaultDialTimeout = 2 * time.Second

type channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error
	Confirm(noWait bool) error
	NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
	Close() error
}

type connection interface {
	Channel() (channel, error)
	Close() error
}

type dialer func(url string) (connection, error)

type liveConnection struct {
	conn *amqp091.Connection
}

func (c liveConnection) Channel() (channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (c liveConnection) Close() error {
	return c.conn.Close()
}

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

func dialAMQP(url string) (connection, error) {
	conn, err := amqp091.DialConfig(url, amqp091.Config{Dial: amqp091.DefaultDial(defaultDialTimeout)})
	if err != nil {
		return nil, err
	}
	return liveConnection{conn: conn}, nil
}
