// Package ratelimit is a standard-library-only sliding-window limiter shared by
// the auth guards. It is deliberately domain-free: it takes string keys, never
// imports apperr, and can be reused by any route that needs a per-key cap.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a mutex-guarded sliding-window limiter. It keeps, per key, the
// timestamps of the attempts still inside the window. Attempts older than the
// window are dropped on every check and a key whose list empties is deleted, so
// memory stays bounded by the keys active in one window (AUTH-17, AUTH-19).
type Limiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	max     int
	window  time.Duration
	now     func() time.Time
}

// New builds a limiter admitting at most max attempts per rolling window. The
// clock is injected so a test can drive the window boundary without waiting.
func New(max int, window time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		entries: make(map[string][]time.Time),
		max:     max,
		window:  window,
		now:     now,
	}
}

// Now returns the limiter's clock reading. Guards pass it to Allow so the
// injected clock flows through the request path.
func (l *Limiter) Now() time.Time {
	return l.now()
}

// Allow records an attempt for key at instant now and reports whether it is
// admitted. The max-th attempt inside the window is admitted; the next one is
// refused with the time until the oldest attempt ages out (AUTH-18, AUTH-19).
func (l *Limiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	l.dropExpired(cutoff)

	active := l.entries[key]
	if l.max <= 0 || len(active) >= l.max {
		return false, l.retryAfter(active, now)
	}

	l.entries[key] = append(active, now)
	return true, 0
}

// dropExpired removes every key whose newest attempt left the window, so the
// map never holds a key that has been idle for a full window. Timestamps are
// recorded in order, so the last entry is the newest.
func (l *Limiter) dropExpired(cutoff time.Time) {
	for key, attempts := range l.entries {
		if len(attempts) == 0 || !attempts[len(attempts)-1].After(cutoff) {
			delete(l.entries, key)
		}
	}
}

// retryAfter is the time until the oldest active attempt leaves the window, or
// the whole window when nothing is recorded.
func (l *Limiter) retryAfter(active []time.Time, now time.Time) time.Duration {
	if len(active) == 0 {
		return l.window
	}
	remaining := active[0].Add(l.window).Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}
