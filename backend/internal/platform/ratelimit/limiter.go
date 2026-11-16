package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	max     int
	window  time.Duration
	now     func() time.Time
}

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

func (l *Limiter) Now() time.Time {
	return l.now()
}

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

func (l *Limiter) dropExpired(cutoff time.Time) {
	for key, attempts := range l.entries {
		if len(attempts) == 0 || !attempts[len(attempts)-1].After(cutoff) {
			delete(l.entries, key)
			continue
		}

		expired := 0
		for expired < len(attempts) && !attempts[expired].After(cutoff) {
			expired++
		}
		if expired > 0 {
			l.entries[key] = attempts[expired:]
		}
	}
}

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
