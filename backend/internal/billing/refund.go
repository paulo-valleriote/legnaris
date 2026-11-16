package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const (
	RefundQueueName = "billing_refunds"
	RefundInterval  = 30 * time.Second
	RefundWindow    = 7 * 24 * time.Hour
)

const (
	ReasonWindowExpired  = "window_expired"
	ReasonAlreadyUsed    = "already_used"
	ReasonNotSubscribed  = "not_subscribed"
	MsgRefundNotEligible = "the refund is not available"
	MsgRefundUnavailable = "refund is temporarily unavailable"
)

var (
	ErrRefundAlreadyUsed  = errors.New("refund right already used")
	ErrRefundStateChanged = errors.New("refund state changed concurrently")
)

type RefundState string

const (
	RefundRequested RefundState = "requested"
	RefundRefunded  RefundState = "refunded"
	RefundCanceled  RefundState = "canceled"
	RefundFailed    RefundState = "failed"
)

func RefundOpen(sub Subscription, now time.Time) bool {
	return sub.Status == StatusActive && sub.RefundState == "" && sub.RefundUsedAt == nil &&
		sub.FirstPaidAt != nil && now.Before(sub.FirstPaidAt.Add(RefundWindow))
}

func refundDenial(sub Subscription, now time.Time) string {
	switch {
	case sub.Status != StatusActive || sub.FirstPaidAt == nil:
		return ReasonNotSubscribed
	case sub.RefundState != "" || sub.RefundUsedAt != nil:
		return ReasonAlreadyUsed
	case !now.Before(sub.FirstPaidAt.Add(RefundWindow)):
		return ReasonWindowExpired
	}
	return ""
}

type RefundLedger interface {
	ClaimRefunds(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Subscription, error)
	RescheduleRefund(ctx context.Context, id string, nextAt time.Time, reason string) error
	SetRefundState(ctx context.Context, id string, from, to RefundState) error
	FailRefund(ctx context.Context, id string, reason string) error
	OldestRefundRequested(ctx context.Context) (time.Time, bool, error)
}

func (s *Service) Refund(ctx context.Context, userID string) error {
	if err := idgen.Validate(userID); err != nil {
		return apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidUser, err)
	}
	live, found, err := s.store.FindLive(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgRefundUnavailable, err)
	}
	now := s.cfg.Now()
	if !found {
		return refundDenied(ReasonNotSubscribed)
	}
	if reason := refundDenial(live, now); reason != "" {
		return refundDenied(reason)
	}

	ok, err := s.store.RequestRefund(ctx, live.ID, now)
	switch {
	case errors.Is(err, ErrRefundAlreadyUsed):
		return refundDenied(ReasonAlreadyUsed)
	case err != nil:
		return apperr.NewError(apperr.CodeInternal, MsgRefundUnavailable, err)
	case !ok:
		return refundDenied(ReasonAlreadyUsed)
	}
	return nil
}

func refundDenied(reason string) error {
	return apperr.NewError(apperr.CodeRefundNotEligible, MsgRefundNotEligible, nil).
		WithDetails(map[string]any{"reason": reason})
}

type RefundDeps struct {
	Provider Provider
	Ledger   RefundLedger
}

type refundStore struct {
	ledger RefundLedger
}

func (s refundStore) Claim(ctx context.Context, _ string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[Subscription], error) {
	subs, err := s.ledger.ClaimRefunds(ctx, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[Subscription], 0, len(subs))
	for _, sub := range subs {
		jobs = append(jobs, sweeper.Job[Subscription]{ID: sub.ID, Attempts: sub.RefundAttempts, Payload: sub})
	}
	return jobs, nil
}

func (s refundStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, reason string) error {
	return s.ledger.RescheduleRefund(ctx, id, nextAt, reason)
}

func (s refundStore) MarkDead(ctx context.Context, id string, _ time.Time, reason string) error {
	return s.ledger.FailRefund(ctx, id, reason)
}

func (s refundStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.ledger.OldestRefundRequested(ctx)
}

func NewRefundQueue(deps RefundDeps, attemptCap, batch int) (sweeper.Queue, error) {
	return sweeper.NewQueue(sweeper.QueueSpec[Subscription]{
		Name:       RefundQueueName,
		Interval:   RefundInterval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      refundStore{ledger: deps.Ledger},
		Handle:     refundHandler{deps: deps}.Handle,
	})
}

type refundHandler struct {
	deps RefundDeps
}

func (h refundHandler) Handle(ctx context.Context, job sweeper.Job[Subscription], _ time.Time) error {
	sub := job.Payload
	if sub.RefundState == RefundRequested {
		if err := h.deps.Provider.RefundLatest(ctx, RefundRequest{
			ProviderSubscriptionID: sub.ProviderSubID,
			IdempotencyKey:         "refund:" + sub.ID,
		}); err != nil {
			return providerFailure(err)
		}
		if err := h.deps.Ledger.SetRefundState(ctx, sub.ID, RefundRequested, RefundRefunded); err != nil {
			return fmt.Errorf("mark refunded: %w", err)
		}
		sub.RefundState = RefundRefunded
	}
	if sub.RefundState == RefundRefunded {
		if err := h.deps.Provider.CancelNow(ctx, CancelRequest{
			ProviderSubscriptionID: sub.ProviderSubID,
			IdempotencyKey:         "cancel:" + sub.ID,
		}); err != nil {
			return providerFailure(err)
		}
		if err := h.deps.Ledger.SetRefundState(ctx, sub.ID, RefundRefunded, RefundCanceled); err != nil {
			return fmt.Errorf("mark canceled: %w", err)
		}
	}
	return nil
}

func providerFailure(err error) error {
	if errors.Is(err, ErrProviderRejected) {
		return fmt.Errorf("%w: %w", sweeper.ErrPermanent, err)
	}
	return fmt.Errorf("provider refund step: %w", err)
}
