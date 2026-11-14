// Package relay publishes guaranteed domain events from the event outbox. It
// claims a batch with a lease, publishes each entry through the Publisher port
// with confirms, marks it published on success and applies a bounded backoff on
// failure. It never writes Mongo directly; all state moves through the outbox
// port (ADR 0010, ADR 0013).
package relay

import (
	"context"
	"time"

	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

const attrError = "error"

// Publisher publishes one encoded entry to the broker and returns nil only when
// the broker confirmed it. Any other outcome is a transient failure, and the
// relay reschedules the entry.
type Publisher interface {
	Publish(ctx context.Context, entry outbox.Entry) error
}

// Default relay settings, applied by Config when a field is zero.
const (
	DefaultPollInterval   = time.Second
	DefaultLease          = 30 * time.Second
	DefaultBatch          = 25
	DefaultConfirmTimeout = 5 * time.Second
	DefaultMaxAttempts    = 5
)

// DefaultBackoff is the increasing delay schedule between publish attempts. The
// last entry repeats once the schedule runs out.
var DefaultBackoff = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	time.Minute,
	5 * time.Minute,
}

// Config tunes the relay. A zero field takes its default in withDefaults.
type Config struct {
	PollInterval   time.Duration
	Lease          time.Duration
	Batch          int
	ConfirmTimeout time.Duration
	MaxAttempts    int
	Backoff        []time.Duration
	Now            func() time.Time
	ClaimerID      string
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultPollInterval
	}
	if c.Lease <= 0 {
		c.Lease = DefaultLease
	}
	if c.Batch <= 0 {
		c.Batch = DefaultBatch
	}
	if c.ConfirmTimeout <= 0 {
		c.ConfirmTimeout = DefaultConfirmTimeout
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if len(c.Backoff) == 0 {
		c.Backoff = DefaultBackoff
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Relay polls the event outbox and moves guaranteed events to the broker.
type Relay struct {
	repository outbox.Repository
	publisher  Publisher
	config     Config
	logger     logger.StructuredLogger
}

// New builds a relay over the outbox store and the broker publisher.
func New(repository outbox.Repository, publisher Publisher, config Config, log logger.StructuredLogger) *Relay {
	return &Relay{repository: repository, publisher: publisher, config: config, logger: log}
}

// Run runs one pass, then waits PollInterval, until ctx is cancelled. It
// finishes the in-flight batch before returning nil.
func (r *Relay) Run(ctx context.Context) error {
	config := r.config.withDefaults()
	for {
		r.pass(ctx, config)
		if ctx.Err() != nil {
			return nil
		}

		timer := time.NewTimer(config.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// RunOnce claims and processes a single batch. It is the deterministic seam the
// tests drive.
func (r *Relay) RunOnce(ctx context.Context) {
	r.pass(ctx, r.config.withDefaults())
}

func (r *Relay) pass(ctx context.Context, config Config) {
	entries, err := r.repository.Claim(ctx, config.ClaimerID, config.Now(), config.Lease, config.Batch)
	if err != nil {
		r.logger.Error(ctx, "events relay claim failed", logger.Attribute{Key: attrError, Value: err.Error()})
		return
	}
	for _, entry := range entries {
		r.publish(ctx, config, entry)
	}
}

// publish moves one claimed entry. A non-canonical identity is terminal without
// a broker call. A confirmed publish marks the entry published; any other
// outcome backs off or fails the entry at the attempt cap.
func (r *Relay) publish(ctx context.Context, config Config, entry outbox.Entry) {
	now := config.Now()
	if err := idgen.Validate(entry.ID); err != nil {
		r.markTerminal(ctx, entry.ID, now, "invalid identity")
		return
	}

	publishCtx, cancel := context.WithTimeout(ctx, config.ConfirmTimeout)
	err := r.publisher.Publish(publishCtx, entry)
	cancel()
	if err != nil {
		r.backOffOrFail(ctx, config, entry, err.Error(), now)
		return
	}

	if err := r.repository.MarkPublished(ctx, entry.ID, now); err != nil {
		r.logger.Error(ctx, "events relay mark published failed",
			logger.Attribute{Key: "id", Value: entry.ID},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
}

func (r *Relay) backOffOrFail(ctx context.Context, config Config, entry outbox.Entry, reason string, now time.Time) {
	if entry.Attempts+1 >= config.MaxAttempts {
		r.markTerminal(ctx, entry.ID, now, reason)
		return
	}

	nextAt := now.Add(backoff(config.Backoff, entry.Attempts+1))
	if err := r.repository.Reschedule(ctx, entry.ID, now, nextAt, reason); err != nil {
		r.logger.Error(ctx, "events relay reschedule failed",
			logger.Attribute{Key: "id", Value: entry.ID},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
}

func (r *Relay) markTerminal(ctx context.Context, id string, now time.Time, reason string) {
	if err := r.repository.MarkTerminal(ctx, id, now, reason); err != nil {
		r.logger.Error(ctx, "events relay mark terminal failed",
			logger.Attribute{Key: "id", Value: id},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
}

// backoff returns the delay for the given attempt number, repeating the last
// tier once the schedule runs out.
func backoff(schedule []time.Duration, attempt int) time.Duration {
	if len(schedule) == 0 {
		return 0
	}
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(schedule) {
		index = len(schedule) - 1
	}
	return schedule[index]
}
