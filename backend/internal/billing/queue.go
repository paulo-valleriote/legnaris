package billing

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
)

const (
	ConsumerQueueName = "billing_events"
	ConsumerInterval  = 30 * time.Second
)

type consumerStore struct {
	ledger Ledger
}

func (s consumerStore) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[Event], error) {
	events, err := s.ledger.ClaimEvents(ctx, claimer, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[Event], 0, len(events))
	for _, ev := range events {
		jobs = append(jobs, sweeper.Job[Event]{ID: ev.ID, Attempts: ev.Attempts, Payload: ev})
	}
	return jobs, nil
}

func (s consumerStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, reason string) error {
	return s.ledger.RescheduleEvent(ctx, id, nextAt, reason)
}

func (s consumerStore) MarkDead(ctx context.Context, id string, _ time.Time, reason string) error {
	return s.ledger.FailEvent(ctx, id, reason)
}

func (s consumerStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.ledger.OldestReceived(ctx)
}

func NewConsumerQueue(deps ConsumerDeps, attemptCap, batch int) (sweeper.Queue, error) {
	consumer := NewConsumer(deps)
	return sweeper.NewQueue(sweeper.QueueSpec[Event]{
		Name:       ConsumerQueueName,
		Interval:   ConsumerInterval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      consumerStore{ledger: deps.Ledger},
		Handle:     consumer.Handle,
	})
}
