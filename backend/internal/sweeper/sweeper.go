package sweeper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/logger"
)

const Lease = 5 * time.Minute

var ErrInvalidQueue = errors.New("invalid sweeper queue")

var ErrPermanent = errors.New("permanent failure")

type Job[T any] struct {
	ID       string
	Attempts int
	Payload  T
}

type Store[T any] interface {
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Job[T], error)
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, reason string) error
	MarkDead(ctx context.Context, id string, now time.Time, reason string) error
	OldestPending(ctx context.Context) (time.Time, bool, error)
}

type Handler[T any] func(ctx context.Context, job Job[T], now time.Time) error

type Observer interface {
	OldestPending(queue string, age time.Duration)
	Failed(queue string)
}

type QueueSpec[T any] struct {
	Name       string
	Interval   time.Duration
	AttemptCap int
	Batch      int
	Store      Store[T]
	Handle     Handler[T]
}

type Queue struct {
	Name     string
	Interval time.Duration
	sweep    func(ctx context.Context, now time.Time, claimer string, obs Observer, log logger.StructuredLogger) error
}

func NewQueue[T any](spec QueueSpec[T]) (Queue, error) {
	if err := spec.validate(); err != nil {
		return Queue{}, err
	}
	return Queue{
		Name:     spec.Name,
		Interval: spec.Interval,
		sweep: func(ctx context.Context, now time.Time, claimer string, obs Observer, log logger.StructuredLogger) error {
			return sweepQueue(ctx, spec, now, claimer, obs, log)
		},
	}, nil
}

func (spec QueueSpec[T]) validate() error {
	switch {
	case spec.Name == "":
		return fmt.Errorf("%w: empty name", ErrInvalidQueue)
	case spec.Interval <= 0:
		return fmt.Errorf("%w: %s: interval must be positive", ErrInvalidQueue, spec.Name)
	case spec.AttemptCap <= 0:
		return fmt.Errorf("%w: %s: attempt cap must be positive", ErrInvalidQueue, spec.Name)
	case spec.Batch <= 0:
		return fmt.Errorf("%w: %s: batch must be positive", ErrInvalidQueue, spec.Name)
	case spec.Store == nil:
		return fmt.Errorf("%w: %s: nil store", ErrInvalidQueue, spec.Name)
	case spec.Handle == nil:
		return fmt.Errorf("%w: %s: nil handler", ErrInvalidQueue, spec.Name)
	}
	return nil
}

type Sweeper struct {
	queues   []Queue
	claimer  string
	observer Observer
	logger   logger.StructuredLogger
	lastRun  map[string]time.Time
	tick     time.Duration
}

func New(queues []Queue, claimer string, obs Observer, log logger.StructuredLogger) (*Sweeper, error) {
	if len(queues) == 0 {
		return nil, fmt.Errorf("%w: no queues registered", ErrInvalidQueue)
	}
	if obs == nil {
		return nil, fmt.Errorf("%w: nil observer", ErrInvalidQueue)
	}

	seen := make(map[string]struct{}, len(queues))
	tick := queues[0].Interval
	for _, q := range queues {
		if _, dup := seen[q.Name]; dup {
			return nil, fmt.Errorf("%w: duplicate name %s", ErrInvalidQueue, q.Name)
		}
		seen[q.Name] = struct{}{}
		tick = min(tick, q.Interval)
	}

	return &Sweeper{
		queues:   queues,
		claimer:  claimer,
		observer: obs,
		logger:   log,
		lastRun:  make(map[string]time.Time, len(queues)),
		tick:     tick,
	}, nil
}

func (s *Sweeper) RunOnce(ctx context.Context, now time.Time) error {
	var errs []error
	for _, q := range s.queues {
		if !s.due(q, now) {
			continue
		}
		s.lastRun[q.Name] = now
		if err := q.sweep(ctx, now, s.claimer, s.observer, s.logger); err != nil {
			errs = append(errs, fmt.Errorf("queue %s: %w", q.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Sweeper) due(q Queue, now time.Time) bool {
	last, ran := s.lastRun[q.Name]
	return !ran || now.Sub(last) >= q.Interval
}

func (s *Sweeper) Run(ctx context.Context) error {
	for {
		if err := s.RunOnce(ctx, time.Now()); err != nil {
			s.logger.Error(ctx, "sweeper pass failed", logger.Attribute{Key: "error", Value: err.Error()})
		}
		if ctx.Err() != nil {
			return nil
		}

		timer := time.NewTimer(s.tick)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func sweepQueue[T any](ctx context.Context, spec QueueSpec[T], now time.Time, claimer string, obs Observer, log logger.StructuredLogger) error {
	oldestErr := reportOldest(ctx, spec, now, obs)

	jobs, err := spec.Store.Claim(ctx, claimer, now, Lease, spec.Batch)
	if err != nil {
		return errors.Join(oldestErr, fmt.Errorf("claim: %w", err))
	}
	for _, job := range jobs {
		handleJob(ctx, spec, job, now, obs, log)
	}
	return oldestErr
}

func reportOldest[T any](ctx context.Context, spec QueueSpec[T], now time.Time, obs Observer) error {
	oldest, pending, err := spec.Store.OldestPending(ctx)
	if err != nil {
		return fmt.Errorf("oldest pending: %w", err)
	}
	if !pending {
		obs.OldestPending(spec.Name, 0)
		return nil
	}
	obs.OldestPending(spec.Name, now.Sub(oldest))
	return nil
}

func handleJob[T any](ctx context.Context, spec QueueSpec[T], job Job[T], now time.Time, obs Observer, log logger.StructuredLogger) {
	err := spec.Handle(ctx, job, now)
	if err == nil {
		return
	}
	obs.Failed(spec.Name)

	reason := err.Error()
	if errors.Is(err, ErrPermanent) || job.Attempts+1 >= spec.AttemptCap {
		if markErr := spec.Store.MarkDead(ctx, job.ID, now, reason); markErr != nil {
			logTransitionFailure(ctx, log, spec.Name, job.ID, "mark dead", markErr)
		}
		return
	}
	if rescheduleErr := spec.Store.Reschedule(ctx, job.ID, now, now, reason); rescheduleErr != nil {
		logTransitionFailure(ctx, log, spec.Name, job.ID, "reschedule", rescheduleErr)
	}
}

func logTransitionFailure(ctx context.Context, log logger.StructuredLogger, queue, id, action string, err error) {
	log.Error(ctx, "sweeper transition failed",
		logger.Attribute{Key: "queue", Value: queue},
		logger.Attribute{Key: "id", Value: id},
		logger.Attribute{Key: "action", Value: action},
		logger.Attribute{Key: "error", Value: err.Error()})
}
