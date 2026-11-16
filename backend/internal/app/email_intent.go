package app

import (
	"context"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/outbox"
	"legnaris/backend/shared/logger"
)

type EmailIntent struct {
	repository outbox.Repository
	logger     logger.StructuredLogger
}

var _ auth.CodeMailer = (*EmailIntent)(nil)

func NewEmailIntent(repository outbox.Repository, log logger.StructuredLogger) *EmailIntent {
	return &EmailIntent{repository: repository, logger: log}
}

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

func (e *EmailIntent) Requeue(ctx context.Context, id string, now time.Time) (string, error) {
	cloned, err := e.repository.Clone(ctx, id, now)
	if err != nil {
		return "", err
	}
	return cloned.ID, nil
}

func (e *EmailIntent) ResendsSince(ctx context.Context, since time.Time) (int, error) {
	return e.repository.CountKindSince(ctx, outbox.KindResend, since)
}

func (e *EmailIntent) SentSince(ctx context.Context, since time.Time) (int, error) {
	return e.repository.CountSince(ctx, since)
}
