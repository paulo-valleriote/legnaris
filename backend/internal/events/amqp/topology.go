package amqp

import (
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeEvents   = "legnaris.events"
	ExchangeUnrouted = "legnaris.events.unrouted"
	QueueUnrouted    = "events.unrouted"

	argAlternateExchange = "alternate-exchange"
	argQueueType         = "x-queue-type"
	queueTypeQuorum      = "quorum"
)

func declareTopology(ch channel) error {
	if err := ch.ExchangeDeclare(ExchangeUnrouted, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeUnrouted, err)
	}
	if _, err := ch.QueueDeclare(QueueUnrouted, true, false, false, false, amqp091.Table{argQueueType: queueTypeQuorum}); err != nil {
		return fmt.Errorf("declare queue %s: %w", QueueUnrouted, err)
	}
	if err := ch.QueueBind(QueueUnrouted, "", ExchangeUnrouted, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", QueueUnrouted, err)
	}
	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, amqp091.Table{argAlternateExchange: ExchangeUnrouted}); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeEvents, err)
	}
	return nil
}
