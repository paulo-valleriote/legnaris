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

const (
	ExchangeGeneration = "generation"
	QueueRequests      = "generation.requests"
	ExchangeDead       = "generation.dead"
	QueueDead          = "generation.dead"
	KeyDead            = "generation.dead"
	bindingPattern     = "generation.*.requested"

	argQueueType      = "x-queue-type"
	argDeadLetterExch = "x-dead-letter-exchange"
	argDeadLetterKey  = "x-dead-letter-routing-key"
	queueTypeQuorum   = "quorum"
)

const DefaultConfirmTimeout = 5 * time.Second

var ErrPublishRefused = errors.New("broker refused the generation job")

func RoutingKey(kind string) string {
	return "generation." + kind + ".requested"
}

type Config struct {
	URL            string
	ConfirmTimeout time.Duration
}

type Publisher struct {
	url            string
	confirmTimeout time.Duration
	dial           dialer

	mu      sync.Mutex
	conn    connection
	ch      channel
	returns *unroutable
}

type unroutable struct {
	returns chan amqp091.Return

	mu  sync.Mutex
	ids map[string]struct{}
}

func newUnroutable(returns chan amqp091.Return) *unroutable {
	return &unroutable{returns: returns, ids: map[string]struct{}{}}
}

func (u *unroutable) take(messageID string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.drainLocked()
	_, returned := u.ids[messageID]
	delete(u.ids, messageID)
	return returned
}

func (u *unroutable) drainLocked() {
	for {
		select {
		case r := <-u.returns:
			u.ids[r.MessageId] = struct{}{}
		default:
			return
		}
	}
}

func New(cfg Config) *Publisher {
	timeout := cfg.ConfirmTimeout
	if timeout <= 0 {
		timeout = DefaultConfirmTimeout
	}
	return &Publisher{url: cfg.URL, confirmTimeout: timeout, dial: defaultDialer}
}

func (p *Publisher) Publish(ctx context.Context, msg generation.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode generation job: %w", err)
	}

	ch, returns, err := p.acquire()
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
		p.reset(ch)
		return fmt.Errorf("publish generation job: %w", err)
	}
	return p.awaitConfirm(ctx, ch, returns, pending, msg.ID)
}

func (p *Publisher) acquire() (channel, *unroutable, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch == nil {
		if err := p.dialLocked(); err != nil {
			return nil, nil, err
		}
	}
	return p.ch, p.returns, nil
}

func (p *Publisher) awaitConfirm(ctx context.Context, ch channel, returns *unroutable, pending confirmation, messageID string) error {
	confirmCtx, cancel := context.WithTimeout(ctx, p.confirmTimeout)
	defer cancel()

	acked, err := pending.WaitContext(confirmCtx)
	if err != nil {
		p.reset(ch)
		return fmt.Errorf("%w: confirm wait: %w", ErrPublishRefused, err)
	}
	returned := returns.take(messageID)
	if !acked {
		return fmt.Errorf("%w: nacked", ErrPublishRefused)
	}
	if returned {
		return fmt.Errorf("%w: unroutable", ErrPublishRefused)
	}
	return nil
}

func (p *Publisher) dialLocked() error {
	conn, err := p.dial(p.url)
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}
	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	p.conn = conn
	p.ch = ch
	p.returns = newUnroutable(ch.NotifyReturn(make(chan amqp091.Return, 1)))
	return nil
}

func (p *Publisher) reset(failed channel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch == nil || p.ch != failed {
		return
	}
	p.resetLocked()
}

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

func (p *Publisher) DeclareTopology() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		return nil
	}
	return p.dialLocked()
}

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
