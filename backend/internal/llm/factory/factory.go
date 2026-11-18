package factory

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/logger"
)

const ProviderOpenRouter = "openrouter"

var ErrUnknownProvider = errors.New("unknown llm provider")

type Config struct {
	Provider        string
	BaseURL         string
	APIKey          string
	Model           string
	ReasoningEffort string
	Timeout         time.Duration
	Logger          logger.StructuredLogger
}

func New(cfg Config) (llm.Client, error) {
	switch cfg.Provider {
	case ProviderOpenRouter:
		client, err := openrouter.New(openrouter.Config{
			BaseURL:         cfg.BaseURL,
			APIKey:          cfg.APIKey,
			Model:           cfg.Model,
			ReasoningEffort: cfg.ReasoningEffort,
			Timeout:         cfg.Timeout,
			Logger:          cfg.Logger,
		})
		if err != nil {
			return nil, err
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, cfg.Provider)
	}
}
