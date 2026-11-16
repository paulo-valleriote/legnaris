package relay

import (
	"context"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

const attrError = "error"

type Relay struct {
	repository outbox.Repository
	publisher  Publisher
	observer   Observer
	config     Config
	logger     logger.StructuredLogger
}

func New(repository outbox.Repository, publisher Publisher, observer Observer, config Config, log logger.StructuredLogger) *Relay {
	return &Relay{
		repository: repository,
		publisher:  publisher,
		observer:   observer,
		config:     config,
		logger:     log,
	}
}

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

func (r *Relay) RunOnce(ctx context.Context) {
	r.pass(ctx, r.config.withDefaults())
}

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

func (r *Relay) markTerminal(ctx context.Context, id string, now time.Time, reason string) {
	if err := r.repository.MarkTerminal(ctx, id, now, reason); err != nil {
		r.logger.Error(ctx, "relay mark terminal failed",
			logger.Attribute{Key: "id", Value: id},
			logger.Attribute{Key: attrError, Value: err.Error()})
	}
	r.notify(ResultFailed)
}

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

func (r *Relay) notify(result string) {
	if r.observer != nil {
		r.observer.PublishResult(result)
	}
}

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
