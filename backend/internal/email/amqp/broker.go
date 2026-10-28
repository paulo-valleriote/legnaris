package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

// defaultConfirmTimeout bounds the publisher-confirm wait when the caller does
// not set one, so a stalled broker can never pin the relay loop (EML-07).
const defaultConfirmTimeout = 5 * time.Second

// ErrBrokerUnavailable reports an absent, unusable or unreachable broker
// configuration at startup. Effect: the relay fails fast and never polls
// (EML-15).
var ErrBrokerUnavailable = errors.New("message broker is unavailable")

// ValidateURL rejects an absent or unusable broker URL with the typed startup
// error, so a process that needs the broker never begins work without one
// (EML-15, WKR-19).
func ValidateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%w: RABBITMQ_URL is not set", ErrBrokerUnavailable)
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") || parsed.Host == "" {
		return fmt.Errorf("%w: RABBITMQ_URL is not a valid AMQP URI", ErrBrokerUnavailable)
	}
	return nil
}

// Config configures a broker connection.
type Config struct {
	// ConfirmTimeout bounds each publish's confirmation wait.
	ConfirmTimeout time.Duration
	// Prefetch bounds the unacknowledged deliveries the consumer holds; zero
	// leaves the broker's default (WKR-16).
	Prefetch int
	// RetryDelays sets the retry tiers declared with the topology; zero tiers
	// use their defaults (WKR-14).
	RetryDelays RetryDelays
	// Logger records state changes and blocking notifications.
	Logger logger.StructuredLogger
}

// Broker is a RabbitMQ connection with a publisher-confirm channel.
type Broker struct {
	conn           connection
	ch             channel
	returns        chan amqp.Return
	logger         logger.StructuredLogger
	confirmTimeout time.Duration
	prefetch       int
	retryDelays    RetryDelays

	// publishMu serializes publishes: the worker publishes from concurrent
	// handlers and the confirm bookkeeping below is per channel.
	publishMu sync.Mutex

	consumerMu  sync.Mutex
	consumerCh  channel
	consumerSeq int

	mu       sync.Mutex
	blocked  bool
	unrouted map[string]struct{}

	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// Dial opens a real broker connection with automatic reconnection and topology
// recovery enabled (EML-06).
func Dial(url string, cfg Config) (*Broker, error) {
	return dialWith(defaultDialer, url, cfg)
}

// dialWith is the testable seam: a fake dialer supplies a fake connection so
// the publish and declaration paths need no broker (EML-07).
func dialWith(dial dialer, url string, cfg Config) (*Broker, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("%w: RABBITMQ_URL is not set", ErrBrokerUnavailable)
	}

	conn, err := dial(url, amqp.Config{Recovery: &amqp.Recovery{}})
	if err != nil {
		return nil, fmt.Errorf("%w: dial broker: %v", ErrBrokerUnavailable, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: open channel: %v", ErrBrokerUnavailable, err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("%w: enable publisher confirms: %v", ErrBrokerUnavailable, err)
	}

	confirmTimeout := cfg.ConfirmTimeout
	if confirmTimeout <= 0 {
		confirmTimeout = defaultConfirmTimeout
	}

	broker := &Broker{
		conn:           conn,
		ch:             ch,
		returns:        ch.NotifyReturn(make(chan amqp.Return, 1)),
		logger:         cfg.Logger,
		confirmTimeout: confirmTimeout,
		prefetch:       cfg.Prefetch,
		retryDelays:    cfg.RetryDelays,
		unrouted:       make(map[string]struct{}),
		done:           make(chan struct{}),
	}

	stateChanges := make(chan *amqp.StateChanged, 1)
	conn.NotifyStateChange(stateChanges)
	blocked := conn.NotifyBlocked(make(chan amqp.Blocking, 1))
	broker.wg.Add(1)
	go broker.supervise(stateChanges, blocked)

	return broker, nil
}

// DeclareTopology declares the email topology idempotently, before the first
// publish; a declaration failure stops startup (EML-06).
func (b *Broker) DeclareTopology() error {
	delays, err := b.retryDelays.resolve()
	if err != nil {
		return fmt.Errorf("declare email topology: %w", err)
	}
	if err := emailTopology(delays).declare(b.ch); err != nil {
		return fmt.Errorf("declare email topology: %w", err)
	}
	return nil
}

// Publish encodes the message and publishes it mandatorily and persistently
// to the work queue, then waits for the publisher confirm. It reports nil only
// when the broker accepted the message for routing: a nack, a confirm timeout, a
// transport error, a blocked connection or a recorded basic.return are
// transient, so the relay reschedules instead of marking the row published
// (EML-02, EML-07).
func (b *Broker) Publish(ctx context.Context, msg email.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%w: encode message: %v", email.ErrTransient, err)
	}

	props := provision(msg)
	props.Body = body
	return b.publish(ctx, exchangeMain, routingKeyOTP, props)
}

