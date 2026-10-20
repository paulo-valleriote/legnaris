package auth_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"legnaris/backend/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	familyA = "01900000-0000-7000-8000-0000000000aa"
	familyB = "01900000-0000-7000-8000-0000000000bb"
)

func newCSRFManager(t *testing.T) auth.CSRFTokenManager {
	t.Helper()
	return auth.NewCSRFTokenManager([]byte("test-secret"))
}

func TestCSRFTokenManager_Verify_RejectsMalformed(t *testing.T) {
	manager := newCSRFManager(t)

	cases := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "literal null", token: "null"},
		{name: "not base64", token: "not a base64 value!!"},
		{name: "base64 but not a token", token: base64.RawURLEncoding.EncodeToString([]byte("short"))},
		{name: "truncated token", token: base64.RawURLEncoding.EncodeToString(make([]byte, 24))},
		{name: "oversized token", token: base64.RawURLEncoding.EncodeToString(make([]byte, 64))},
	}

	for _, tc := range cases {
		t.Run("Error: "+tc.name, func(t *testing.T) {
			err := manager.Verify(tc.token, familyA)
			assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
		})
	}
}

func TestCSRFTokenManager_Verify_RejectsTamperedMAC(t *testing.T) {
	manager := newCSRFManager(t)

	token, err := manager.Issue(familyA)
	require.NoError(t, err)

	decoded, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.NotEmpty(t, decoded)

	t.Run("Error: a flipped MAC byte is refused", func(t *testing.T) {
		tampered := make([]byte, len(decoded))
		copy(tampered, decoded)
		tampered[len(tampered)-1] ^= 0xff

		err := manager.Verify(base64.RawURLEncoding.EncodeToString(tampered), familyA)
		assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
	})

	t.Run("Error: a flipped nonce byte is refused", func(t *testing.T) {
		tampered := make([]byte, len(decoded))
		copy(tampered, decoded)
		tampered[0] ^= 0xff

		err := manager.Verify(base64.RawURLEncoding.EncodeToString(tampered), familyA)
		assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
	})
}

func TestCSRFTokenManager_Verify_RejectsForeignFamily(t *testing.T) {
	manager := newCSRFManager(t)

	token, err := manager.Issue(familyA)
	require.NoError(t, err)

	t.Run("Error: a token minted for another family is refused", func(t *testing.T) {
		err := manager.Verify(token, familyB)
		assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
	})

	t.Run("Error: a token minted for a family is refused against an empty family", func(t *testing.T) {
		err := manager.Verify(token, "")
		assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
	})

	t.Run("Error: a token minted by another secret is refused", func(t *testing.T) {
		foreign := auth.NewCSRFTokenManager([]byte("other-secret"))
		foreignToken, issueErr := foreign.Issue(familyA)
		require.NoError(t, issueErr)

		err := manager.Verify(foreignToken, familyA)
		assert.ErrorIs(t, err, auth.ErrCSRFTokenInvalid)
	})
}

func TestCSRFTokenManager_Issue(t *testing.T) {
	t.Run("Happy path: an issued token verifies for its family", func(t *testing.T) {
		manager := newCSRFManager(t)

		token, err := manager.Issue(familyA)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		require.NoError(t, manager.Verify(token, familyA))
	})

	t.Run("Edge: the same token verifies twice (stateless, not single-use)", func(t *testing.T) {
		manager := newCSRFManager(t)

		token, err := manager.Issue(familyA)
		require.NoError(t, err)

		require.NoError(t, manager.Verify(token, familyA))
		require.NoError(t, manager.Verify(token, familyA))
	})

	t.Run("Edge: every issue yields a different value (fresh nonce, rotation changes the token)", func(t *testing.T) {
		manager := newCSRFManager(t)

		seen := make(map[string]struct{})
		for i := 0; i < 64; i++ {
			token, err := manager.Issue(familyA)
			require.NoError(t, err)
			_, duplicate := seen[token]
			require.False(t, duplicate, "each issue must mint a fresh nonce")
			seen[token] = struct{}{}
		}
	})

	t.Run("Edge: an empty family can still be issued and verified", func(t *testing.T) {
		manager := newCSRFManager(t)

		token, err := manager.Issue("")
		require.NoError(t, err)
		require.NoError(t, manager.Verify(token, ""))
	})

	t.Run("Edge: the token is 48 bytes of nonce plus MAC", func(t *testing.T) {
		manager := newCSRFManager(t)

		token, err := manager.Issue(familyA)
		require.NoError(t, err)

		decoded, err := base64.RawURLEncoding.DecodeString(token)
		require.NoError(t, err)
		assert.Len(t, decoded, 48)
	})
}

// Verify must not leak which family was expected through a distinct error
// value: every failure is the same sentinel (AUTH-10, AUTH-11).
func TestCSRFTokenManager_Verify_FailuresAreIndistinguishable(t *testing.T) {
	manager := newCSRFManager(t)

	tokens := []string{"", "not-base64", base64.RawURLEncoding.EncodeToString(make([]byte, 48))}
	for _, token := range tokens {
		err := manager.Verify(token, familyA)
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrCSRFTokenInvalid))
	}
}
