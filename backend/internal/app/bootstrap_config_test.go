package app

import (
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
)

// TestAuthServiceConfig pins the wiring of the resolved environment onto the
// auth service's configuration: the rollout gate travels with the service, so
// the decision is made in one place (TFA-01, TFA-02, TFA-07).
func TestAuthServiceConfig(t *testing.T) {
	now := auth.Clock(func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) })

	t.Run("Edge: the gate is disabled by default", func(t *testing.T) {
		cfg := authServiceConfig(env.Variables{AuthRefreshTTL: time.Hour}, now)

		assert.False(t, cfg.RequireEmailVerification)
		assert.Equal(t, time.Hour, cfg.RefreshTTL)
	})

	t.Run("Happy path: the gate reaches the service configuration", func(t *testing.T) {
		cfg := authServiceConfig(env.Variables{
			AuthRefreshTTL:           2 * time.Hour,
			RequireEmailVerification: true,
		}, now)

		assert.True(t, cfg.RequireEmailVerification)
		assert.Equal(t, 2*time.Hour, cfg.RefreshTTL)
		assert.NotNil(t, cfg.Now)
	})
}
