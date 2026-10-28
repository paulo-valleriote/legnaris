package email_test

import (
	"testing"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
)

// TestDestination_Names pins the closed vocabulary: the names are domain words
// and never broker routing keys (WKR-09, WKR-11, WKR-12).
func TestDestination_Names(t *testing.T) {
	tests := []struct {
		name        string
		destination email.Destination
		want        string
	}{
		{name: "Happy path: the first retry tier", destination: email.DestinationRetry30s, want: "retry_30s"},
		{name: "Happy path: the second retry tier", destination: email.DestinationRetry2m, want: "retry_2m"},
		{name: "Happy path: the last retry tier", destination: email.DestinationRetry5m, want: "retry_5m"},
		{name: "Happy path: the dead-letter route", destination: email.DestinationDeadLetter, want: "dead_letter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, string(tc.destination))
			assert.True(t, tc.destination.Valid())
		})
	}
}

// TestDestination_Valid proves the vocabulary is closed.
func TestDestination_Valid(t *testing.T) {
	tests := []struct {
		name        string
		destination email.Destination
	}{
		{name: "Edge: the zero value", destination: ""},
		{name: "Edge: a broker routing key", destination: "email.dead"},
		{name: "Edge: the work queue is never a destination", destination: "email.otp"},
		{name: "Edge: a different case", destination: "Dead_Letter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, tc.destination.Valid())
		})
	}
}
