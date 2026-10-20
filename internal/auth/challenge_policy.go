package auth

import "time"

// ChallengePolicy is the resolved, configuration-backed set of limits the
// challenge service enforces. It is built once at boot and carried by the
// service; it is never stored per challenge (data-model.md §3).
type ChallengePolicy struct {
	// Lifetime is the deadline set at issuance and at each resend (OTP-10).
	Lifetime time.Duration
	// Ceiling is the absolute limit from creation a resend can never pass
	// (OTP-10).
	Ceiling time.Duration
	// ResendCooldown is the window in which a resend is answered but records no
	// new send (OTP-10).
	ResendCooldown time.Duration
	// MaxAttempts is the verification attempts per challenge before it becomes
	// unusable (OTP-05).
	MaxAttempts int
	// DailySendCap is the send intentions per day before the API answers
	// unavailable (OTP-13).
	DailySendCap int
	// UserWindow and MaxPerUserWindow are the durable per-account bound on the
	// challenge routes (OTP-14).
	UserWindow       time.Duration
	MaxPerUserWindow int
}

// Deadline returns the challenge's next expiry: now plus the configured
// lifetime, clamped to the challenge's absolute ceiling, so a resend can extend
// the deadline but never past createdAt+Ceiling (OTP-10).
func (p ChallengePolicy) Deadline(createdAt, now time.Time) time.Time {
	deadline := now.Add(p.Lifetime)
	if ceiling := createdAt.Add(p.Ceiling); deadline.After(ceiling) {
		return ceiling
	}
	return deadline
}

// CooldownElapsed reports whether a resend may record a new send at now: it is
// allowed exactly at lastSentAt+ResendCooldown and rejected one nanosecond
// before (OTP-10).
func (p ChallengePolicy) CooldownElapsed(lastSentAt, now time.Time) bool {
	return !now.Before(lastSentAt.Add(p.ResendCooldown))
}

// DayStart returns the UTC midnight of the given instant, the start of the
// window the daily send cap counts from (OTP-13). The instant is resolved in
// UTC first, so a configured window is always a true UTC day.
func (p ChallengePolicy) DayStart(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
