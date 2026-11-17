package campaign

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2Hasher_HashAndVerify(t *testing.T) {
	h, err := NewArgon2Hasher()
	require.NoError(t, err)

	t.Run("Happy path: the correct password verifies against its own hash", func(t *testing.T) {
		encoded, err := h.Hash("correct horse")
		require.NoError(t, err)
		assert.True(t, h.Verify(encoded, "correct horse"))
		assert.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$"))
	})

	t.Run("Edge: a wrong password does not verify", func(t *testing.T) {
		encoded, err := h.Hash("correct horse")
		require.NoError(t, err)
		assert.False(t, h.Verify(encoded, "Correct horse"))
		assert.False(t, h.Verify(encoded, ""))
	})

	t.Run("Edge: hashing the same password twice yields different encodings because of the salt", func(t *testing.T) {
		first, err := h.Hash("same")
		require.NoError(t, err)
		second, err := h.Hash("same")
		require.NoError(t, err)
		assert.NotEqual(t, first, second)
		assert.True(t, h.Verify(first, "same"))
		assert.True(t, h.Verify(second, "same"))
	})

	t.Run("Edge: the encoding never contains the plain password", func(t *testing.T) {
		encoded, err := h.Hash("hunter2-hunter2")
		require.NoError(t, err)
		assert.NotContains(t, encoded, "hunter2")
	})

	t.Run("Edge: the dummy hash never verifies a real password", func(t *testing.T) {
		assert.NotEmpty(t, h.DummyHash())
		assert.False(t, h.Verify(h.DummyHash(), "not-the-dummy-password"))
		assert.False(t, h.Verify(h.DummyHash(), ""))
	})

	malformed := []struct {
		name    string
		encoded string
	}{
		{name: "Edge: empty encoding never verifies", encoded: ""},
		{name: "Edge: a bcrypt hash never verifies as argon2id", encoded: "$2a$12$abcdefghijklmnopqrstuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuu"},
		{name: "Edge: a truncated encoding never verifies", encoded: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA"},
		{name: "Edge: a wrong argon2 version never verifies", encoded: "$argon2id$v=16$m=19456,t=2,p=1$c2FsdA$a2V5"},
		{name: "Edge: non-base64 salt never verifies", encoded: "$argon2id$v=19$m=19456,t=2,p=1$!!!$a2V5"},
		{name: "Edge: an empty derived key never verifies", encoded: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$"},
	}
	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, h.Verify(tc.encoded, "anything"))
		})
	}
}
