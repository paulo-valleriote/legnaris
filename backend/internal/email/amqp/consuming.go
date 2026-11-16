package amqp

import (
	"context"
	"fmt"
	"os"

	"legnaris/backend/internal/email/worker"
	"legnaris/backend/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ackHandle struct {
	delivery amqp.Delivery
}

func (h ackHandle) Ack() error {
	return h.delivery.Ack(false)
}

func (b *Broker) Consume(ctx context.Context) (<-chan worker.Delivery, error) {
	b.consumerMu.Lock()
	defer b.consumerMu.Unlock()

	b.closeConsumerLocked()

	ch, err := b.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("%w: open consumer channel: %v", ErrBrokerUnavailable, err)
	}

	if b.prefetch > 0 {
		if err := ch.Qos(b.prefetch, 0, false); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("%w: set prefetch: %v", ErrBrokerUnavailable, err)
		}
	}

	b.consumerSeq++
	tag := consumerTag(b.consumerSeq)
	deliveries, err := ch.Consume(queueWork, tag, false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("%w: register consumer: %v", ErrBrokerUnavailable, err)
	}
	b.consumerCh = ch

	stream := make(chan worker.Delivery)
	b.wg.Add(1)
	go b.forward(ctx, ch, tag, deliveries, stream)
	return stream, nil
}

func (b *Broker) forward(ctx context.Context, ch channel, tag string, deliveries <-chan amqp.Delivery, stream chan<- worker.Delivery) {
	defer b.wg.Done()
	defer close(stream)

	for {
		select {
		case <-ctx.Done():
			if err := ch.Cancel(tag, false); err != nil {
				b.logger.Warn(context.Background(), "cancel consumer failed", logger.Attribute{Key: "error", Value: err.Error()})
			}
			return
		case <-b.done:
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			select {
			case stream <- worker.Delivery{Body: delivery.Body, Handle: ackHandle{delivery: delivery}}:
			case <-ctx.Done():
			case <-b.done:
				return
			}
		}
	}
}

func (b *Broker) closeConsumer() {
	b.consumerMu.Lock()
	defer b.consumerMu.Unlock()
	b.closeConsumerLocked()
}

func (b *Broker) closeConsumerLocked() {
	if b.consumerCh != nil {
		_ = b.consumerCh.Close()
		b.consumerCh = nil
	}
}

func consumerTag(seq int) string {
	host, err := os.Hostname()
	if err != nil {
		host = "worker"
	}
	return fmt.Sprintf("legnaris-email-%s-%d-%d", host, os.Getpid(), seq)
}
