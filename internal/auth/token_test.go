package auth_test

import (
	"testing"
	"time"

	"legnaris/internal/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenManager(t *testing.T) {
	const secret = "test-secret"

	manager, err := auth.NewTokenManager(secret, 15*time.Minute)
	require.NoError(t, err)

	t.Run("Happy path: issue and parse round-trips the user id", func(t *testing.T) {
		token, err := manager.Issue("user-1")
		require.NoError(t, err)

		assert.NotEmpty(t, token.Value)
		assert.Equal(t, int((15 * time.Minute).Seconds()), token.ExpiresIn)

		userID, err := manager.Parse(token.Value)
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("Error: empty secret is rejected at construction", func(t *testing.T) {
		_, err := auth.NewTokenManager("", time.Minute)
		require.ErrorIs(t, err, auth.ErrEmptyJWTSecret)
	})

	t.Run("Error: expired token is rejected", func(t *testing.T) {
		expired, err := auth.NewTokenManager(secret, -time.Minute)
		require.NoError(t, err)

		token, err := expired.Issue("user-1")
		require.NoError(t, err)

		_, err = manager.Parse(token.Value)
		require.ErrorIs(t, err, auth.ErrExpiredToken)
	})

	t.Run("Error: token signed with another secret is rejected", func(t *testing.T) {
		other, err := auth.NewTokenManager("other-secret", time.Minute)
		require.NoError(t, err)

		token, err := other.Issue("user-1")
		require.NoError(t, err)

		_, err = manager.Parse(token.Value)
		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	t.Run("Error: unsigned alg=none token is rejected", func(t *testing.T) {
		unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "attacker"})
		raw, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		_, err = manager.Parse(raw)
		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	t.Run("Edge: garbage token is rejected", func(t *testing.T) {
		_, err := manager.Parse("not-a-jwt")
		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})
}
