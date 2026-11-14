package amqp

import (
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Broker entity names of the domain event topology (ADR 0010, Topology; ADR
// 0013, Amendments). The API and the relay only publish to ExchangeEvents and
// know no queues: consumers declare and own their queues.
const (
	// ExchangeEvents is the durable topic exchange; the routing key is the event type.
	ExchangeEvents = "legnaris.events"
	// ExchangeUnrouted is the alternate exchange that keeps events with no bound queue.
	ExchangeUnrouted = "legnaris.events.unrouted"
	// QueueUnrouted holds events that reached no consumer; alert when non-empty.
	QueueUnrouted = "events.unrouted"

	argAlternateExchange = "alternate-exchange"
	argQueueType         = "x-queue-type"
	queueTypeQuorum      = "quorum"
)

// declareTopology declares the event topology. Every declaration is idempotent
// on the broker, so running it on each (re)connect is safe. The unrouted
// exchange is declared first because the main exchange names it as alternate.
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
