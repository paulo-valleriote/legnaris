package auth_test

import (
	"testing"
	"time"

	"legnaris/internal/auth"

	"github.com/stretchr/testify/assert"
)

// TestChallenge_Usable pins the usability predicate on a fixed clock, so the
// expiry boundary is crossed with no sleep (OTP-04…OTP-06, OTP-20).
func TestChallenge_Usable(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	consumedAt := now.Add(-time.Minute)
	const maxAttempts = 5

	tests := []struct {
		name      string
		challenge auth.Challenge
		want      bool
	}{
		{
			name:      "Edge: a zero-value record is unusable",
			challenge: auth.Challenge{},
			want:      false,
		},
		{
			name:      "Happy path: a fresh record is usable",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute)},
			want:      true,
		},
		{
			name:      "Edge: a consumed record is unusable",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), ConsumedAt: &consumedAt},
			want:      false,
		},
		{
			name:      "Edge: an expired record is unusable",
			challenge: auth.Challenge{ExpiresAt: now.Add(-time.Nanosecond)},
			want:      false,
		},
		{
			name:      "Edge: expiry exactly at the instant is expired",
			challenge: auth.Challenge{ExpiresAt: now},
			want:      false,
		},
		{
			name:      "Edge: one nanosecond before expiry is still usable",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Nanosecond)},
			want:      true,
		},
		{
			name:      "Edge: a record at the attempt cap is unusable",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), Attempts: maxAttempts},
			want:      false,
		},
		{
			name:      "Edge: one below the attempt cap is usable",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), Attempts: maxAttempts - 1},
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.challenge.Usable(now, maxAttempts))
		})
	}
}

// TestChallenge_FailureResult pins the internal classification that reaches only
// metrics and logs, never the uniform response (OTP-08, OTP-16).
func TestChallenge_FailureResult(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	consumedAt := now.Add(-time.Minute)
	const maxAttempts = 5

	tests := []struct {
		name      string
		challenge auth.Challenge
		want      string
	}{
		{
			name:      "Edge: a consumed record is classified as consumed",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), ConsumedAt: &consumedAt},
			want:      auth.ResultConsumed,
		},
		{
			name:      "Edge: an expired record is classified as expired",
			challenge: auth.Challenge{ExpiresAt: now},
			want:      auth.ResultExpired,
		},
		{
			name:      "Edge: a record at the cap is classified as exhausted",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), Attempts: maxAttempts},
			want:      auth.ResultExhausted,
		},
		{
			name:      "Happy path: a usable record is classified as invalid",
			challenge: auth.Challenge{ExpiresAt: now.Add(time.Minute), Attempts: 1},
			want:      auth.ResultInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.challenge.FailureResult(now, maxAttempts))
		})
	}
}
