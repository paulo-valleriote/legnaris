package email_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/email"
	"legnaris/backend/shared/logger"
	loggermocks "legnaris/backend/shared/logger/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func attributeValue(attributes []logger.Attribute, key string) string {
	for _, attribute := range attributes {
		if attribute.Key == key {
			return attribute.Value
		}
	}
	return ""
}

// TestLogSender proves the development sender makes no provider call, records a
// masked recipient, and never logs the code (EML-10, EML-13).
func TestLogSender(t *testing.T) {
	log := loggermocks.NewMockStructuredLogger(t)

	var captured []logger.Attribute
	log.EXPECT().Info(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ string, args ...logger.Attribute) { captured = args }).Once()

	sender := email.NewLogSender(log)
	outbound := email.Outbound{
		IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		To:             "a@example.com",
		Subject:        "Your Legnaris login code",
		Text:           "code 042917",
		HTML:           "<p>code 042917</p>",
	}

	require.NoError(t, sender.Send(context.Background(), outbound))

	t.Run("Happy path: the recipient is masked as the first character plus the domain", func(t *testing.T) {
		assert.Equal(t, "a***@example.com", attributeValue(captured, "to"))
	})

	t.Run("Happy path: the identity identifies the message", func(t *testing.T) {
		assert.Equal(t, outbound.IdempotencyKey, attributeValue(captured, "idempotency_key"))
	})

	t.Run("Edge: the code never appears in any logged attribute", func(t *testing.T) {
		for _, attribute := range captured {
			assert.NotContains(t, attribute.Value, "042917", "attribute %q must not leak the code", attribute.Key)
		}
	})

	t.Run("Edge: the message body is never logged", func(t *testing.T) {
		for _, attribute := range captured {
			assert.NotEqual(t, outbound.Text, attribute.Value)
			assert.NotEqual(t, outbound.HTML, attribute.Value)
		}
	})
}

// TestMaskRecipient pins the masking contract, including the malformed cases
// (EML-13).
func TestMaskRecipient(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "Happy path: a typical address keeps the first character and the domain", in: "a@example.com", want: "a***@example.com"},
		{name: "Happy path: a longer local part is fully masked", in: "player@example.com", want: "p***@example.com"},
		{name: "Edge: no domain becomes the placeholder", in: "player@", want: "***"},
		{name: "Edge: no at sign becomes the placeholder", in: "player", want: "***"},
		{name: "Edge: empty becomes the placeholder", in: "", want: "***"},
		{name: "Edge: a leading at sign becomes the placeholder", in: "@example.com", want: "***"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, email.MaskRecipient(tc.in))
		})
	}
}
