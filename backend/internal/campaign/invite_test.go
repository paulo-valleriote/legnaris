package campaign

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidInviteCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "Happy path: a 24 character base64url code is valid", code: "Abcdefghijklmnopqrstuvwx", want: true},
		{name: "Edge: dash and underscore are allowed", code: "Abc-efghijklmnopqrstuv_x", want: true},
		{name: "Edge: empty code is invalid", code: "", want: false},
		{name: "Edge: 23 characters is invalid", code: strings.Repeat("a", 23), want: false},
		{name: "Edge: 25 characters is invalid", code: strings.Repeat("a", 25), want: false},
		{name: "Edge: padding character is invalid", code: strings.Repeat("a", 23) + "=", want: false},
		{name: "Edge: whitespace is invalid", code: strings.Repeat("a", 23) + " ", want: false},
		{name: "Edge: a newline inside the code is invalid", code: strings.Repeat("a", 12) + "\n" + strings.Repeat("a", 11), want: false},
		{name: "Edge: non-ASCII characters are invalid", code: strings.Repeat("á", 24), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, validInviteCode(tc.code))
		})
	}
}

func TestNewInviteCode(t *testing.T) {
	t.Run("Happy path: a generated code is 24 url-safe characters", func(t *testing.T) {
		code, err := newInviteCode()
		require.NoError(t, err)
		assert.True(t, validInviteCode(code))
	})

	t.Run("Edge: consecutive codes never repeat", func(t *testing.T) {
		seen := make(map[string]struct{}, 200)
		for range 200 {
			code, err := newInviteCode()
			require.NoError(t, err)
			_, dup := seen[code]
			require.False(t, dup)
			seen[code] = struct{}{}
		}
	})
}

func TestValidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "Happy path: a typical passphrase is valid", password: "correct horse battery", want: true},
		{name: "Edge: empty password is invalid", password: "", want: false},
		{name: "Edge: 7 characters is invalid", password: strings.Repeat("a", 7), want: false},
		{name: "Edge: exactly 8 characters is valid", password: strings.Repeat("a", 8), want: true},
		{name: "Edge: exactly 128 characters is valid", password: strings.Repeat("a", 128), want: true},
		{name: "Edge: 129 characters is invalid", password: strings.Repeat("a", 129), want: false},
		{name: "Edge: 8 multi-byte characters count as 8", password: strings.Repeat("á", 8), want: true},
		{name: "Edge: 7 multi-byte characters are invalid (counted by runes, not bytes)", password: strings.Repeat("á", 7), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidPassword(tc.password))
		})
	}
}
