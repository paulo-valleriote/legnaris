package worker

import (
	"context"
	"fmt"

	"legnaris/backend/internal/email"
	"legnaris/backend/shared/logger"
)

func tierFor(attempt int) email.Destination {
	switch {
	case attempt <= 0:
		return email.DestinationRetry30s
	case attempt == 1:
		return email.DestinationRetry2m
	default:
		return email.DestinationRetry5m
	}
}

func (w *Worker) retry(ctx context.Context, delivery Delivery, msg email.Message, attempt int) {
	next := msg
	next.Attempt = attempt + 1

	if err := w.republisher.PublishTo(ctx, tierFor(attempt), next); err != nil {
		w.leaveUnacknowledged(ctx, msg, "retry republish", err)
		return
	}

	w.logger.Info(ctx, "send failed; scheduled for retry", describe(next)...)
	w.observer.SendResult(ResultRetried)
	w.acknowledge(ctx, delivery, msg)
}

func (w *Worker) deadLetter(ctx context.Context, delivery Delivery, msg email.Message) {
	if err := w.republisher.PublishTo(ctx, email.DestinationDeadLetter, msg); err != nil {
		w.leaveUnacknowledged(ctx, msg, "dead-letter publish", err)
		return
	}

	w.observer.SendResult(ResultDeadLettered)
	w.observer.MessageDead()
	w.acknowledge(ctx, delivery, msg)
}

func (w *Worker) deadLetterBody(ctx context.Context, delivery Delivery) {
	if err := w.republisher.PublishBodyTo(ctx, email.DestinationDeadLetter, delivery.Body); err != nil {
		w.logger.Error(ctx, "dead-letter publish not confirmed; the broker will redeliver",
			logger.Attribute{Key: attrError, Value: fmt.Errorf("%w: %w", ErrPublishUnconfirmed, err).Error()})
		return
	}

	w.observer.SendResult(ResultDeadLettered)
	w.observer.MessageDead()
	if err := delivery.Handle.Ack(); err != nil {
		w.logger.Error(ctx, "acknowledgement failed; the broker will redeliver",
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
}

func (w *Worker) leaveUnacknowledged(ctx context.Context, msg email.Message, step string, cause error) {
	w.logger.Error(ctx, step+" not confirmed; the broker will redeliver", append(describe(msg),
		logger.Attribute{Key: attrError, Value: fmt.Errorf("%w: %w", ErrPublishUnconfirmed, cause).Error()})...)
}
