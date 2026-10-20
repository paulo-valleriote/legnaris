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

	manager, err := auth.NewTokenManager(secret, 15*time.Minute, nil)
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
		_, err := auth.NewTokenManager("", time.Minute, nil)
		require.ErrorIs(t, err, auth.ErrEmptyJWTSecret)
	})

	t.Run("Error: expired token is rejected", func(t *testing.T) {
		expired, err := auth.NewTokenManager(secret, -time.Minute, nil)
		require.NoError(t, err)

		token, err := expired.Issue("user-1")
		require.NoError(t, err)

		_, err = manager.Parse(token.Value)
		require.ErrorIs(t, err, auth.ErrExpiredToken)
	})

	t.Run("Error: token signed with another secret is rejected", func(t *testing.T) {
		other, err := auth.NewTokenManager("other-secret", time.Minute, nil)
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

// TestTokenManager_Clock proves issuance and validation read the injected clock
// so expiry boundaries are crossed without sleeping (PRQ-16, SC-007).
func TestTokenManager_Clock(t *testing.T) {
	const secret = "test-secret"
	const ttl = 15 * time.Minute

	frozen := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	now := frozen

	manager, err := auth.NewTokenManager(secret, ttl, func() time.Time { return now })
	require.NoError(t, err)

	t.Run("Happy path: issuance stamps iat and exp from the injected clock", func(t *testing.T) {
		token, err := manager.Issue("user-1")
		require.NoError(t, err)

		claims := &jwt.RegisteredClaims{}
		_, _, err = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(token.Value, claims)
		require.NoError(t, err)
		require.NotNil(t, claims.IssuedAt)
		require.NotNil(t, claims.ExpiresAt)
		assert.True(t, frozen.Equal(claims.IssuedAt.Time), "iat must come from the injected clock")
		assert.True(t, frozen.Add(ttl).Equal(claims.ExpiresAt.Time), "exp must be iat plus the TTL")
	})

	t.Run("Edge: a token is valid one nanosecond before expiry", func(t *testing.T) {
		now = frozen
		token, err := manager.Issue("user-1")
		require.NoError(t, err)

		now = frozen.Add(ttl).Add(-time.Nanosecond)
		userID, err := manager.Parse(token.Value)
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("Edge: the exact expiry instant is expired", func(t *testing.T) {
		now = frozen
		token, err := manager.Issue("user-1")
		require.NoError(t, err)

		now = frozen.Add(ttl)
		_, err = manager.Parse(token.Value)
		require.ErrorIs(t, err, auth.ErrExpiredToken)
	})

	t.Run("Edge: a nil clock falls back to the system clock", func(t *testing.T) {
		system, err := auth.NewTokenManager(secret, ttl, nil)
		require.NoError(t, err)

		token, err := system.Issue("user-1")
		require.NoError(t, err)

		userID, err := system.Parse(token.Value)
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})
}
