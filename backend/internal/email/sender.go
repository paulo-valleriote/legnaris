package email

import (
	"context"
	"errors"
)

type Outbound struct {
	IdempotencyKey string
	To             string
	Subject        string
	Text           string
	HTML           string
}

type Sender interface {
	Send(ctx context.Context, m Outbound) error
}

var (
	ErrTransient = errors.New("transient send failure")
	ErrPermanent = errors.New("permanent send failure")
)

var ErrQuotaExhausted = errors.New("provider quota exhausted")

var ErrRateLimited = errors.New("provider rate limited")
