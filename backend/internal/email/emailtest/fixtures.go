package emailtest

import (
	"time"

	"legnaris/backend/internal/email"
)

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

func Outbound() email.Outbound {
	return email.Outbound{
		IdempotencyKey: "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
		To:             "player@example.com",
		Subject:        "Your Legnaris login code",
		Text:           "code 042917",
		HTML:           "<p>code 042917</p>",
	}
}
