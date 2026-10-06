package auth_test

import (
	"strings"
	"testing"

	"legnaris/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHasher(t *testing.T) {
	hasher, err := auth.NewPasswordHasher(4)
	require.NoError(t, err)

	t.Run("Happy path: hash then compare succeeds", func(t *testing.T) {
		hash, err := hasher.Hash("correct horse battery staple")
		require.NoError(t, err)

		assert.NotEqual(t, "correct horse battery staple", hash)
		require.NoError(t, hasher.Compare(hash, "correct horse battery staple"))
	})

	t.Run("Error: wrong password fails with a typed mismatch", func(t *testing.T) {
		hash, err := hasher.Hash("s3cret-password")
		require.NoError(t, err)

		err = hasher.Compare(hash, "wrong-password")
		require.Error(t, err)
		assert.ErrorIs(t, err, auth.ErrPasswordMismatch)
	})

	t.Run("Edge: empty password is still hashed and comparable", func(t *testing.T) {
		hash, err := hasher.Hash("")
		require.NoError(t, err)

		require.NoError(t, hasher.Compare(hash, ""))
	})

	t.Run("Error: cost below the bcrypt minimum is rejected", func(t *testing.T) {
		_, err := auth.NewPasswordHasher(1)
		require.ErrorIs(t, err, auth.ErrInvalidBcryptCost)
	})

	t.Run("Error: cost above the bcrypt maximum is rejected", func(t *testing.T) {
		_, err := auth.NewPasswordHasher(32)
		require.ErrorIs(t, err, auth.ErrInvalidBcryptCost)
	})

	t.Run("Edge: the hash never contains the raw password", func(t *testing.T) {
		hash, err := hasher.Hash("plaintext-needle")
		require.NoError(t, err)

		assert.False(t, strings.Contains(hash, "plaintext-needle"))
	})
}
