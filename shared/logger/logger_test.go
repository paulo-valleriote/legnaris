package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()

	assert.NotNil(t, logger, "logger constructor cannot return a nil value")
	assert.NotNil(t, logger.slog(), "logger constructor cannot return a nil slog.Logger")
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
