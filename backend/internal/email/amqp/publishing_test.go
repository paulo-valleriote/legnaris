package amqp

import (
	"testing"
	"time"

	"legnaris/backend/internal/email"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

// TestProvision_Properties pins the derived broker properties and proves the
// code never leaks into them (EML-07, EML-13).
func TestProvision_Properties(t *testing.T) {
	msg := testMessage()
	props := provision(msg)

	t.Run("Happy path: content type, persistent delivery and identity", func(t *testing.T) {
		assert.Equal(t, "application/json", props.ContentType)
		assert.Equal(t, amqp.Persistent, props.DeliveryMode)
		assert.Equal(t, msg.ID, props.MessageId, "the intention identity is the broker-level message id")
	})

	t.Run("Happy path: the publication time is stamped so the broker can report the head message's age (OBS-03)", func(t *testing.T) {
		before := time.Now().Add(-time.Second)
		stamped := provision(msg).Timestamp
		after := time.Now().Add(time.Second)

		assert.False(t, stamped.IsZero())
		assert.True(t, stamped.After(before) && stamped.Before(after), "the stamp must be the publication time, got %s", stamped)
	})

	t.Run("Edge: an empty message is still stamped", func(t *testing.T) {
		assert.False(t, provision(email.Message{}).Timestamp.IsZero())
	})

	t.Run("Edge: the code is absent from every derived property", func(t *testing.T) {
		assert.NotContains(t, props.ContentType, msg.Data.Code)
		assert.NotContains(t, props.MessageId, msg.Data.Code)
	})

	t.Run("Edge: an empty identity still derives a code-free property set", func(t *testing.T) {
		empty := provision(email.Message{})
		assert.Equal(t, "application/json", empty.ContentType)
		assert.Empty(t, empty.MessageId)
	})
}
