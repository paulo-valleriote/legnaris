package billing

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const (
	MsgInvalidSignature = "invalid webhook signature"
	MsgInvalidWebhook   = "invalid webhook payload"
	MsgWebhookBusy      = "webhook is temporarily unavailable"
)

type Inbox struct {
	provider Provider
	ledger   Ledger
	now      func() time.Time
}

func NewInbox(provider Provider, ledger Ledger, now func() time.Time) *Inbox {
	if now == nil {
		now = time.Now
	}
	return &Inbox{provider: provider, ledger: ledger, now: now}
}

func (in *Inbox) Receive(ctx context.Context, payload []byte, signature string) error {
	n, err := in.provider.ParseWebhook(payload, signature)
	if err != nil {
		return mapWebhookError(err)
	}

	now := in.now()
	ev := Event{
		Provider:               ProviderStripe,
		ProviderEventID:        n.ID,
		Kind:                   n.Kind,
		ProviderSubscriptionID: n.ProviderSubscriptionID,
		ProviderCustomerID:     n.ProviderCustomerID,
		Status:                 EventReceived,
		Attempts:               0,
		NextAttemptAt:          now,
		ReceivedAt:             now,
		Payload:                payload,
		ExpiresAt:              now.Add(inboxRetentionDuration),
	}
	ev.UserID, err = in.owner(ctx, n)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgWebhookBusy, err)
	}
	if n.Kind == KindIgnored || ev.UserID == "" {
		ev.Kind = KindIgnored
		ev.Status = EventProcessed
		ev.Reason = ReasonIgnored
		ev.ProcessedAt = &now
	}
	ev.ID, err = idgen.New()
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgWebhookBusy, err)
	}

	if err := in.ledger.InsertEvent(ctx, ev); err != nil {
		if errors.Is(err, ErrDuplicateEvent) {
			return nil
		}
		return apperr.NewError(apperr.CodeInternal, MsgWebhookBusy, err)
	}
	return nil
}

func (in *Inbox) owner(ctx context.Context, n Notification) (string, error) {
	if n.UserID != "" {
		return n.UserID, nil
	}
	userID, found, err := in.ledger.ResolveUser(ctx, n.ProviderSubscriptionID, n.ProviderCustomerID)
	if err != nil || !found {
		return "", err
	}
	return userID, nil
}

func mapWebhookError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidSignature):
		return apperr.NewError(apperr.CodeInvalidSignature, MsgInvalidSignature, err)
	case errors.Is(err, ErrProviderUnavailable):
		return apperr.NewError(apperr.CodeBillingUnavailable, MsgBillingUnavailable, err)
	case errors.Is(err, ErrProviderRejected):
		return apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidWebhook, err)
	}
	return apperr.NewError(apperr.CodeInternal, MsgWebhookBusy, err)
}
