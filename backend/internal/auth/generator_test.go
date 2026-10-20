package auth_test

import (
	"testing"

	"legnaris/backend/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenGenerator(t *testing.T) {
	generator := auth.NewRefreshTokenGenerator()

	t.Run("Happy path: generates distinct non-empty tokens", func(t *testing.T) {
		first, err := generator.Generate()
		require.NoError(t, err)
		second, err := generator.Generate()
		require.NoError(t, err)

		assert.NotEmpty(t, first)
		assert.NotEqual(t, first, second)
	})

	t.Run("Happy path: hash is deterministic, fixed-length and not the raw token", func(t *testing.T) {
		raw, err := generator.Generate()
		require.NoError(t, err)

		hash := auth.HashRefreshToken(raw)
		assert.Equal(t, hash, auth.HashRefreshToken(raw))
		assert.NotEqual(t, raw, hash)
		assert.Len(t, hash, 64)
	})
}
