package amqp

import (
	"errors"
	"fmt"
	"math"
	"time"

	"legnaris/backend/internal/email"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

	workQueueMaxLength = 10000
)

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

type exchangeSpec struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	Args       amqp.Table
}

type bindingSpec struct {
	Exchange   string
	RoutingKey string
}

type queueSpec struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	Args       amqp.Table
	Bindings   []bindingSpec
}

type topology struct {
	Exchanges []exchangeSpec
	Queues    []queueSpec
}

func emailTopology(delays RetryDelays) topology {
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
				Args:    quorumArgs(retryTierArgs(delays.First)),
				Bindings: []bindingSpec{
					{Exchange: exchangeRetry, RoutingKey: routingKey30s},
				},
			},
			{
				Name:    queueRetry2m,
				Durable: true,
				Args:    quorumArgs(retryTierArgs(delays.Second)),
				Bindings: []bindingSpec{
					{Exchange: exchangeRetry, RoutingKey: routingKey2m},
				},
			},
			{
				Name:    queueRetry5m,
				Durable: true,
				Args:    quorumArgs(retryTierArgs(delays.Later)),
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

func quorumArgs(args amqp.Table) amqp.Table {
	if args == nil {
		args = amqp.Table{}
	}
	args[argQueueType] = queueTypeQuorum
	return args
}

func workQueueArgs() amqp.Table {
	return amqp.Table{
		argMaxLength:          int32(workQueueMaxLength),
		argOverflow:           overflowReject,
		argDeadLetterExchange: exchangeRetry,
		argDeadLetterKey:      routingKey30s,
	}
}

func retryTierArgs(delay time.Duration) amqp.Table {
	return amqp.Table{
		argMessageTTL:         int32(delay.Milliseconds()),
		argDeadLetterExchange: exchangeMain,
		argDeadLetterKey:      routingKeyOTP,
	}
}

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

var ErrUnknownDestination = errors.New("unknown email destination")

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

const (
	DefaultRetryDelayFirst  = 30 * time.Second
	DefaultRetryDelaySecond = 2 * time.Minute
	DefaultRetryDelayLater  = 5 * time.Minute
)

var ErrInvalidRetryDelay = errors.New("email: invalid retry delay")

type RetryDelays struct {
	First  time.Duration
	Second time.Duration
	Later  time.Duration
}

func (d RetryDelays) resolve() (RetryDelays, error) {
	resolved := RetryDelays{
		First:  orDefault(d.First, DefaultRetryDelayFirst),
		Second: orDefault(d.Second, DefaultRetryDelaySecond),
		Later:  orDefault(d.Later, DefaultRetryDelayLater),
	}
	for _, delay := range []time.Duration{resolved.First, resolved.Second, resolved.Later} {
		if delay < time.Millisecond || delay.Milliseconds() > math.MaxInt32 {
			return RetryDelays{}, fmt.Errorf("%w: %s", ErrInvalidRetryDelay, delay)
		}
	}
	return resolved, nil
}

func orDefault(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}
	return value
}
