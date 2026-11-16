package auth

import (
	"context"
	"errors"
	"time"
)

type CodeMailer interface {
	Record(ctx context.Context, id, to string, purpose Purpose, code string, expiresInMinutes int, now time.Time) error
	Requeue(ctx context.Context, id string, now time.Time) (string, error)
	SentSince(ctx context.Context, since time.Time) (int, error)
	ResendsSince(ctx context.Context, since time.Time) (int, error)
}

var ErrSendBudgetExhausted = errors.New("daily email send budget exhausted")
