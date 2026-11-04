package factory_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/internal/email/factory"
	"legnaris/backend/internal/email/resend"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() factory.Config {
	return factory.Config{
		Provider: factory.ProviderLog,
		Timeout:  time.Second,
		Logger:   logger.NewLogger(slog.LevelError),
	}
}

// TestNew_SelectsTheAdapter proves the configuration selects the adapter the
// caller expects (EML-08).
func TestNew_SelectsTheAdapter(t *testing.T) {
	t.Run("Happy path: the log provider builds the development sender", func(t *testing.T) {
		sender, err := factory.New(testConfig())
		require.NoError(t, err)
		assert.IsType(t, &email.LogSender{}, sender)

		require.NoError(t, sender.Send(context.Background(), email.Outbound{
			IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
			To:             "player@example.com",
			Subject:        "Your Legnaris login code",
		}))
	})

	t.Run("Happy path: the dev provider builds the development sender outside production", func(t *testing.T) {
		for _, appEnv := range []string{"", "development", "staging"} {
			cfg := testConfig()
			cfg.Provider = factory.ProviderDev
			cfg.AppEnv = appEnv

			sender, err := factory.New(cfg)
			require.NoError(t, err, "APP_ENV=%q", appEnv)
			assert.IsType(t, &email.DevSender{}, sender)
		}
	})

	t.Run("Happy path: the resend provider builds the real adapter", func(t *testing.T) {
		cfg := testConfig()
		cfg.Provider = factory.ProviderResend
		cfg.APIKey = "re_test"
		cfg.From = "no-reply@example.com"

		sender, err := factory.New(cfg)
		require.NoError(t, err)
		assert.IsType(t, &resend.Sender{}, sender)
	})
}

// TestNew_RejectsUnknownProvider proves a provider string that selects no
// adapter is a typed build error, never a silent fallback (EML-08, EML-19).
func TestNew_RejectsUnknownProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "Error: an empty provider", provider: ""},
		{name: "Error: an unrelated provider", provider: "smtp"},
		{name: "Error: a misspelled provider", provider: "Resend"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Provider = tc.provider

			sender, err := factory.New(cfg)

			require.Error(t, err)
			assert.Nil(t, sender, "an unknown provider must build no sender")
			assert.ErrorIs(t, err, factory.ErrUnknownProvider)
		})
	}
}

// TestNew_ResendRequiresConfiguration proves the Resend branch surfaces the
// adapter's own build error for an empty key or From (EML-12).
func TestNew_ResendRequiresConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		from   string
	}{
		{name: "Error: empty API key", apiKey: "", from: "no-reply@example.com"},
		{name: "Error: empty From", apiKey: "re_test", from: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Provider = factory.ProviderResend
			cfg.APIKey = tc.apiKey
			cfg.From = tc.from

			sender, err := factory.New(cfg)

			require.Error(t, err)
			assert.Nil(t, sender)
			assert.ErrorIs(t, err, resend.ErrMissingConfiguration)
		})
	}
}

// TestNew_RefusesDevProviderInProduction proves the development sender, which
// writes codes in clear, never builds when APP_ENV names production, whatever
// the casing or padding (D10).
func TestNew_RefusesDevProviderInProduction(t *testing.T) {
	tests := []struct {
		name   string
		appEnv string
	}{
		{name: "Error: APP_ENV=production", appEnv: "production"},
		{name: "Edge: APP_ENV in upper case", appEnv: "PRODUCTION"},
		{name: "Edge: APP_ENV with surrounding spaces", appEnv: "  production "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Provider = factory.ProviderDev
			cfg.AppEnv = tc.appEnv

			sender, err := factory.New(cfg)

			require.Error(t, err)
			assert.Nil(t, sender, "the development sender must not be built in production")
			assert.ErrorIs(t, err, factory.ErrDevProviderInProduction)
		})
	}
}
