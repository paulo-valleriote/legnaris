package amqp

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/amqp/mocks"
	sharedjson "legnaris/backend/shared/json"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jobID = "0190a1b2-0000-7000-8000-0000000000a1"

func TestNewConsumer(t *testing.T) {
	tests := []struct {
		name         string
		cfg          ConsumerConfig
		wantPrefetch int
		wantRetry    time.Duration
	}{
		{name: "Edge: zero values take the documented defaults", cfg: ConsumerConfig{}, wantPrefetch: DefaultPrefetch, wantRetry: DefaultRetryDelay},
		{name: "Edge: negative values take the defaults too", cfg: ConsumerConfig{Prefetch: -1, RetryDelay: -time.Second}, wantPrefetch: DefaultPrefetch, wantRetry: DefaultRetryDelay},
		{name: "Happy path: explicit values are kept", cfg: ConsumerConfig{Prefetch: 2, RetryDelay: time.Second}, wantPrefetch: 2, wantRetry: time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConsumer(tc.cfg, nil, logger.NewLogger(slog.LevelError))
			assert.Equal(t, tc.wantPrefetch, c.prefetch)
			assert.Equal(t, tc.wantRetry, c.retryDelay)
		})
	}
}

func TestDecodeMessage(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantID  string
		wantErr bool
	}{
		{name: "Happy path: a message with a canonical UUID v7 is accepted", body: `{"generation_id":"` + jobID + `","kind":"character"}`, wantID: jobID},
		{name: "Error: an empty body is refused", body: ``, wantErr: true},
		{name: "Error: a body that is not JSON is refused", body: `not json`, wantErr: true},
		{name: "Error: a missing identifier is refused", body: `{"kind":"character"}`, wantErr: true},
		{name: "Error: an identifier that is not a UUID is refused", body: `{"generation_id":"abc","kind":"character"}`, wantErr: true},
		{name: "Edge: an upper-case identifier is refused because ids are canonical", body: `{"generation_id":"0190A1B2-0000-7000-8000-0000000000A1","kind":"character"}`, wantErr: true},
		{name: "Edge: an unknown field from a newer producer is tolerated", body: `{"generation_id":"` + jobID + `","kind":"character","extra":1}`, wantID: jobID},
		{name: "Error: trailing data after the message is refused", body: `{"generation_id":"` + jobID + `","kind":"character"} {}`, wantErr: true},
		{name: "Error: a body over the size bound is refused", body: `{"generation_id":"` + jobID + `","kind":"character","pad":"` + strings.Repeat("a", sharedjson.MaxBodySize) + `"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := decodeMessage([]byte(tc.body))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, msg.ID)
			assert.Equal(t, "character", msg.Kind)
		})
	}
}

func TestProcessDelivery(t *testing.T) {
	log := logger.NewLogger(slog.LevelError)
	validBody := []byte(`{"generation_id":"` + jobID + `","kind":"character"}`)

	t.Run("Happy path: a completed job is acknowledged once", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Ack(false).Return(nil).Once()
		var got generation.Message
		handle := func(_ context.Context, msg generation.Message) error {
			got = msg
			return nil
		}

		processDelivery(context.Background(), validBody, ack, handle, time.Millisecond, log)
		assert.Equal(t, jobID, got.ID)
	})

	t.Run("Edge: a malformed message is rejected without requeue so it dead-letters", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Reject(false).Return(nil).Once()
		handle := func(context.Context, generation.Message) error {
			t.Fatal("the worker must never see a malformed message")
			return nil
		}

		processDelivery(context.Background(), []byte(`{"generation_id":"nope"}`), ack, handle, time.Millisecond, log)
	})

	t.Run("Error: a failed job is requeued, never acknowledged", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Nack(false, true).Return(nil).Once()
		handle := func(context.Context, generation.Message) error { return errors.New("mongo down") }

		processDelivery(context.Background(), validBody, ack, handle, time.Millisecond, log)
	})

	t.Run("Edge: a cancelled context still settles the delivery without waiting", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Nack(false, true).Return(nil).Once()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		handle := func(context.Context, generation.Message) error { return errors.New("stopped") }

		processDelivery(ctx, validBody, ack, handle, time.Hour, log)
	})

	t.Run("Edge: an acknowledgement error does not panic the consumer", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Ack(false).Return(errors.New("channel closed")).Once()
		handle := func(context.Context, generation.Message) error { return nil }

		processDelivery(context.Background(), validBody, ack, handle, time.Millisecond, log)
	})

	t.Run("Edge: the handler receives a context that is passed through", func(t *testing.T) {
		ack := mocks.NewMockAcknowledger(t)
		ack.EXPECT().Ack(false).Return(nil).Once()
		type key struct{}
		ctx := context.WithValue(context.Background(), key{}, "marker")
		handle := func(got context.Context, _ generation.Message) error {
			assert.Equal(t, "marker", got.Value(key{}))
			return nil
		}

		processDelivery(ctx, validBody, ack, handle, time.Millisecond, log)
	})

}
