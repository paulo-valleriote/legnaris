package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger(slog.LevelInfo)

	assert.NotNil(t, logger, "logger constructor cannot return a nil value")
	assert.NotNil(t, logger.slog(), "logger constructor cannot return a nil slog.Logger")
}

func TestLoggerWithAddsAttributesToLogEntries(t *testing.T) {
	var output bytes.Buffer
	logger := Logger{log: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
		ReplaceAttr: redactSecrets,
	}))}

	logger.With("request_id", "request-123").Info(
		t.Context(),
		"request completed",
		Attribute{Key: "status", Value: "success"},
	)

	var entry map[string]interface{}
	err := json.Unmarshal(output.Bytes(), &entry)

	assert.NoError(t, err)
	assert.Equal(t, "request-123", entry["request_id"])
	assert.Equal(t, "success", entry["status"])
}

func TestLoggerRedactsSensitiveFields(t *testing.T) {
	var output bytes.Buffer
	logger := Logger{log: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
		ReplaceAttr: redactSecrets,
	}))}

	logger.Info(
		t.Context(),
		"user data",
		Attribute{Key: "email", Value: "user@example.com"},
		Attribute{Key: "api_key", Value: "secret-key"},
		Attribute{Key: "name", Value: "Ada"},
	)

	var entry map[string]interface{}
	err := json.Unmarshal(output.Bytes(), &entry)

	assert.NoError(t, err)
	assert.Equal(t, "[REDACTED]", entry["email"])
	assert.Equal(t, "[REDACTED]", entry["api_key"])
	assert.Equal(t, "Ada", entry["name"])
}

func TestToArgs(t *testing.T) {
	type testStruct struct {
		Name     string
		Message  string
		Args     []Attribute
		Expected []slog.Attr
	}

	tc := []testStruct{
		{
			Name:     "Empty attributes",
			Message:  "Test message",
			Args:     []Attribute{},
			Expected: []slog.Attr{},
		},
		{
			Name:    "Single attribute",
			Message: "Test message",
			Args: []Attribute{
				{Key: "key1", Value: "value1"},
			},
			Expected: []slog.Attr{
				slog.String("key1", "value1"),
			},
		},
		{
			Name:    "Multiple attributes",
			Message: "Test message",
			Args: []Attribute{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: "value2"},
			},
			Expected: []slog.Attr{
				slog.String("key1", "value1"),
				slog.String("key2", "value2"),
			},
		},
	}

	for _, test := range tc {
		t.Run(test.Name, func(t *testing.T) {
			result := toArgs(test.Args)
			assert.Equal(t, test.Expected, result)
		})
	}
}
