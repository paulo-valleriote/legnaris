package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/shared/idgen"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"
)

const (
	attrError   = "error"
	attrID      = "message_id"
	attrPurpose = "purpose"
	attrTo      = "to"
	attrAttempt = "attempt"
	attrSize    = "size"
)

type Worker struct {
	source      DeliverySource
	republisher Republisher
	sender      email.Sender
	renderer    email.Renderer
	observer    Observer
	config      Config
	logger      logger.StructuredLogger
	breaker     *breaker.Breaker
}

func New(source DeliverySource, republisher Republisher, sender email.Sender, renderer email.Renderer, observer Observer, config Config, log logger.StructuredLogger) *Worker {
	config = config.withDefaults()
	return &Worker{
		source:      source,
		republisher: republisher,
		sender:      sender,
		renderer:    renderer,
		observer:    observer,
		config:      config,
		logger:      log,
		breaker:     breaker.New(config.BreakerThreshold, config.BreakerProbeInterval, config.Now),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	inflightCtx, cancelInflight := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelInflight()

	var inflight sync.WaitGroup
	slots := make(chan struct{}, w.config.Prefetch)

	for ctx.Err() == nil {
		stream, err := w.source.Consume(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			w.logger.Error(ctx, "consumer registration failed", logger.Attribute{Key: attrError, Value: err.Error()})
			if !sleep(ctx, w.config.RegisterRetryDelay) {
				break
			}
			continue
		}
		w.consume(ctx, inflightCtx, stream, slots, &inflight)
	}

	w.drain(cancelInflight, &inflight)
	return nil
}

func (w *Worker) consume(ctx, inflightCtx context.Context, stream <-chan Delivery, slots chan struct{}, inflight *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			return
		case slots <- struct{}{}:
		}

		probe, ok := w.admit(ctx)
		if !ok {
			<-slots
			return
		}

		select {
		case <-ctx.Done():
			w.breaker.Release(probe)
			<-slots
			return
		case delivery, ok := <-stream:
			if !ok {
				w.breaker.Release(probe)
				<-slots
				w.logger.Warn(ctx, "delivery stream ended; registering the consumer again")
				return
			}
			inflight.Add(1)
			go func() {
				defer inflight.Done()
				defer func() { <-slots }()
				defer w.breaker.Release(probe)
				w.handle(inflightCtx, delivery, probe)
			}()
		}
	}
}

func (w *Worker) admit(ctx context.Context) (uint64, bool) {
	for {
		decision := w.breaker.Admit()
		if decision.Proceed {
			return decision.Probe, true
		}
		if !sleep(ctx, decision.Wait) {
			return 0, false
		}
	}
}

