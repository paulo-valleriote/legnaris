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

const defaultConfirmTimeout = 5 * time.Second

var ErrBrokerUnavailable = errors.New("message broker is unavailable")

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

type Config struct {
	ConfirmTimeout time.Duration
	Prefetch       int
	RetryDelays    RetryDelays
	Logger         logger.StructuredLogger
}

type Broker struct {
	conn           connection
	ch             channel
	returns        chan amqp.Return
	logger         logger.StructuredLogger
	confirmTimeout time.Duration
	prefetch       int
	retryDelays    RetryDelays

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

func Dial(url string, cfg Config) (*Broker, error) {
	return dialWith(defaultDialer, url, cfg)
}

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

func (b *Broker) Publish(ctx context.Context, msg email.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%w: encode message: %v", email.ErrTransient, err)
	}

	props := provision(msg)
	props.Body = body
	return b.publish(ctx, exchangeMain, routingKeyOTP, props)
}

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
