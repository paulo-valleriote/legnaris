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

// TestDevSender proves the development sender logs the code in clear, keeps
// the recipient readable, and makes no provider call (EML-10, D10).
func TestDevSender(t *testing.T) {
	tests := []struct {
		name     string
		outbound email.Outbound
		wantText string
	}{
		{
			name: "Happy path: the code in the text body is logged in clear",
			outbound: email.Outbound{
				IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
				To:             "player@example.com",
				Subject:        "Your Legnaris verification code",
				Text:           "Your code is 042917",
				HTML:           "<p>Your code is 042917</p>",
			},
			wantText: "Your code is 042917",
		},
		{
			name: "Edge: an empty body is logged as empty without failing",
			outbound: email.Outbound{
				IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
				To:             "player@example.com",
				Subject:        "Your Legnaris verification code",
			},
			wantText: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log := loggermocks.NewMockStructuredLogger(t)

			var captured []logger.Attribute
			log.EXPECT().Info(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(_ context.Context, _ string, args ...logger.Attribute) { captured = args }).Once()

			require.NoError(t, email.NewDevSender(log).Send(context.Background(), tc.outbound))

			assert.Equal(t, tc.outbound.IdempotencyKey, attributeValue(captured, "idempotency_key"))
			assert.Equal(t, tc.outbound.To, attributeValue(captured, "to"), "the development sender keeps the recipient readable")
			assert.Equal(t, tc.wantText, attributeValue(captured, "text"))
		})
	}
}
