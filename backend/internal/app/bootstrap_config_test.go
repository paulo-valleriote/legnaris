package app

import (
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/shared/env"

	"github.com/stretchr/testify/assert"
)

// TestAuthServiceConfig pins the wiring of the resolved environment onto the
// auth service's configuration.
func TestAuthServiceConfig(t *testing.T) {
	now := auth.Clock(func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) })

	t.Run("Happy path: the refresh TTL and clock reach the service configuration", func(t *testing.T) {
		cfg := authServiceConfig(env.Variables{AuthRefreshTTL: 2 * time.Hour}, now)

		assert.Equal(t, 2*time.Hour, cfg.RefreshTTL)
		assert.NotNil(t, cfg.Now)
	})
}
