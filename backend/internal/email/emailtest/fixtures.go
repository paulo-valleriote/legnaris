package emailtest

import (
	"time"

	"legnaris/backend/internal/email"
)

// Message builds a representative published-message value. It is built over
// email.Message only, never an outbox.Intent, so a suite using it stays inside
// the sending side (EML-05).
func Message() email.Message {
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

// Outbound builds a representative rendered message ready to hand to a Sender.
func Outbound() email.Outbound {
	return email.Outbound{
		IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		To:             "player@example.com",
		Subject:        "Your Legnaris login code",
		Text:           "code 042917",
		HTML:           "<p>code 042917</p>",
	}
}
