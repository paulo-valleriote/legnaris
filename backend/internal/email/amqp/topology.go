package amqp

import (
	"errors"
	"fmt"

	"legnaris/backend/internal/email"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Broker entity names, routing keys and queue arguments. They are code
// constants so the declared topology is one source of truth and the deployment
// needs no broker plugin or version-specific extension (EML-06).
const (
	exchangeMain  = "legnaris.email"
	exchangeRetry = "legnaris.email.retry"

	queueWork     = "email.otp"
	queueRetry30s = "email.retry.30s"
	queueRetry2m  = "email.retry.2m"
	queueRetry5m  = "email.retry.5m"
	queueDead     = "email.dead"

	routingKeyOTP  = "email.otp"
	routingKey30s  = "30s"
	routingKey2m   = "2m"
	routingKey5m   = "5m"
	routingKeyDead = "email.dead"

	// workQueueMaxLength caps the main queue; the queue-depth alert fires at
	// 80% of it (8000 ready messages).
	workQueueMaxLength = 10000
)

// AMQP argument and property keys shared by the topology specification.
const (
	argQueueType          = "x-queue-type"
	argMaxLength          = "x-max-length"
	argOverflow           = "x-overflow"
	argMessageTTL         = "x-message-ttl"
	argDeadLetterExchange = "x-dead-letter-exchange"
	argDeadLetterKey      = "x-dead-letter-routing-key"

	queueTypeQuorum = "quorum"
	overflowReject  = "reject-publish"
)

// exchangeSpec is one declared exchange.
type exchangeSpec struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	Args       amqp.Table
}

// bindingSpec is one queue binding.
type bindingSpec struct {
	Exchange   string
	RoutingKey string
}

// queueSpec is one declared queue and its bindings.
type queueSpec struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	Args       amqp.Table
	Bindings   []bindingSpec
}

// topology is the declarative description of the email broker topology.
type topology struct {
	Exchanges []exchangeSpec
	Queues    []queueSpec
}

// emailTopology returns the entities data-model.md §4 fixes: a direct main
// exchange, a quorum work queue that dead-letters to the retry exchange, three
// delay-tier retry queues that dead-letter back, and a dead-letter queue. All
// are durable and declared idempotently before the first publish (EML-06).
func emailTopology() topology {
	return topology{
		Exchanges: []exchangeSpec{
			{Name: exchangeMain, Kind: amqp.ExchangeDirect, Durable: true},
			{Name: exchangeRetry, Kind: amqp.ExchangeDirect, Durable: true},
		},
		Queues: []queueSpec{
			{
				Name:    queueWork,
				Durable: true,
				Args:    quorumArgs(workQueueArgs()),
				Bindings: []bindingSpec{
					{Exchange: exchangeMain, RoutingKey: routingKeyOTP},
				},
			},
			{
				Name:    queueRetry30s,
				Durable: true,
				Args:    quorumArgs(retryTierArgs(30000)),
				Bindings: []bindingSpec{
					{Exchange: exchangeRetry, RoutingKey: routingKey30s},
				},
			},
			{
				Name:    queueRetry2m,
				Durable: true,
				Args:    quorumArgs(retryTierArgs(120000)),
				Bindings: []bindingSpec{
					{Exchange: exchangeRetry, RoutingKey: routingKey2m},
				},
			},
			{
				Name:    queueRetry5m,
				Durable: true,
				Args:    quorumArgs(retryTierArgs(300000)),
				Bindings: []bindingSpec{
					{Exchange: exchangeRetry, RoutingKey: routingKey5m},
				},
			},
			{
				Name:    queueDead,
				Durable: true,
				Args:    quorumArgs(nil),
				Bindings: []bindingSpec{
					{Exchange: exchangeMain, RoutingKey: routingKeyDead},
				},
			},
		},
	}
}

// quorumArgs returns a copy of args with the quorum queue type set.
func quorumArgs(args amqp.Table) amqp.Table {
	if args == nil {
		args = amqp.Table{}
	}
	args[argQueueType] = queueTypeQuorum
	return args
}

// workQueueArgs returns the main work queue's arguments: a bounded length that
// rejects rather than drops on overflow, and a dead-letter route to the first
// retry tier (EML-06).
func workQueueArgs() amqp.Table {
	return amqp.Table{
		argMaxLength:          int32(workQueueMaxLength),
		argOverflow:           overflowReject,
		argDeadLetterExchange: exchangeRetry,
		argDeadLetterKey:      routingKey30s,
	}
}

// retryTierArgs returns one delay tier's arguments: a message TTL that routes
// back to the main exchange when it expires (EML-06).
func retryTierArgs(ttlMilliseconds int32) amqp.Table {
	return amqp.Table{
		argMessageTTL:         ttlMilliseconds,
		argDeadLetterExchange: exchangeMain,
		argDeadLetterKey:      routingKeyOTP,
	}
}

// declare declares every exchange, queue and binding idempotently. A repeated
// identical declaration is a no-op, so the topology is safe to declare on every
// startup (EML-06).
func (t topology) declare(ch channel) error {
	for _, exchange := range t.Exchanges {
		if err := ch.ExchangeDeclare(exchange.Name, exchange.Kind, exchange.Durable, exchange.AutoDelete, exchange.Internal, false, exchange.Args); err != nil {
			return fmt.Errorf("declare exchange %s: %w", exchange.Name, err)
		}
	}

	for _, queue := range t.Queues {
		if _, err := ch.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, false, queue.Args); err != nil {
			return fmt.Errorf("declare queue %s: %w", queue.Name, err)
		}
		for _, binding := range queue.Bindings {
			if err := ch.QueueBind(queue.Name, binding.RoutingKey, binding.Exchange, false, nil); err != nil {
				return fmt.Errorf("bind queue %s to %s: %w", queue.Name, binding.Exchange, err)
			}
		}
	}
	return nil
}

// ErrUnknownDestination reports a destination outside the domain's closed
// vocabulary; nothing is published (WKR-09).
var ErrUnknownDestination = errors.New("unknown email destination")

// route maps a destination to the exchange and routing key that reach its
// queue. It is the only place a routing key is chosen for the worker (WKR-09,
// WKR-11).
func route(destination email.Destination) (exchange, key string, err error) {
	switch destination {
	case email.DestinationRetry30s:
		return exchangeRetry, routingKey30s, nil
	case email.DestinationRetry2m:
		return exchangeRetry, routingKey2m, nil
	case email.DestinationRetry5m:
		return exchangeRetry, routingKey5m, nil
	case email.DestinationDeadLetter:
		return exchangeMain, routingKeyDead, nil
	default:
		return "", "", fmt.Errorf("%w: %q", ErrUnknownDestination, string(destination))
	}
}
