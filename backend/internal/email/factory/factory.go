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

const (
	ProviderLog    = "log"
	ProviderDev    = "dev"
	ProviderResend = "resend"
)

var ErrUnknownProvider = errors.New("unknown email provider")

var ErrDevProviderInProduction = errors.New("the dev email provider is refused in production")

const appEnvProduction = "production"

type Config struct {
	Provider string
	AppEnv   string
	APIKey   string
	From     string
	Timeout  time.Duration
	Logger   logger.StructuredLogger
}

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

func isProduction(appEnv string) bool {
	return strings.EqualFold(strings.TrimSpace(appEnv), appEnvProduction)
}
