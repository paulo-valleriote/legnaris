package email_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	sharedjson "legnaris/backend/shared/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenPath is the pinned wire contract (EML-22). The suite reads the real
// file, so an accidental change to the contract fails here.
const goldenPath = "testdata/message.golden.json"

func readGolden(t *testing.T) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(goldenPath))
	require.NoErrorf(t, err, "read %s", goldenPath)
	return raw
}

// goldenMessage reconstructs the representative message the golden file pins.
func goldenMessage() email.Message {
	return email.Message{
		ID:       "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		Type:     email.TypeOTP,
		To:       "player@example.com",
		Template: string(email.PurposeLoginOTP),
		Data: email.Data{
			Code:             "042917",
			ExpiresInMinutes: 10,
			Purpose:          email.PurposeLoginOTP,
		},
		CreatedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Attempt:   0,
	}
}

// TestMessage_GoldenFile pins the published contract: the encoded message must
// equal the committed golden file exactly (EML-07, EML-22).
func TestMessage_GoldenFile(t *testing.T) {
	encoded, err := json.Marshal(goldenMessage())
	require.NoError(t, err)

	assert.JSONEq(t, string(readGolden(t)), string(encoded),
		"the encoded message must match the pinned contract")
}

// TestMessage_DecodesGolden proves the golden file is a valid message and that
// every field round-trips through the contract type.
func TestMessage_DecodesGolden(t *testing.T) {
	var decoded email.Message
	require.NoError(t, json.Unmarshal(readGolden(t), &decoded))

	t.Run("Happy path: the golden file decodes into the contract type", func(t *testing.T) {
		assert.Equal(t, goldenMessage(), decoded)
	})

	t.Run("Edge: a leading-zero code survives as a string", func(t *testing.T) {
		assert.Equal(t, "042917", decoded.Data.Code,
			"the code is a string so a leading zero is never lost to a number")
	})
}

// TestMessage_CreatedAtRFC3339 guards the wire timestamp format: the broker
// consumer parses it as RFC 3339 UTC, so the encoding must not drift.
func TestMessage_CreatedAtRFC3339(t *testing.T) {
	encoded, err := json.Marshal(goldenMessage())
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &raw))

	var createdAt string
	require.NoError(t, json.Unmarshal(raw["created_at"], &createdAt))
	assert.Equal(t, "2026-10-20T12:00:00Z", createdAt)
}

// TestMessage_GoldenDecodesThroughTheConsumerDecoder proves the shape the worker
// decodes is the shape the publisher pins, and that a newer publisher's extra
// field does not break it (WKR-04).
func TestMessage_GoldenDecodesThroughTheConsumerDecoder(t *testing.T) {
	t.Run("Happy path: the golden file decodes into the pinned message", func(t *testing.T) {
		var got email.Message
		require.NoError(t, sharedjson.DecodeBytes(readGolden(t), &got))
		assert.Equal(t, goldenMessage(), got)
	})

	t.Run("Edge: an unknown field from a newer publisher is tolerated", func(t *testing.T) {
		var object map[string]any
		require.NoError(t, json.Unmarshal(readGolden(t), &object))
		object["priority"] = "high"
		raw, err := json.Marshal(object)
		require.NoError(t, err)

		var got email.Message
		require.NoError(t, sharedjson.DecodeBytes(raw, &got))
		assert.Equal(t, goldenMessage(), got)
	})
}
