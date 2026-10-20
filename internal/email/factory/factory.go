// Package factory selects the Sender adapter the configuration asks for, so the
// port's callers never branch on the provider themselves and adding a provider
// is one entry here plus one adapter (EML-08).
package factory

import (
	"errors"
	"fmt"
	"time"

	"legnaris/internal/email"
	"legnaris/internal/email/resend"
	"legnaris/shared/logger"
)

// Provider names the adapters EMAIL_PROVIDER selects.
const (
	// ProviderLog is the development sender: no network call (EML-10).
	ProviderLog = "log"
	// ProviderResend is the real provider adapter (EML-09).
	ProviderResend = "resend"
)

// ErrUnknownProvider reports an EMAIL_PROVIDER value that selects no adapter;
// the effect is a permanent build error, never a send-time surprise (EML-19).
//
// It is a plain errors.New sentinel, never apperr: this domain is a non-HTTP
// layer with no status to map (see email.ErrTransient in sender.go).
var ErrUnknownProvider = errors.New("unknown email provider")

// Config is the sender selection the process builds from the shared
// configuration surface (EML-08, EML-14).
type Config struct {
	Provider string
	APIKey   string
	From     string
	Timeout  time.Duration
	Logger   logger.StructuredLogger
}

// New builds the adapter the provider selects: the development sender for
// ProviderLog and the Resend adapter for ProviderResend. An unknown provider is
// ErrUnknownProvider, and the Resend adapter refuses an empty key or From, so a
// misconfiguration fails when the sender is built rather than on the first send
// (EML-08, EML-12, EML-19).
func New(cfg Config) (email.Sender, error) {
	switch cfg.Provider {
	case ProviderLog:
		return email.NewLogSender(cfg.Logger), nil
	case ProviderResend:
		sender, err := resend.New(cfg.APIKey, cfg.From, cfg.Timeout)
		if err != nil {
			return nil, err
		}
		return sender, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, cfg.Provider)
	}
}
