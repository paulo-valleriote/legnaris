package auth

import (
	"context"
	"errors"
	"time"
)

// CodeMailer records the durable intention that a code must be emailed. It is
// declared by the consumer (internal/auth) and implemented over the email
// domain's outbox at the composition root, so the auth domain never imports the
// email domain (R8, OTP-12).
type CodeMailer interface {
	// Record writes a new pending intention with the supplied identity,
	// recipient, purpose, code, lifetime and instant. The recipient is the
	// account's canonical address.
	Record(ctx context.Context, id, to string, purpose Purpose, code string, expiresInMinutes int, now time.Time) error
	// Requeue writes a new intention carrying the same recipient, template and
	// payload as the intention identified by id, with a fresh identity, and
	// returns that new identity (OTP-10).
	Requeue(ctx context.Context, id string, now time.Time) (string, error)
	// SentSince counts the intentions recorded at or after the given instant,
	// the daily cap's only input (OTP-13).
	SentSince(ctx context.Context, since time.Time) (int, error)
}

// ErrSendBudgetExhausted reports that the daily send cap has been reached, so no
// new intention may be written and the request must answer unavailable. It is a
// plain sentinel, never apperr: this is not an HTTP layer, exactly like the
// email domain's ErrTransient/ErrPermanent.
var ErrSendBudgetExhausted = errors.New("daily email send budget exhausted")
