// Package worker owns the delivery policy of the email pipeline: it decodes a
// queued message, decides from the message's own deadline whether the code is
// still worth sending, renders and sends it, retries transient failures through
// the broker's delay tiers and parks the ones that can never succeed. It needs
// no database: the code's lifetime travels on the message (WKR-01, WKR-03).
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

// Worker consumes deliveries and applies the delivery policy to each one.
type Worker struct {
	source      DeliverySource
	republisher Republisher
	sender      email.Sender
	renderer    email.Renderer
	observer    Observer
	config      Config
	logger      logger.StructuredLogger
	breaker     *breaker
}

// New builds a worker over the delivery source, the republisher, the sending
// port and the renderer; every zero value in config is filled from its
// documented default (WKR-14, WKR-16, LGN-149).
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
		breaker:     newBreaker(config.BreakerThreshold, config.BreakerProbeInterval, config.Now),
	}
}

// Run consumes until ctx is cancelled. When the delivery stream ends it
// registers the consumer again instead of failing, so a broker outage is
// logged rather than fatal (WKR-17). On stop it takes no new message, waits for
// the messages in flight for DrainTimeout and leaves the unfinished ones
// unacknowledged for the broker to redeliver (WKR-18).
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

// consume handles the stream until it ends or ctx is cancelled. A slot is taken
// before each delivery is read, so no more than Prefetch messages are held at
// once (WKR-16). While the provider breaker is open the stream is not read at
// all, so the broker keeps the backlog (LGN-149).
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
			w.breaker.release(probe)
			<-slots
			return
		case delivery, ok := <-stream:
			if !ok {
				w.breaker.release(probe)
				<-slots
				w.logger.Warn(ctx, "delivery stream ended; registering the consumer again")
				return
			}
			inflight.Add(1)
			go func() {
				defer inflight.Done()
				defer func() { <-slots }()
				defer w.breaker.release(probe)
				w.handle(inflightCtx, delivery, probe)
			}()
		}
	}
}

// admit blocks until the breaker lets one delivery through. It returns the probe
// token when the admission is a probe, and false when ctx is cancelled first.
func (w *Worker) admit(ctx context.Context) (uint64, bool) {
	for {
		decision := w.breaker.admit()
		if decision.proceed {
			return decision.probe, true
		}
		if !sleep(ctx, decision.wait) {
			return 0, false
		}
	}
}

// drain waits for the in-flight messages for DrainTimeout, then cancels what is
// left: an unfinished message stays unacknowledged (WKR-18).
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

// handle takes one delivery to exactly one outcome: sent and acknowledged,
// dead-lettered and acknowledged, expired and acknowledged without a send, or
// left unacknowledged for redelivery. The acknowledgement is always last
// (WKR-02).
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

// classifyOutcome maps one send error to its breaker outcome. A transient error
// that is not a quota or rate limit is a provider outage (5xx, timeout).
func classifyOutcome(err error) breakerOutcome {
	switch {
	case err == nil:
		return outcomeSuccess
	case errors.Is(err, email.ErrQuotaExhausted), errors.Is(err, email.ErrRateLimited):
		return outcomeNeutral
	case errors.Is(err, email.ErrTransient):
		return outcomeOutage
	default:
		return outcomeNeutral
	}
}

// reportBreaker feeds one send outcome to the breaker and reports a transition.
func (w *Worker) reportBreaker(ctx context.Context, probe uint64, outcome breakerOutcome) {
	switch w.breaker.record(probe, outcome) {
	case transitionOpened:
		w.logger.Warn(ctx, "provider breaker opened; consumption paused")
		w.observer.BreakerOpened()
	case transitionClosed:
		w.logger.Info(ctx, "provider breaker closed; consumption resumed")
		w.observer.BreakerClosed()
	case transitionNone:
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

// maxLifetimeMinutes is the largest lifetime whose duration still fits in a
// time.Duration; anything above it never expires within a representable time.
const maxLifetimeMinutes int64 = math.MaxInt64 / int64(time.Minute)

// expired reports whether the message's own deadline has passed. A zero or
// negative lifetime has always passed (WKR-07).
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

// decode turns a body into a validated message. Every failure wraps
// ErrUnusableMessage: nothing about the message can change on a retry (WKR-04).
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

// describe identifies a message in a log line by its identity and purpose, with
// the recipient masked. The code is never an attribute (WKR-06, WKR-22).
func describe(msg email.Message) []logger.Attribute {
	return []logger.Attribute{
		{Key: attrID, Value: msg.ID},
		{Key: attrPurpose, Value: string(msg.Data.Purpose)},
		{Key: attrTo, Value: email.MaskRecipient(msg.To)},
		{Key: attrAttempt, Value: fmt.Sprint(msg.Attempt)},
	}
}
