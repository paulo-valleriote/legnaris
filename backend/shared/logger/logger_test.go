package logger

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "Happy path: password is redacted",
			key:      "password",
			value:    "secret123",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: token is redacted",
			key:      "auth_token",
			value:    "abc123",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: email is redacted",
			key:      "email",
			value:    "user@example.com",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: user_email is redacted",
			key:      "user_email",
			value:    "user@example.com",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: cpf is redacted",
			key:      "cpf",
			value:    "123.456.789-00",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: cookie with prefix is redacted",
			key:      "_cookie",
			value:    "session=abc",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: hash with suffix is redacted",
			key:      "password_hash",
			value:    "abc123",
			expected: "[REDACTED]",
		},
		{
			name:     "Edge: key with email substring is redacted",
			key:      "preemail",
			value:    "not-an-email",
			expected: "[REDACTED]",
		},
		{
			name:     "Edge: uppercase key is redacted",
			key:      "PASSWORD",
			value:    "secret123",
			expected: "[REDACTED]",
		},
		{
			name:     "Edge: empty key is not redacted",
			key:      "",
			value:    "public",
			expected: "public",
		},
		{
			name:     "Happy path: the refresh cookie name is redacted",
			key:      "__host-legnaris_refresh",
			value:    "raw-refresh-credential",
			expected: "[REDACTED]",
		},
		{
			name:     "Happy path: the CSRF cookie name is redacted",
			key:      "__host-legnaris_csrf",
			value:    "csrf-token-value",
			expected: "[REDACTED]",
		},
		{
			name:     "Edge: a cookie key with an uppercase name is redacted",
			key:      "__Host-Legnaris_Refresh",
			value:    "raw-refresh-credential",
			expected: "[REDACTED]",
		},
		{
			name:     "Edge: unrelated key is not redacted",
			key:      "username",
			value:    "john_doe",
			expected: "john_doe",
		},
		{
			name:     "Edge: name field is not redacted",
			key:      "name",
			value:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "Happy path: the prompt token count is not redacted",
			key:      "prompt_tokens",
			value:    "11",
			expected: "11",
		},
		{
			name:     "Happy path: the completion token count is not redacted",
			key:      "completion_tokens",
			value:    "7",
			expected: "7",
		},
		{
			name:     "Happy path: the total token count is not redacted",
			key:      "total_tokens",
			value:    "18",
			expected: "18",
		},
		{
			name:     "Happy path: the reasoning token count is not redacted",
			key:      "reasoning_tokens",
			value:    "6000",
			expected: "6000",
		},
		{
			name:     "Edge: a credential token is still redacted",
			key:      "access_token",
			value:    "raw-credential",
			expected: "[REDACTED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attr := slog.String(tc.key, tc.value)
			result := redactSecrets(nil, attr)
			assert.Equal(t, tc.expected, result.Value.String())
		})
	}
}

func TestRedactSecrets_DoesNotModifyKey(t *testing.T) {
	attr := slog.String("user_password", "secret123")
	result := redactSecrets(nil, attr)
	assert.Equal(t, "user_password", result.Key)
	assert.Equal(t, "[REDACTED]", result.Value.String())
}

func TestLogger_Info(t *testing.T) {
	log := NewLogger(slog.LevelInfo)
	require.NotNil(t, log)
	assert.NotPanics(t, func() {
		log.Info(context.Background(), "test message", Attribute{Key: "key", Value: "value"})
	})
}

func TestLogger_With(t *testing.T) {
	log := NewLogger(slog.LevelInfo)
	child := log.With("request_id", "abc123")
	require.NotNil(t, child)
	assert.NotPanics(t, func() {
		child.Info(context.Background(), "test with context")
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		limit int
		want  string
	}{
		{name: "Happy path: a short value is returned unchanged", value: "short", limit: 10, want: "short"},
		{name: "Edge: a value of exactly the limit is unchanged", value: "12345", limit: 5, want: "12345"},
		{name: "Edge: an empty value stays empty", value: "", limit: 5, want: ""},
		{name: "Edge: a zero limit yields an empty value", value: "abc", limit: 0, want: ""},
		{name: "Edge: a negative limit yields an empty value", value: "abc", limit: -1, want: ""},
		{name: "Happy path: an ASCII value is cut at the limit", value: "abcdef", limit: 4, want: "abcd"},
		{
			name:  "Edge: a cut that lands inside a rune backs off to the boundary",
			value: "aaaé",
			limit: 4,
			want:  "aaa",
		},
		{
			name:  "Happy path: a multi-byte value is cut on a rune boundary",
			value: strings.Repeat("é", 10),
			limit: 5,
			want:  "éé",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.value, tc.limit)
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, len(got), max(tc.limit, 0))
			assert.True(t, utf8.ValidString(got))
		})
	}
}
