package auth

import "time"

// Clock is the single time source for every auth timestamp the service writes
// or compares. Production passes time.Now; tests pass a frozen or stepped
// function so expiry boundaries are crossed without sleeping (PRQ-16, SC-007).
type Clock func() time.Time

// Now reports the clock's instant. A nil clock means the system clock, matching
// the rate limiter's existing convention, so an unwired caller stays correct.
func (c Clock) Now() time.Time {
	if c == nil {
		return time.Now()
	}
	return c()
}
