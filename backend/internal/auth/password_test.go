package auth_test

import (
	"strings"
	"testing"

	"legnaris/backend/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHasher(t *testing.T) {
	hasher, err := auth.NewPasswordHasher(12)
	require.NoError(t, err, "the documented floor (12) must be accepted")

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

	t.Run("Error: cost below the required floor is rejected", func(t *testing.T) {
		_, err := auth.NewPasswordHasher(11)
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

// TestPasswordHasherDummyHash pins the digest the unknown-address sign-in
// compares against: it must be a bcrypt hash at the hasher's own work factor,
// so both credential failures cost the same work whatever AUTH_BCRYPT_COST is.
// bcrypt spends the rounds encoded in the digest it is given, so a digest
// pinned to a fixed cost would re-open the account-existence oracle the
// comparison exists to close (TFA-05).
func TestPasswordHasherDummyHash(t *testing.T) {
	t.Run("Happy path: the dummy hash carries the configured cost", func(t *testing.T) {
		hasher, err := auth.NewPasswordHasher(12)
		require.NoError(t, err)

		cost, err := bcrypt.Cost([]byte(hasher.DummyHash()))
		require.NoError(t, err)
		assert.Equal(t, 12, cost)
	})

	t.Run("Edge: a raised work factor moves the dummy hash with it", func(t *testing.T) {
		hasher, err := auth.NewPasswordHasher(13)
		require.NoError(t, err)

		cost, err := bcrypt.Cost([]byte(hasher.DummyHash()))
		require.NoError(t, err)
		assert.Equal(t, 13, cost, "the dummy hash must never fall behind the configured cost")
	})

	t.Run("Edge: the dummy hash is stable across calls", func(t *testing.T) {
		hasher, err := auth.NewPasswordHasher(12)
		require.NoError(t, err)

		assert.Equal(t, hasher.DummyHash(), hasher.DummyHash())
	})

	t.Run("Edge: the dummy hash follows the cost and nothing else", func(t *testing.T) {
		floor, err := auth.NewPasswordHasher(12)
		require.NoError(t, err)

		raised, err := auth.NewPasswordHasher(13)
		require.NoError(t, err)

		assert.NotEqual(t, floor.DummyHash(), raised.DummyHash(),
			"a different work factor must produce a different digest")
		assert.Equal(t, len(floor.DummyHash()), len(raised.DummyHash()),
			"the digest must stay a well-formed bcrypt string")
	})

	t.Run("Edge: no password matches the dummy hash", func(t *testing.T) {
		hasher, err := auth.NewPasswordHasher(12)
		require.NoError(t, err)

		for _, password := range []string{"", "correct horse battery staple"} {
			assert.ErrorIs(t, hasher.Compare(hasher.DummyHash(), password), auth.ErrPasswordMismatch,
				"the dummy hash must never accept a password")
		}
	})

	t.Run("Error: a cost outside the accepted range builds no dummy hash", func(t *testing.T) {
		_, err := auth.NewPasswordHasher(11)
		require.ErrorIs(t, err, auth.ErrInvalidBcryptCost)

		_, err = auth.NewPasswordHasher(32)
		require.ErrorIs(t, err, auth.ErrInvalidBcryptCost)
	})
}
