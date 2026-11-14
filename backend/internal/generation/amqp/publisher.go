// Package amqp is the RabbitMQ adapter of the generation queue. It declares the
// generation topology idempotently and publishes each job with publisher confirms
// and mandatory routing, so a nil return means the broker routed the job. The
// connection is opened on first use and dropped on any failure, so an API that
// starts without a broker keeps answering and the next publish redials (ADR 0007).
package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"legnaris/backend/internal/generation"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Broker entity names and routing of the generation topology (ADR 0007).
const (
	// ExchangeGeneration is the topic exchange jobs are published to.
	ExchangeGeneration = "generation"
	// QueueRequests is the quorum queue the worker consumes. Its rejected jobs
	// dead-letter to ExchangeDead.
	QueueRequests = "generation.requests"
	// ExchangeDead is the dead-letter exchange of QueueRequests.
	ExchangeDead = "generation.dead"
	// QueueDead holds the jobs dead-lettered for operator attention.
	QueueDead = "generation.dead"
	// KeyDead is the routing key that binds QueueDead to ExchangeDead.
	KeyDead = "generation.dead"
	// bindingPattern matches every kind's requested routing key.
	bindingPattern = "generation.*.requested"

	argQueueType      = "x-queue-type"
	argDeadLetterExch = "x-dead-letter-exchange"
	argDeadLetterKey  = "x-dead-letter-routing-key"
	queueTypeQuorum   = "quorum"
)

// DefaultConfirmTimeout bounds the wait for a publisher confirm when the config
// leaves it zero, so a stalled broker never holds a request open.
const DefaultConfirmTimeout = 5 * time.Second

// ErrPublishRefused reports a job the broker did not accept: a nack, an unroutable
// message or a confirm that did not arrive in time.
var ErrPublishRefused = errors.New("broker refused the generation job")

// RoutingKey returns the routing key of a job kind.
func RoutingKey(kind string) string {
	return "generation." + kind + ".requested"
}

// Config configures the publisher.
type Config struct {
	// URL is the AMQP URI of the broker.
	URL string
	// ConfirmTimeout bounds each publish's confirmation wait. Zero uses
	// DefaultConfirmTimeout.
	ConfirmTimeout time.Duration
}

// Publisher publishes generation jobs and implements generation.Publisher.
type Publisher struct {
	url            string
	confirmTimeout time.Duration
	dial           dialer

	mu      sync.Mutex
	conn    connection
	ch      channel
	returns chan amqp091.Return
}

// New returns a publisher that connects on first use.
func New(cfg Config) *Publisher {
	timeout := cfg.ConfirmTimeout
	if timeout <= 0 {
		timeout = DefaultConfirmTimeout
	}
	return &Publisher{url: cfg.URL, confirmTimeout: timeout, dial: defaultDialer}
}

// Publish encodes the message, publishes it persistently and mandatorily, and waits
// for the broker's confirm. Only identifiers are encoded: the concept never leaves
// the database through the broker.
func (p *Publisher) Publish(ctx context.Context, msg generation.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode generation job: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ch, err := p.channelLocked()
	if err != nil {
		return err
	}
	pending, err := ch.PublishWithDeferredConfirmWithContext(ctx, ExchangeGeneration, RoutingKey(msg.Kind), true, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    msg.ID,
		Body:         body,
	})
	if err != nil {
		p.resetLocked()
		return fmt.Errorf("publish generation job: %w", err)
	}
	return p.awaitConfirmLocked(ctx, pending, msg.ID)
}

// awaitConfirmLocked waits for the confirm of one publish and checks for a return
// that names the same job.
func (p *Publisher) awaitConfirmLocked(ctx context.Context, pending confirmation, messageID string) error {
	confirmCtx, cancel := context.WithTimeout(ctx, p.confirmTimeout)
	defer cancel()

	acked, err := pending.WaitContext(confirmCtx)
	if err != nil {
		p.resetLocked()
		return fmt.Errorf("%w: confirm wait: %v", ErrPublishRefused, err)
	}
	if !acked {
		return fmt.Errorf("%w: nacked", ErrPublishRefused)
	}
	if p.returnedLocked(messageID) {
		return fmt.Errorf("%w: unroutable", ErrPublishRefused)
	}
	return nil
}

// returnedLocked reports whether the broker returned the message named by id. It
// drains the returns already delivered, without blocking.
func (p *Publisher) returnedLocked(messageID string) bool {
	returned := false
	for {
		select {
		case r := <-p.returns:
			if r.MessageId == messageID {
				returned = true
			}
		default:
			return returned
		}
	}
}

// channelLocked returns the open channel, dialling and declaring the topology when
// there is none.
func (p *Publisher) channelLocked() (channel, error) {
	if p.ch != nil {
		return p.ch, nil
	}
	conn, err := p.dial(p.url)
	if err != nil {
		return nil, fmt.Errorf("dial broker: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}
	p.conn = conn
	p.ch = ch
	p.returns = ch.NotifyReturn(make(chan amqp091.Return, 1))
	return ch, nil
}

// resetLocked drops the connection after a failure, so the next publish redials.
func (p *Publisher) resetLocked() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.ch = nil
	p.conn = nil
	p.returns = nil
}

// DeclareTopology opens the broker connection and declares the topology, so a
// process can fail fast when the broker is unusable.
func (p *Publisher) DeclareTopology() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.channelLocked(); err != nil {
		return err
	}
	return nil
}

// Close releases the connection. A publisher that never connected closes cleanly.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return nil
	}
	err := p.conn.Close()
	p.resetLocked()
	return err
}

// declareTopology declares the exchanges, queues and bindings. Every declaration is
// idempotent on the broker, so each connection may run it.
func declareTopology(ch channel) error {
	if err := ch.ExchangeDeclare(ExchangeGeneration, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeGeneration, err)
	}
	if err := ch.ExchangeDeclare(ExchangeDead, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeDead, err)
	}
	if _, err := ch.QueueDeclare(QueueDead, true, false, false, false, amqp091.Table{argQueueType: queueTypeQuorum}); err != nil {
		return fmt.Errorf("declare queue %s: %w", QueueDead, err)
	}
	if err := ch.QueueBind(QueueDead, KeyDead, ExchangeDead, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", QueueDead, err)
	}
	requests := amqp091.Table{
		argQueueType:      queueTypeQuorum,
		argDeadLetterExch: ExchangeDead,
		argDeadLetterKey:  KeyDead,
	}
	if _, err := ch.QueueDeclare(QueueRequests, true, false, false, false, requests); err != nil {
		return fmt.Errorf("declare queue %s: %w", QueueRequests, err)
	}
	if err := ch.QueueBind(QueueRequests, bindingPattern, ExchangeGeneration, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", QueueRequests, err)
	}
	return nil
}
