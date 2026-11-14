// Package factory selects the llm.Client adapter the configuration asks for, so
// callers never branch on the provider themselves. Adding a provider is one case
// here plus one adapter.
package factory

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/logger"
)

// ProviderOpenRouter names the adapter LLM_PROVIDER selects. It is the only
// gateway (ADR 0008).
const ProviderOpenRouter = "openrouter"

// ErrUnknownProvider reports an LLM_PROVIDER value that selects no adapter. It is
// a plain sentinel: the failure is a boot error and has no HTTP status.
var ErrUnknownProvider = errors.New("unknown llm provider")

// Config is the client selection the process builds from the shared configuration.
type Config struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
	Logger   logger.StructuredLogger
}

// New builds the adapter the provider selects. An unknown provider fails with
// ErrUnknownProvider, and the adapter rejects a missing key, a :free model or an
// invalid endpoint, so a misconfiguration fails at construction (ADR 0008).
func New(cfg Config) (llm.Client, error) {
	switch cfg.Provider {
	case ProviderOpenRouter:
		client, err := openrouter.New(openrouter.Config{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
			Logger:  cfg.Logger,
		})
		if err != nil {
			return nil, err
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, cfg.Provider)
	}
}
