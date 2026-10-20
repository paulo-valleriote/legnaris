package app

import (
	"context"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/logger"
)

// EmailIntent adapts the challenge domain's CodeMailer port onto the email
// outbox delivered by the email feature, so backend/internal/auth never imports the
// email domain (R8, OTP-12). The request-serving process only writes a durable
// intention: it never contacts the provider or the broker.
type EmailIntent struct {
	repository outbox.Repository
	logger     logger.StructuredLogger
}

// The adapter must satisfy the consumer-declared port.
var _ auth.CodeMailer = (*EmailIntent)(nil)

// NewEmailIntent builds the code-mailer adapter over the outbox store.
func NewEmailIntent(repository outbox.Repository, log logger.StructuredLogger) *EmailIntent {
	return &EmailIntent{repository: repository, logger: log}
}

// Record writes one pending intention carrying the code, its lifetime and the
// purpose. The recipient is masked in the log line and the code is never logged
// (OTP-12, OTP-15).
func (e *EmailIntent) Record(ctx context.Context, id, to string, purpose auth.Purpose, code string, expiresInMinutes int, now time.Time) error {
	intent, err := outbox.NewIntent(id, to, string(purpose), outbox.Payload{
		Code:             code,
		ExpiresInMinutes: expiresInMinutes,
		Purpose:          string(purpose),
	}, now)
	if err != nil {
		return err
	}

	if err := e.repository.Create(ctx, intent); err != nil {
		return err
	}

	e.logger.Info(ctx, "email intent recorded",
		logger.Attribute{Key: "intent_id", Value: id},
		logger.Attribute{Key: "template", Value: string(purpose)},
		logger.Attribute{Key: "to", Value: email.MaskRecipient(to)},
	)
	return nil
}

// Requeue records a resend: it clones the identified intention into a new row
// with a fresh identity and returns that identity, so the code is re-sent
// without being rotated (OTP-10).
func (e *EmailIntent) Requeue(ctx context.Context, id string, now time.Time) (string, error) {
	cloned, err := e.repository.Clone(ctx, id, now)
	if err != nil {
		return "", err
	}
	return cloned.ID, nil
}

// SentSince forwards the daily-cap window to the store's count (OTP-13).
func (e *EmailIntent) SentSince(ctx context.Context, since time.Time) (int, error) {
	return e.repository.CountSince(ctx, since)
}