// PublishTo publishes the message to a destination the domain names, with the
// same guarantees as Publish (WKR-09, WKR-11, WKR-13).
func (b *Broker) PublishTo(ctx context.Context, destination email.Destination, msg email.Message) error {
	exchange, key, err := route(destination)
	if err != nil {
		return err
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%w: encode message: %v", email.ErrTransient, err)
	}

	props := provision(msg)
	props.Body = body
	return b.publish(ctx, exchange, key, props)
}

// PublishBodyTo publishes a body that could not be decoded, unchanged, under a
// fresh identity so an unroutable publish is still detected (WKR-04).
func (b *Broker) PublishBodyTo(ctx context.Context, destination email.Destination, body []byte) error {
	exchange, key, err := route(destination)
	if err != nil {
		return err
	}

	id, err := idgen.New()
	if err != nil {
		return fmt.Errorf("%w: %v", email.ErrTransient, err)
	}

	props := provision(email.Message{ID: id})
	props.Body = body
	return b.publish(ctx, exchange, key, props)
}

func (b *Broker) publish(ctx context.Context, exchange, key string, props amqp.Publishing) error {
	if b.isBlocked() {
		return fmt.Errorf("%w: connection blocked", email.ErrTransient)
	}

	b.publishMu.Lock()
	defer b.publishMu.Unlock()

	pending, err := b.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, true, false, props)
	if err != nil {
		return fmt.Errorf("%w: publish: %v", email.ErrTransient, err)
	}

	confirmCtx, cancel := context.WithTimeout(ctx, b.confirmTimeout)
	defer cancel()

	acked, err := pending.WaitContext(confirmCtx)
	if err != nil {
		return fmt.Errorf("%w: confirm wait: %v", email.ErrTransient, err)
	}
	if !acked {
		return fmt.Errorf("%w: broker nacked the message", email.ErrTransient)
	}

	b.drainReturns()
	if b.takeReturned(props.MessageId) {
		return fmt.Errorf("%w: message unroutable", email.ErrTransient)
	}
	return nil
}

// Close stops the supervisor and closes the channel and connection.
func (b *Broker) Close() error {
	b.closeOnce.Do(func() { close(b.done) })
	b.wg.Wait()

	b.closeConsumer()
	if b.ch != nil {
		_ = b.ch.Close()
	}
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

// supervise drains the connection state and blocking notifications, so the
// library's goroutines do not leak and the relay can log a broker outage
// instead of exiting (EML-06).
func (b *Broker) supervise(stateChanges <-chan *amqp.StateChanged, blocked <-chan amqp.Blocking) {
	defer b.wg.Done()

	for {
		select {
		case <-b.done:
			return
		case change, ok := <-stateChanges:
			if !ok {
				stateChanges = nil
				continue
			}
			b.logger.Info(context.Background(), "broker state changed",
				logger.Attribute{Key: "from", Value: string(change.From)},
				logger.Attribute{Key: "to", Value: string(change.To)})
		case notification, ok := <-blocked:
			if !ok {
				blocked = nil
				continue
			}
			b.setBlocked(notification.Active)
			b.logger.Warn(context.Background(), "broker blocking changed",
				logger.Attribute{Key: "active", Value: fmt.Sprintf("%t", notification.Active)},
				logger.Attribute{Key: "reason", Value: notification.Reason})
		}
	}
}

// drainReturns records every currently available basic.return without blocking.
func (b *Broker) drainReturns() {
	for {
		select {
		case returned := <-b.returns:
			b.recordReturn(returned.MessageId)
		default:
			return
		}
	}
}

func (b *Broker) recordReturn(messageID string) {
	if messageID == "" {
		return
	}
	b.mu.Lock()
	b.unrouted[messageID] = struct{}{}
	b.mu.Unlock()
}

// takeReturned reports whether a basic.return was recorded for the identity and
// clears it.
func (b *Broker) takeReturned(messageID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.unrouted[messageID]; !ok {
		return false
	}
	delete(b.unrouted, messageID)
	return true
}

func (b *Broker) isBlocked() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocked
}

func (b *Broker) setBlocked(blocked bool) {
	b.mu.Lock()
	b.blocked = blocked
	b.mu.Unlock()
}
