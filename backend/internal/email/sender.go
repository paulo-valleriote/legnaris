package email

import (
	"context"
	"errors"
)

// Outbound is an already-rendered email handed to a Sender: the caller renders
// from a template and the adapter only transports the result, so the provider
// never reaches into the template registry (EML-08).
type Outbound struct {
	// IdempotencyKey is the intention identity; the provider de-duplicates a
	// republish by it (EML-09, EML-18).
	IdempotencyKey string
	To             string
	Subject        string
	Text           string
	HTML           string
}

// Sender is the one port through which every caller sends a rendered email;
// provider-specific behaviour stays behind an adapter selected by
// configuration (EML-08).
type Sender interface {
	Send(ctx context.Context, m Outbound) error
}

// ErrTransient and ErrPermanent classify a send failure at the port boundary:
// a caller retries an error wrapping ErrTransient and gives up on one wrapping
// ErrPermanent, deciding with errors.Is (EML-12).
//
// They are plain errors.New sentinels, never apperr: this domain is a non-HTTP
// layer with no status to map. The precedent is backend/shared/idgen.ErrInvalidID and
// backend/shared/db's own typed errors; the constitution's Principle I is scoped to
// HTTP boundaries and there is none here.
var (
	ErrTransient = errors.New("transient send failure")
	ErrPermanent = errors.New("permanent send failure")
)

// ErrQuotaExhausted marks a transient failure caused by the provider's sending
// quota (daily or monthly) being spent. It always wraps ErrTransient as well, so
// callers that only retry keep working. A circuit breaker must not count it as a
// provider outage: the quota resets on a known schedule (LGN-156).
var ErrQuotaExhausted = errors.New("provider quota exhausted")

// ErrRateLimited marks a transient failure caused by the provider's per-second
// request rate being exceeded. It always wraps ErrTransient. A circuit breaker
// must not count it as a provider outage: the rate window is seconds long
// (LGN-156).
var ErrRateLimited = errors.New("provider rate limited")
