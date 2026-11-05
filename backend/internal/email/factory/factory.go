// Package factory selects the Sender adapter the configuration asks for, so the
// port's callers never branch on the provider themselves and adding a provider
// is one entry here plus one adapter (EML-08).
package factory

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/resend"
	"legnaris/backend/shared/logger"
)

// Provider names the adapters EMAIL_PROVIDER selects.
const (
	// ProviderLog is the development sender: no network call (EML-10).
	ProviderLog = "log"
	// ProviderDev is the local-development sender that logs the message body,
	// code included, in clear. It is refused in production (D10).
	ProviderDev = "dev"
	// ProviderResend is the real provider adapter (EML-09).
	ProviderResend = "resend"
)

// ErrUnknownProvider reports an EMAIL_PROVIDER value that selects no adapter;
// the effect is a permanent build error, never a send-time surprise (EML-19).
//
// It is a plain errors.New sentinel, never apperr: this domain is a non-HTTP
// layer with no status to map (see email.ErrTransient in sender.go).
var ErrUnknownProvider = errors.New("unknown email provider")

// ErrDevProviderInProduction reports the development sender requested in a
// production deployment. It is a boot error: the worker stops before it dials
// the broker, so a code can never be written to logs in production (D10).
var ErrDevProviderInProduction = errors.New("the dev email provider is refused in production")

// appEnvProduction is the APP_ENV value that forbids the development sender.
const appEnvProduction = "production"

// Config is the sender selection the process builds from the shared
// configuration surface (EML-08, EML-14).
type Config struct {
	Provider string
	AppEnv   string
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
	case ProviderDev:
		if isProduction(cfg.AppEnv) {
			return nil, ErrDevProviderInProduction
		}
		return email.NewDevSender(cfg.Logger), nil
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

// isProduction matches APP_ENV case-insensitively and ignoring surrounding
// spaces, so "Production" cannot slip past the guard.
func isProduction(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), appEnvProduction)
}
