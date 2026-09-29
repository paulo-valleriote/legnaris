package logger

import (
	"context"
	"log/slog"
	"testing"

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
