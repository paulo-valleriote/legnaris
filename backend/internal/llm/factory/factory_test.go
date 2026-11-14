package factory_test

import (
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/llm/factory"
	"legnaris/backend/internal/llm/openrouter"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() factory.Config {
	return factory.Config{
		Provider: factory.ProviderOpenRouter,
		BaseURL:  "https://openrouter.example/api/v1",
		APIKey:   "test-key",
		Model:    "deepseek/deepseek-v4.1-flash",
		Timeout:  time.Second,
		Logger:   logger.NewLogger(slog.LevelError),
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *factory.Config)
		wantErr error
	}{
		{name: "Happy path: openrouter selects the OpenRouter adapter", mutate: func(*factory.Config) {}},
		{name: "Edge: unknown provider fails at construction", mutate: func(c *factory.Config) { c.Provider = "opencode" }, wantErr: factory.ErrUnknownProvider},
		{name: "Edge: empty provider fails at construction", mutate: func(c *factory.Config) { c.Provider = "" }, wantErr: factory.ErrUnknownProvider},
		{name: "Edge: provider name is case sensitive", mutate: func(c *factory.Config) { c.Provider = "OpenRouter" }, wantErr: factory.ErrUnknownProvider},
		{name: "Edge: missing API key propagates from the adapter", mutate: func(c *factory.Config) { c.APIKey = "" }, wantErr: openrouter.ErrMissingAPIKey},
		{name: "Edge: :free model propagates from the adapter", mutate: func(c *factory.Config) { c.Model = "x/y:free" }, wantErr: openrouter.ErrFreeModel},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			client, err := factory.New(cfg)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, client)
				return
			}
			require.NoError(t, err)
			assert.IsType(t, &openrouter.Client{}, client)
		})
	}
}
