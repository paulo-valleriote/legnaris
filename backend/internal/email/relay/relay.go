// Package relay owns the publication loop: it claims a batch of stored
// intentions with a lease, publishes each to the broker through the Publisher
// port, marks the row published on a confirmed publish, and applies a bounded
// backoff on failure. It never writes Mongo directly; all state moves through
// the outbox port (EML-02…EML-06).
package relay

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

// attrError is the structured-log key for a failure cause.
const attrError = "error"

// Relay polls the outbox and moves intentions to the broker.
type Relay struct {
	repository outbox.Repository
	publisher  Publisher
	observer   Observer
	config     Config
	logger     logger.StructuredLogger
}

// New builds a relay over the outbox store, the broker publisher and the
// metrics observer.
func New(repository outbox.Repository, publisher Publisher, observer Observer, config Config, log logger.StructuredLogger) *Relay {
	return &Relay{
		repository: repository,
		publisher:  publisher,
		observer:   observer,
		config:     config,
		logger:     log,
	}
}

// Run polls on PollInterval until ctx is cancelled, finishing the in-flight
// batch before returning (EML-05).
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

// RunOnce claims and processes a single batch, then refreshes the backlog
// gauge. It is the deterministic seam the suite drives; Run polls it on an
// interval (EML-05).
func (r *Relay) RunOnce(ctx context.Context) {
	r.pass(ctx, r.config.withDefaults())
}

// pass claims and processes one batch, then refreshes the backlog gauge.
func (r *Relay) pass(ctx context.Context, config Config) {
	intents, err := r.repository.Claim(ctx, config.ClaimerID, config.Now(), config.Lease, config.Batch)
	if err != nil {
		r.logger.Error(ctx, "relay claim failed", logger.Attribute{Key: attrError, Value: err.Error()})
		return
	}

	r.reportBacklog(ctx)
	for _, intent := range intents {
		r.publish(ctx, config, intent)
	}
}

// publish moves one claimed intention: it rejects a non-canonical identity,
// marks published on a confirmed publish, and otherwise backs off or fails the
// row at the cap (EML-02, EML-04, EML-18, EML-19).
func (r *Relay) publish(ctx context.Context, config Config, intent outbox.Intent) {
	now := config.Now()
	if err := idgen.Validate(intent.ID); err != nil {
		r.markTerminal(ctx, intent.ID, now, "invalid identity")
		return
	}

	if err := r.publisher.Publish(ctx, encode(intent)); err != nil {
		r.backOffOrFail(ctx, config, intent, err.Error(), now)
		return
	}

	if err := r.repository.MarkPublished(ctx, intent.ID, now); err != nil {
		r.logger.Error(ctx, "relay mark published failed",
			logger.Attribute{Key: "id", Value: intent.ID},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
	r.notify(ResultPublished)
}

// backOffOrFail reschedules the row with the next tier delay, or marks it
// terminal once the publication-attempt cap is reached (EML-04).
func (r *Relay) backOffOrFail(ctx context.Context, config Config, intent outbox.Intent, reason string, now time.Time) {
	attempts := intent.Attempts + 1
	if attempts >= config.MaxAttempts {
		r.markTerminal(ctx, intent.ID, now, reason)
		return
	}

	if err := r.repository.Reschedule(ctx, intent.ID, now, now.Add(backoff(config.Backoff, attempts)), reason); err != nil {
		r.logger.Error(ctx, "relay reschedule failed",
			logger.Attribute{Key: "id", Value: intent.ID},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
	r.notify(ResultRetried)
}

// markTerminal fails an intention for operator attention and reports it.
func (r *Relay) markTerminal(ctx context.Context, id string, now time.Time, reason string) {
	if err := r.repository.MarkTerminal(ctx, id, now, reason); err != nil {
		r.logger.Error(ctx, "relay mark terminal failed",
			logger.Attribute{Key: "id", Value: id},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
	r.notify(ResultFailed)
}

// reportBacklog republishes the pending count to the observer (EML-23).
func (r *Relay) reportBacklog(ctx context.Context) {
	pending, err := r.repository.Backlog(ctx)
	if err != nil {
		r.logger.Error(ctx, "relay backlog failed", logger.Attribute{Key: attrError, Value: err.Error()})
		return
	}
	if r.observer != nil {
		r.observer.BacklogPending(pending)
	}
}

// notify reports one outcome, tolerating a relay built without an observer.
func (r *Relay) notify(result string) {
	if r.observer != nil {
		r.observer.PublishResult(result)
	}
}

// encode maps a stored intention onto the published message contract; the send
// attempt is 0 and the created time is the intention's, so a republish after a
// crash is byte-identical and the provider de-duplicates it (EML-07, EML-18).
func encode(intent outbox.Intent) email.Message {
	return email.Message{
		ID:       intent.ID,
		Type:     email.TypeOTP,
		To:       intent.To,
		Template: intent.Template,
		Data: email.Data{
			Code:             intent.Payload.Code,
			ExpiresInMinutes: intent.Payload.ExpiresInMinutes,
			Purpose:          email.Purpose(intent.Payload.Purpose),
		},
		CreatedAt: intent.CreatedAt,
		Attempt:   0,
	}
}

// backoff returns the delay for the given attempt count, capping at the last
// tier so the schedule never runs out before the attempt cap (EML-04).
func backoff(schedule []time.Duration, attempts int) time.Duration {
	if len(schedule) == 0 {
		return 0
	}
	index := attempts - 1
	if index < 0 {
		index = 0
	}
	if index >= len(schedule) {
		index = len(schedule) - 1
	}
	return schedule[index]
}
