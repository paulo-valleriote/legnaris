package relay

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/idgen"
)

const QueueName = "event_outbox"

type Publisher interface {
	Publish(ctx context.Context, entry outbox.Entry) error
}

type Config struct {
	Interval       time.Duration
	Batch          int
	ConfirmTimeout time.Duration
	AttemptCap     int
}

func NewQueue(repository outbox.Repository, publisher Publisher, cfg Config) (sweeper.Queue, error) {
	if repository == nil || publisher == nil {
		return sweeper.Queue{}, fmt.Errorf("%w: %s: nil repository or publisher", sweeper.ErrInvalidQueue, QueueName)
	}
	if cfg.ConfirmTimeout <= 0 {
		return sweeper.Queue{}, fmt.Errorf("%w: %s: confirm timeout must be positive", sweeper.ErrInvalidQueue, QueueName)
	}

	return sweeper.NewQueue(sweeper.QueueSpec[outbox.Entry]{
		Name:       QueueName,
		Interval:   cfg.Interval,
		AttemptCap: cfg.AttemptCap,
		Batch:      cfg.Batch,
		Store:      store{repository: repository},
		Handle:     handler(publisher, repository, cfg.ConfirmTimeout),
	})
}

func handler(publisher Publisher, repository outbox.Repository, confirmTimeout time.Duration) sweeper.Handler[outbox.Entry] {
	return func(ctx context.Context, job sweeper.Job[outbox.Entry], now time.Time) error {
		if err := idgen.Validate(job.ID); err != nil {
			return fmt.Errorf("%w: invalid identity: %v", sweeper.ErrPermanent, err)
		}
		if err := publish(ctx, publisher, job.Payload, confirmTimeout); err != nil {
			return err
		}
		return repository.MarkPublished(ctx, job.ID, now)
	}
}

func publish(ctx context.Context, publisher Publisher, entry outbox.Entry, confirmTimeout time.Duration) error {
	publishCtx, cancel := context.WithTimeout(ctx, confirmTimeout)
	defer cancel()
	return publisher.Publish(publishCtx, entry)
}

type store struct {
	repository outbox.Repository
}

func (s store) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[outbox.Entry], error) {
	entries, err := s.repository.Claim(ctx, claimer, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[outbox.Entry], 0, len(entries))
	for _, entry := range entries {
		jobs = append(jobs, sweeper.Job[outbox.Entry]{ID: entry.ID, Attempts: entry.Attempts, Payload: entry})
	}
	return jobs, nil
}

func (s store) Reschedule(ctx context.Context, id string, now, nextAt time.Time, reason string) error {
	return s.repository.Reschedule(ctx, id, now, nextAt, reason)
}

func (s store) MarkDead(ctx context.Context, id string, now time.Time, reason string) error {
	return s.repository.MarkTerminal(ctx, id, now, reason)
}

func (s store) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.repository.OldestPending(ctx)
}
