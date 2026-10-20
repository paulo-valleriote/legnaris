// Package email owns the provider-agnostic email domain shared by the relay
// (which publishes a send intention) and the future worker (which renders it
// and calls a provider): the published message contract, the sending port, the
// embedded templates and the development sender. It imports only the standard
// library and backend/shared/*, so it can be exercised with plain values and no
// services.
package email

import "time"

// Purpose selects the embedded template that renders a message; one template
// exists per purpose (EML-11).
type Purpose string

const (
	// PurposeVerifyEmail is the email-address verification purpose.
	PurposeVerifyEmail Purpose = "verify_email"
	// PurposeLoginOTP is the login one-time-code purpose.
	PurposeLoginOTP Purpose = "login_otp"
)

// TypeOTP is the single message type this feature publishes (EML-07).
const TypeOTP = "email.otp"

// Message is the one wire shape handed to the broker. The relay encodes an
// intention into it, the broker adapter publishes it, and the future worker
// will decode it; the golden file in testdata pins the shape so the three
// cannot drift (EML-07, EML-18, EML-22).
type Message struct {
	// ID is the intention's canonical UUID v7 and also the provider
	// idempotency key (EML-18).
	ID string `json:"id"`
	// Type is TypeOTP for this feature.
	Type string `json:"type"`
	To   string `json:"to"`
	// Template is the purpose that selected the template (EML-11).
	Template string `json:"template"`
	// Data carries the code, its lifetime and the purpose.
	Data Data `json:"data"`
	// CreatedAt is the wire timestamp, always RFC 3339 UTC.
	CreatedAt time.Time `json:"created_at"`
	// Attempt is the send attempt: the relay publishes 0 and the worker bumps
	// it on a send retry. The outbox's own attempts counter counts publish
	// retries and never appears on the wire.
	Attempt int `json:"attempt"`
}

// Data is the message payload rendered by a template. The code is a
// short-lived credential: it must never reach a log line, an error message or
// a subject (EML-13).
type Data struct {
	Code             string  `json:"code"`
	ExpiresInMinutes int     `json:"expires_in_minutes"`
	Purpose          Purpose `json:"purpose"`
}
