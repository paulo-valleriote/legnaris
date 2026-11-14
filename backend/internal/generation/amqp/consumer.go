package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// attrError is the log key of a failure reason.
const attrError = "error"

// DefaultPrefetch bounds the unacknowledged jobs one consumer holds (LGN-230).
const DefaultPrefetch = 4

// DefaultRetryDelay is the pause before a reconnect or a requeue after a failed job.
const DefaultRetryDelay = 5 * time.Second

// Handler runs one job. A nil return acknowledges the message; an error requeues it.
type Handler func(ctx context.Context, msg generation.Message) error

// ConsumerConfig configures the generation consumer.
type ConsumerConfig struct {
	// URL is the AMQP URI of the broker.
	URL string
	// Prefetch bounds the unacknowledged jobs. Zero uses DefaultPrefetch.
	Prefetch int
	// RetryDelay is the pause before a reconnect or a requeue. Zero uses DefaultRetryDelay.
	RetryDelay time.Duration
}

// Acknowledger settles one delivery. amqp091.Delivery satisfies it.
type Acknowledger interface {
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
	Reject(requeue bool) error
}

// Consumer consumes generation jobs from the requests queue with manual
// acknowledgement. It implements the worker loop of ADR 0007: a job is acknowledged
// only after its terminal state is written.
type Consumer struct {
	url        string
	prefetch   int
	retryDelay time.Duration
	handler    Handler
	logger     logger.StructuredLogger
}

// NewConsumer builds a consumer that dials on Run. Zero config values take their
// defaults.
func NewConsumer(cfg ConsumerConfig, handler Handler, log logger.StructuredLogger) *Consumer {
	prefetch := cfg.Prefetch
	if prefetch <= 0 {
		prefetch = DefaultPrefetch
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = DefaultRetryDelay
	}
	return &Consumer{url: cfg.URL, prefetch: prefetch, retryDelay: retryDelay, handler: handler, logger: log}
}

// Run consumes until ctx is cancelled. A lost connection or channel is logged and
// the consumer dials again after the retry delay, so a broker restart does not stop
// the worker.
func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		err := c.session(ctx)
		if ctx.Err() != nil {
			return nil
		}
		c.logger.Error(ctx, "generation consumer stopped; reconnecting",
			logger.Attribute{Key: attrError, Value: err.Error()})
		if !pause(ctx, c.retryDelay) {
			return nil
		}
	}
	return nil
}

// session dials, declares the topology and consumes until the stream ends or ctx is
// cancelled. In-flight jobs finish before the session returns.
func (c *Consumer) session(ctx context.Context) error {
	conn, err := amqp091.Dial(c.url)
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := declareTopology(amqpChannel{ch: ch}); err != nil {
		return err
	}
	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		return fmt.Errorf("set prefetch: %w", err)
	}
	deliveries, err := ch.Consume(QueueRequests, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", QueueRequests, err)
	}

	var inflight sync.WaitGroup
	defer inflight.Wait()
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("delivery stream closed")
			}
			inflight.Add(1)
			go func() {
				defer inflight.Done()
				processDelivery(ctx, delivery.Body, delivery, c.handler, c.retryDelay, c.logger)
			}()
		}
	}
}

// processDelivery decodes one message and settles it. A message that cannot be
// decoded or names no valid job is rejected without requeue, so it dead-letters for
// an operator. A job that fails is requeued after the retry delay. Only a completed
// job is acknowledged.
func processDelivery(ctx context.Context, body []byte, ack Acknowledger, handle Handler, retryDelay time.Duration, log logger.StructuredLogger) {
	msg, err := decodeMessage(body)
	if err != nil {
		log.Warn(ctx, "generation message rejected", logger.Attribute{Key: attrError, Value: err.Error()})
		_ = ack.Reject(false)
		return
	}
	if err := handle(ctx, msg); err != nil {
		log.Warn(ctx, "generation job not settled; requeued",
			logger.Attribute{Key: "generation_id", Value: msg.ID},
			logger.Attribute{Key: attrError, Value: err.Error()})
		pause(ctx, retryDelay)
		_ = ack.Nack(false, true)
		return
	}
	_ = ack.Ack(false)
}

// decodeMessage parses a job message and checks its identifier, so a malformed
// message never reaches the worker.
func decodeMessage(body []byte) (generation.Message, error) {
	var msg generation.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return generation.Message{}, fmt.Errorf("decode job message: %w", err)
	}
	if err := idgen.Validate(msg.ID); err != nil {
		return generation.Message{}, fmt.Errorf("job message id: %w", err)
	}
	return msg, nil
}

// pause waits d or until ctx ends. It reports whether the full pause elapsed.
func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
