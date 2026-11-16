package amqp

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultBuffer         = 1024
	DefaultConfirmTimeout = 5 * time.Second
	DefaultRetryInterval  = 5 * time.Second
	DefaultDrainTimeout   = 5 * time.Second
)

var errURLNotSet = errors.New("RABBITMQ_URL is not set")

type Config struct {
	URL            string
	Recorder       events.DropRecorder
	Logger         logger.StructuredLogger
	Buffer         int
	ConfirmTimeout time.Duration
	RetryInterval  time.Duration

	dial dialer
	now  func() time.Time
}

type job struct {
	eventType string
	id        string
	body      []byte
	at        time.Time
}

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

	mu     sync.RWMutex
	closed bool

	closeOnce sync.Once
	abandon   atomic.Bool

	conn        connection
	ch          channel
	confirms    chan amqp091.Confirmation
	lastAttempt time.Time
}

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

func (p *Publisher) deliver(j job) {
	if !p.ensureConnected() {
		p.drop(j.eventType, events.ReasonBrokerUnavailable)
		return
	}
	if reason := p.send(j); reason != "" {
		p.drop(j.eventType, reason)
	}
}

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

func (p *Publisher) open() error {
	if p.url == "" {
		return errURLNotSet
	}

	conn, ch, err := openConfirmedChannel(p.dial, p.url)
	if err != nil {
		return err
	}

	p.conn = conn
	p.ch = ch
	p.confirms = ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	return nil
}

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

func (p *Publisher) drop(eventType, reason string) {
	p.recorder.EventDropped(eventType, reason)
	p.logger.Warn(
		context.Background(),
		"domain event dropped",
		logger.Attribute{Key: "type", Value: eventType},
		logger.Attribute{Key: "reason", Value: reason},
	)
}

type discardRecorder struct{}

func (discardRecorder) EventDropped(string, string) {}
