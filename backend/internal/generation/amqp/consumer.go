package amqp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/shared/idgen"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

const attrError = "error"

const DefaultPrefetch = 4

const DefaultRetryDelay = 5 * time.Second

type Handler func(ctx context.Context, msg generation.Message) error

type ConsumerConfig struct {
	URL        string
	Prefetch   int
	RetryDelay time.Duration
}

type Acknowledger interface {
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
	Reject(requeue bool) error
}

type Consumer struct {
	url        string
	prefetch   int
	retryDelay time.Duration
	handler    Handler
	logger     logger.StructuredLogger
}

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

func decodeMessage(body []byte) (generation.Message, error) {
	var msg generation.Message
	if err := sharedjson.DecodeBytes(body, &msg); err != nil {
		return generation.Message{}, fmt.Errorf("decode job message: %w", err)
	}
	if err := idgen.Validate(msg.ID); err != nil {
		return generation.Message{}, fmt.Errorf("job message id: %w", err)
	}
	return msg, nil
}

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
