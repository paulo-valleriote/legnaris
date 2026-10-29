package amqp

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"legnaris/backend/internal/email"
	"legnaris/backend/shared/idgen"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_PublishConsumeAcknowledge drives a real broker end to end:
// it declares the topology, publishes a message, consumes it and acknowledges
// it. It self-skips without RABBITMQ_URL, so no unit run needs a broker
// (WKR-27, SC-009).
func TestIntegration_PublishConsumeAcknowledge(t *testing.T) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		t.Skip("RABBITMQ_URL is not set; skipping the broker integration suite")
	}

	broker, err := Dial(url, Config{Prefetch: 5, Logger: logger.NewLogger(slog.LevelError)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = broker.Close() })
	require.NoError(t, broker.DeclareTopology())

	id, err := idgen.New()
	require.NoError(t, err)
	msg := email.Message{
		ID:        id,
		Type:      email.TypeOTP,
		To:        "player@example.com",
		Template:  string(email.PurposeLoginOTP),
		Data:      email.Data{Code: "042917", ExpiresInMinutes: 10, Purpose: email.PurposeLoginOTP},
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := broker.Consume(ctx)
	require.NoError(t, err)
	require.NoError(t, broker.Publish(ctx, msg))

	for {
		select {
		case delivery, ok := <-stream:
			require.True(t, ok, "the stream ended before the message arrived")
			var got email.Message
			if err := decodeForTest(delivery.Body, &got); err != nil || got.ID != id {
				require.NoError(t, delivery.Handle.Ack())
				continue
			}
			assert.Equal(t, msg.To, got.To)
			require.NoError(t, delivery.Handle.Ack())
			return
		case <-ctx.Done():
			t.Fatal("the published message never arrived")
		}
	}
}

func decodeForTest(body []byte, dst *email.Message) error {
	return sharedjson.DecodeBytes(body, dst)
}
