// Package amqp is the best-effort RabbitMQ adapter for domain events (ADR 0010,
// ADR 0013). Publish never blocks and never fails the caller: events go into a
// bounded buffer drained by one worker goroutine that publishes with confirms.
// Anything that cannot be delivered is dropped, logged and counted in
// legnaris_events_dropped_total.
package amqp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Defaults applied when the matching Config field is zero.
const (
	DefaultBuffer         = 1024
	DefaultConfirmTimeout = 5 * time.Second
	// DefaultRetryInterval is the production reconnect cadence. A zero
	// Config.RetryInterval means every event may retry the connection.
	DefaultRetryInterval = 5 * time.Second
	// DefaultDrainTimeout is the SIGTERM budget for delivering the buffer
	// before the rest is counted as dropped with reason shutdown (ADR 0013).
	DefaultDrainTimeout = 5 * time.Second
)

// errURLNotSet reports an empty broker URL. The publisher still starts and
// drops every event as broker_unavailable, so the API serves without RabbitMQ.
var errURLNotSet = errors.New("RABBITMQ_URL is not set")

// Config configures the publisher.
type Config struct {
	// URL is the AMQP URI of the broker.
	URL string
	// Recorder counts dropped events. Nil discards the counts.
	Recorder events.DropRecorder
	// Logger records drops and connection failures. Nil logs at warn level.
	Logger logger.StructuredLogger
	// Buffer bounds the events waiting for the worker; zero uses DefaultBuffer.
	Buffer int
	// ConfirmTimeout bounds each publish's confirmation wait; zero uses DefaultConfirmTimeout.
	ConfirmTimeout time.Duration
	// RetryInterval is the minimum gap between connection attempts while the
	// broker is down. Zero retries on every event.
	RetryInterval time.Duration

	dial dialer
	now  func() time.Time
}

// job is one encoded event waiting for the worker.
type job struct {
	eventType string
	id        string
	body      []byte
	at        time.Time
}

// Publisher is the best-effort events.Publisher adapter. Create it with New and
// stop it with Shutdown.
type Publisher struct {
	url            string
	dial           dialer
	recorder       events.DropRecorder
	logger         logger.StructuredLogger
	confirmTimeout time.Duration
	retryInterval  time.Duration
	now            func() time.Time

	jobs chan job
	done chan struct{}

	// mu guards closed and the close of jobs: Publish holds it for reading
	// while it offers a job, Shutdown holds it for writing to close the buffer.
	mu     sync.RWMutex
	closed bool

	closeOnce sync.Once
	abandon   atomic.Bool

	// The fields below belong to the worker goroutine only.
	conn        connection
	ch          channel
	confirms    chan amqp091.Confirmation
	lastAttempt time.Time
}

// New starts the worker and returns a Publisher. It does not dial: the broker
// is contacted on the first event, so the API starts while RabbitMQ is down.
func New(cfg Config) *Publisher {
	p := &Publisher{
		url:            cfg.URL,
		dial:           cfg.dial,
		recorder:       cfg.Recorder,
		logger:         cfg.Logger,
		confirmTimeout: cfg.ConfirmTimeout,
		retryInterval:  cfg.RetryInterval,
		now:            cfg.now,
		done:           make(chan struct{}),
	}
	if p.dial == nil {
		p.dial = dialAMQP
	}
	if p.recorder == nil {
		p.recorder = discardRecorder{}
	}
	if p.logger == nil {
		p.logger = logger.NewLogger(slog.LevelWarn)
	}
	if p.confirmTimeout <= 0 {
		p.confirmTimeout = DefaultConfirmTimeout
	}
	if p.now == nil {
		p.now = time.Now
	}
	buffer := cfg.Buffer
	if buffer <= 0 {
		buffer = DefaultBuffer
	}
	p.jobs = make(chan job, buffer)

	go p.run()
	return p
}