func (w *Worker) drain(cancelInflight context.CancelFunc, inflight *sync.WaitGroup) {
	finished := make(chan struct{})
	go func() {
		inflight.Wait()
		close(finished)
	}()

	timer := time.NewTimer(w.config.DrainTimeout)
	defer timer.Stop()

	select {
	case <-finished:
	case <-timer.C:
		cancelInflight()
		w.logger.Warn(context.Background(), "stopped with messages still in flight; the broker will redeliver them")
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (w *Worker) handle(ctx context.Context, delivery Delivery, probe uint64) {
	msg, err := decode(delivery.Body)
	if err != nil {
		w.logger.Warn(ctx, "undecodable message dead-lettered",
			logger.Attribute{Key: attrSize, Value: fmt.Sprint(len(delivery.Body))},
			logger.Attribute{Key: attrError, Value: err.Error()})
		w.deadLetterBody(ctx, delivery)
		return
	}

	if w.expired(msg) {
		w.expire(ctx, delivery, msg)
		return
	}

	rendered, err := w.renderer.Render(msg.Data.Purpose, msg.Data)
	if err != nil {
		w.logger.Warn(ctx, "unrenderable message dead-lettered", append(describe(msg),
			logger.Attribute{Key: attrError, Value: classifyRender(err).Error()})...)
		w.deadLetter(ctx, delivery, msg)
		return
	}

	attempt := max(msg.Attempt, 0)
	if attempt >= w.config.MaxAttempts {
		w.logger.Warn(ctx, "attempt cap reached; message dead-lettered", describe(msg)...)
		w.deadLetter(ctx, delivery, msg)
		return
	}

	started := time.Now()
	err = w.sender.Send(ctx, email.Outbound{
		IdempotencyKey: msg.ID,
		To:             msg.To,
		Subject:        rendered.Subject,
		Text:           rendered.Text,
		HTML:           rendered.HTML,
	})
	w.observer.SendDuration(time.Since(started))
	w.reportBreaker(ctx, probe, classifyOutcome(err))

	switch {
	case err == nil:
		w.sent(ctx, delivery, msg)
	case errors.Is(err, email.ErrPermanent):
		w.logger.Warn(ctx, "permanent send failure; message dead-lettered", describe(msg)...)
		w.deadLetter(ctx, delivery, msg)
	default:
		w.retry(ctx, delivery, msg, attempt)
	}
}

func classifyOutcome(err error) breaker.Outcome {
	switch {
	case err == nil:
		return breaker.OutcomeSuccess
	case errors.Is(err, email.ErrQuotaExhausted), errors.Is(err, email.ErrRateLimited):
		return breaker.OutcomeNeutral
	case errors.Is(err, email.ErrTransient):
		return breaker.OutcomeOutage
	default:
		return breaker.OutcomeNeutral
	}
}

func (w *Worker) reportBreaker(ctx context.Context, probe uint64, outcome breaker.Outcome) {
	switch w.breaker.Record(probe, outcome) {
	case breaker.TransitionOpened:
		w.logger.Warn(ctx, "provider breaker opened; consumption paused")
		w.observer.BreakerOpened()
	case breaker.TransitionClosed:
		w.logger.Info(ctx, "provider breaker closed; consumption resumed")
		w.observer.BreakerClosed()
	case breaker.TransitionNone:
	}
}

func (w *Worker) sent(ctx context.Context, delivery Delivery, msg email.Message) {
	w.logger.Info(ctx, "email sent", describe(msg)...)
	w.observer.SendResult(ResultSent)
	w.acknowledge(ctx, delivery, msg)
}

func (w *Worker) expire(ctx context.Context, delivery Delivery, msg email.Message) {
	w.logger.Info(ctx, "email expired before delivery; dropped", describe(msg)...)
	w.observer.SendResult(ResultExpired)
	w.observer.MessageExpired()
	w.acknowledge(ctx, delivery, msg)
}

func (w *Worker) acknowledge(ctx context.Context, delivery Delivery, msg email.Message) {
	if err := delivery.Handle.Ack(); err != nil {
		w.logger.Error(ctx, "acknowledgement failed; the broker will redeliver", append(describe(msg),
			logger.Attribute{Key: attrError, Value: err.Error()})...)
	}
}

const maxLifetimeMinutes int64 = math.MaxInt64 / int64(time.Minute)

func (w *Worker) expired(msg email.Message) bool {
	if msg.Data.ExpiresInMinutes <= 0 {
		return true
	}
	if int64(msg.Data.ExpiresInMinutes) > maxLifetimeMinutes {
		return false
	}
	deadline := msg.CreatedAt.Add(time.Duration(msg.Data.ExpiresInMinutes) * time.Minute)
	return !w.config.Now().Before(deadline)
}

func decode(raw []byte) (email.Message, error) {
	var msg email.Message
	if err := sharedjson.DecodeBytes(raw, &msg); err != nil {
		return email.Message{}, fmt.Errorf("%w: %w", ErrUnusableMessage, err)
	}
	if err := idgen.Validate(msg.ID); err != nil {
		return email.Message{}, fmt.Errorf("%w: %w", ErrUnusableMessage, err)
	}
	if msg.Type != email.TypeOTP {
		return email.Message{}, fmt.Errorf("%w: type %q", ErrUnusableMessage, msg.Type)
	}
	if msg.Data.Purpose.CarriesCode() && strings.TrimSpace(msg.Data.Code) == "" {
		return email.Message{}, fmt.Errorf("%w: no code to deliver", ErrUnusableMessage)
	}
	return msg, nil
}

func classifyRender(err error) error {
	if errors.Is(err, email.ErrUnknownTemplate) {
		return fmt.Errorf("%w: %w", ErrUnusableMessage, email.ErrUnknownTemplate)
	}
	return fmt.Errorf("%w: render failed", ErrUnusableMessage)
}

func describe(msg email.Message) []logger.Attribute {
	return []logger.Attribute{
		{Key: attrID, Value: msg.ID},
		{Key: attrPurpose, Value: string(msg.Data.Purpose)},
		{Key: attrTo, Value: email.MaskRecipient(msg.To)},
		{Key: attrAttempt, Value: fmt.Sprint(msg.Attempt)},
	}
}
