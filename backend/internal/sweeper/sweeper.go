// Package sweeper drives every pending-work queue from one loop (ADR 0013). A
// queue is a claim over documents that record their own pending state: the
// sweeper leases a batch, runs a handler on each item, reschedules an item whose
// attempt failed and marks it dead once it reaches the attempt cap. Each queue
// has its own interval, and a queue is registered by one entry in the relay's
// table, never by a loop of its own.
package sweeper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/logger"
)

// Lease is how long a claimed item stays with its claimer before another sweeper
// may claim it again. A stopped sweeper therefore never strands an item for longer
// than one lease.
const Lease = 5 * time.Minute

// ErrInvalidQueue reports a queue or sweeper configuration that cannot run. It is
// a plain sentinel; check it with errors.Is.
var ErrInvalidQueue = errors.New("invalid sweeper queue")

// ErrPermanent marks a handler failure that retrying cannot fix. An item whose
// handler returns an error wrapping it is marked dead at once, whatever its
// attempt count.
var ErrPermanent = errors.New("permanent failure")

// Job is one claimed item. Attempts counts the failed attempts recorded before
// this claim, and Payload is the document the handler works on.
type Job[T any] struct {
	ID       string
	Attempts int
	Payload  T
}

// Store is the persistence port of one queue. Every method runs on the
// documents of that queue only.
type Store[T any] interface {
	// Claim leases up to limit eligible items for claimer until now+lease. An
	// item already leased and not yet expired is never returned.
	Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Job[T], error)
	// Reschedule returns a failed item to pending, eligible at nextAt, and counts
	// one more attempt.
	Reschedule(ctx context.Context, id string, now, nextAt time.Time, reason string) error
	// MarkDead moves an item to the dead state, where the sweeper never claims it
	// again and an operator takes over.
	MarkDead(ctx context.Context, id string, now time.Time, reason string) error
	// OldestPending returns the creation time of the oldest pending item. The bool
	// is false when no item is pending.
	OldestPending(ctx context.Context) (time.Time, bool, error)
}

// Handler performs one attempt on a claimed item. A nil return means the handler
// completed the item itself. An error sends the item to Reschedule, or to
// MarkDead when it is permanent or at the attempt cap.
type Handler[T any] func(ctx context.Context, job Job[T], now time.Time) error

// Observer receives the per-queue signals the metrics expose.
type Observer interface {
	// OldestPending reports the age of the oldest pending item; zero when none is
	// pending.
	OldestPending(queue string, age time.Duration)
	// Failed counts one failed handler attempt.
	Failed(queue string)
}

// QueueSpec is the description of one queue: the row a relay registers.
type QueueSpec[T any] struct {
	Name       string
	Interval   time.Duration
	AttemptCap int
	Batch      int
	Store      Store[T]
	Handle     Handler[T]
}

// Queue is a validated, type-erased queue ready for the sweeper table.
type Queue struct {
	Name     string
	Interval time.Duration
	sweep    func(ctx context.Context, now time.Time, claimer string, obs Observer, log logger.StructuredLogger) error
}

// NewQueue validates spec and erases its item type, so queues of different
// payloads share one table. A zero interval, attempt cap or batch, an empty name
// or a nil store or handler is ErrInvalidQueue.
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

// Sweeper runs the queues of one table. It is not safe for concurrent use: one
// goroutine drives RunOnce or Run.
type Sweeper struct {
	queues   []Queue
	claimer  string
	observer Observer
	logger   logger.StructuredLogger
	lastRun  map[string]time.Time
	tick     time.Duration
}

// New builds a sweeper over the queue table. The table must hold at least one
// queue, with unique names, and an observer is required.
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

// RunOnce runs every queue whose interval has elapsed since its last run, judged
// against now. A queue never run is due. It returns the joined errors of the
// queues that failed; the other queues still run.
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

// Run calls RunOnce with the wall clock, then waits for the shortest queue
// interval, until ctx is cancelled. It returns nil on cancellation and logs a
// failed pass without stopping.
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

// sweepQueue runs one queue pass: it reports the oldest pending age first, so the
// gauge reflects the backlog before this pass drains it, then claims a batch and
// handles each item.
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

// handleJob runs one attempt. A failure is counted, then the item is dead when the
// failure is permanent or this attempt reaches the cap, and rescheduled otherwise.
// A failing transition is logged; the item then keeps its lease until it expires.
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