// Publish encodes ev and offers it to the worker without waiting. It never
// blocks: a full buffer or a closed publisher drops the event and counts it.
func (p *Publisher) Publish(ev events.Event) {
	id, err := idgen.New()
	if err != nil {
		p.drop(ev.Type, events.ReasonInvalidEvent)
		return
	}

	at := p.now()
	body, err := events.Encode(ev, id, at)
	if err != nil {
		p.drop(ev.Type, events.ReasonInvalidEvent)
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		p.drop(ev.Type, events.ReasonShutdown)
		return
	}

	select {
	case p.jobs <- job{eventType: ev.Type, id: id, body: body, at: at}:
	default:
		p.drop(ev.Type, events.ReasonBufferFull)
	}
}

// Shutdown stops accepting events and waits for the worker to deliver what is
// buffered. When ctx expires first, the worker stops publishing and every
// remaining event is counted with reason shutdown. Shutdown is safe to call
// more than once.
func (p *Publisher) Shutdown(ctx context.Context) {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.jobs)
		p.mu.Unlock()
	})

	select {
	case <-p.done:
	case <-ctx.Done():
		p.abandon.Store(true)
		<-p.done
	}
}

// run drains the buffer until it is closed, then releases the connection.
func (p *Publisher) run() {
	defer close(p.done)
	defer p.reset()

	for j := range p.jobs {
		if p.abandon.Load() {
			p.drop(j.eventType, events.ReasonShutdown)
			continue
		}
		p.deliver(j)
	}
}

// deliver publishes one job or drops it with the reason that prevented it.
func (p *Publisher) deliver(j job) {
	if !p.ensureConnected() {
		p.drop(j.eventType, events.ReasonBrokerUnavailable)
		return
	}
	if reason := p.send(j); reason != "" {
		p.drop(j.eventType, reason)
	}
}

// ensureConnected reports whether a usable channel exists, dialing at most once
// per retry interval while the broker is down.
func (p *Publisher) ensureConnected() bool {
	if p.ch != nil {
		return true
	}
	now := p.now()
	if !p.lastAttempt.IsZero() && now.Sub(p.lastAttempt) < p.retryInterval {
		return false
	}
	p.lastAttempt = now

	if err := p.open(); err != nil {
		p.logger.Warn(context.Background(), "events broker unavailable", logger.Attribute{Key: "error", Value: err.Error()})
		return false
	}
	return true
}

// open dials the broker, enables confirms and declares the topology. On any
// failure it closes what it opened, so a half-open connection is never kept.
func (p *Publisher) open() error {
	if p.url == "" {
		return errURLNotSet
	}

	conn, err := p.dial(p.url)
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("enable publisher confirms: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	p.conn = conn
	p.ch = ch
	p.confirms = ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	return nil
}

// send publishes one job and waits for its confirmation. It returns the drop
// reason, or the empty string when the broker acked the message. A transport
// failure or a missing confirm resets the channel so the next job reconnects.
func (p *Publisher) send(j job) string {
	ctx, cancel := context.WithTimeout(context.Background(), p.confirmTimeout)
	defer cancel()

	props := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    j.id,
		Type:         j.eventType,
		Timestamp:    j.at,
		Body:         j.body,
	}
	if err := p.ch.PublishWithContext(ctx, ExchangeEvents, j.eventType, false, false, props); err != nil {
		p.reset()
		return events.ReasonBrokerUnavailable
	}

	select {
	case confirmation, ok := <-p.confirms:
		if !ok {
			p.reset()
			return events.ReasonBrokerUnavailable
		}
		if !confirmation.Ack {
			return events.ReasonPublishFailed
		}
		return ""
	case <-ctx.Done():
		p.reset()
		return events.ReasonPublishFailed
	}
}

// reset closes the worker's connection and channel. It is safe to call when
// nothing is open.
func (p *Publisher) reset() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.ch = nil
	p.conn = nil
	p.confirms = nil
}

// drop counts a dropped event and logs it at warn level.
func (p *Publisher) drop(eventType, reason string) {
	p.recorder.EventDropped(eventType, reason)
	p.logger.Warn(
		context.Background(),
		"domain event dropped",
		logger.Attribute{Key: "type", Value: eventType},
		logger.Attribute{Key: "reason", Value: reason},
	)
}

// discardRecorder is the Recorder used when none is configured.
type discardRecorder struct{}

func (discardRecorder) EventDropped(string, string) {}
