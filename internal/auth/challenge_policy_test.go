package auth_test

import (
	"testing"
	"time"

	"legnaris/internal/auth"

	"github.com/stretchr/testify/assert"
)

// testPolicy is a representative configuration: lifetime 10m, ceiling 30m,
// cooldown 60s, cap 5 attempts (data-model.md §3).
func testPolicy() auth.ChallengePolicy {
	return auth.ChallengePolicy{
		Lifetime:         10 * time.Minute,
		Ceiling:          30 * time.Minute,
		ResendCooldown:   60 * time.Second,
		MaxAttempts:      5,
		DailySendCap:     80,
		UserWindow:       time.Hour,
		MaxPerUserWindow: 10,
	}
}

// TestChallengePolicy_Deadline pins the clamped deadline: now+lifetime, never
// past the absolute ceiling (OTP-10).
func TestChallengePolicy_Deadline(t *testing.T) {
	policy := testPolicy()
	createdAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: a fresh deadline is now plus the lifetime", func(t *testing.T) {
		now := createdAt.Add(time.Minute)
		assert.Equal(t, now.Add(policy.Lifetime), policy.Deadline(createdAt, now))
	})

	t.Run("Edge: a deadline past the ceiling is clamped", func(t *testing.T) {
		now := createdAt.Add(25 * time.Minute)
		assert.Equal(t, createdAt.Add(policy.Ceiling), policy.Deadline(createdAt, now))
	})

	t.Run("Edge: a deadline exactly at the ceiling is not clamped further", func(t *testing.T) {
		now := createdAt.Add(policy.Ceiling - policy.Lifetime)
		assert.Equal(t, createdAt.Add(policy.Ceiling), policy.Deadline(createdAt, now))
	})
}

// TestChallengePolicy_CooldownElapsed pins the cooldown boundary: suppressed one
// nanosecond before it ends, allowed exactly at the boundary, with no sleep
// (OTP-10, OTP-20).
func TestChallengePolicy_CooldownElapsed(t *testing.T) {
	policy := testPolicy()
	lastSent := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "Edge: immediately after the send it is suppressed", now: lastSent, want: false},
		{name: "Edge: one nanosecond before the boundary it is suppressed", now: lastSent.Add(policy.ResendCooldown - time.Nanosecond), want: false},
		{name: "Happy path: exactly at the boundary it is allowed", now: lastSent.Add(policy.ResendCooldown), want: true},
		{name: "Happy path: well past the boundary it is allowed", now: lastSent.Add(time.Hour), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policy.CooldownElapsed(lastSent, tc.now))
		})
	}
}

// TestChallengePolicy_DeadlineWithin pins the resend deadline: now plus the
// lifetime, clamped to the stored absolute ceiling (OTP-10).
func TestChallengePolicy_DeadlineWithin(t *testing.T) {
	policy := testPolicy()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		ceiling time.Time
		want    time.Time
	}{
		{
			name:    "Happy path: a ceiling inside the lifetime clamps the deadline",
			ceiling: now.Add(5 * time.Minute),
			want:    now.Add(5 * time.Minute),
		},
		{
			name:    "Edge: a ceiling beyond the lifetime is never reached",
			ceiling: now.Add(30 * time.Minute),
			want:    now.Add(policy.Lifetime),
		},
		{
			name:    "Edge: a ceiling exactly at the lifetime is not clamped further",
			ceiling: now.Add(policy.Lifetime),
			want:    now.Add(policy.Lifetime),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policy.DeadlineWithin(tc.ceiling, now))
		})
	}
}

// TestChallengePolicy_DayStart pins the daily-cap window origin: the UTC
// midnight of the given instant, regardless of the input's location (OTP-13).
func TestChallengePolicy_DayStart(t *testing.T) {
	policy := testPolicy()

	t.Run("Happy path: the day start is the UTC midnight", func(t *testing.T) {
		now := time.Date(2026, 10, 20, 23, 59, 59, 999, time.UTC)
		assert.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), policy.DayStart(now))
	})

	t.Run("Edge: a non-UTC instant resolves to its own UTC midnight", func(t *testing.T) {
		loc := time.FixedZone("UTC+2", 2*60*60)
		now := time.Date(2026, 10, 21, 1, 0, 0, 0, loc) // 2026-10-20T23:00Z
		assert.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), policy.DayStart(now))
	})
}
