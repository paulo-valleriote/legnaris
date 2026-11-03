package ratelimit

import (
	"context"
	"sync"
	"time"
)

// storeTimeout bounds one store round-trip so a hung Redis degrades to the
// local fallback instead of stalling the request path.
const storeTimeout = 250 * time.Millisecond

// StoreBackoff is how long a limiter bypasses a failed store, so an outage costs
// one timeout per backoff instead of one per request.
const StoreBackoff = 2 * time.Second

// Shared decides through a Store common to every replica, so the configured cap
// holds across the whole deployment. When the store cannot answer it fails
// open to a per-process Limiter: availability wins over strictness, and the
// effective cap degrades to the documented N-times approximation until the
// store recovers (ADR 0003).
type Shared struct {
	store     Store
	fallback  *Limiter
	max       int
	window    time.Duration
	now       func() time.Time
	onFailure func(error)

	mu        sync.Mutex
	skipUntil time.Time
}

// NewShared builds a store-backed limiter. onFailure, when non-nil, is told
// about every store error so the caller can log or count the degradation.
func NewShared(store Store, max int, window time.Duration, now func() time.Time, onFailure func(error)) *Shared {
	if now == nil {
		now = time.Now
	}
	return &Shared{
		store:     store,
		fallback:  New(max, window, now),
		max:       max,
		window:    window,
		now:       now,
		onFailure: onFailure,
	}
}

// Now returns the limiter's clock reading.
func (s *Shared) Now() time.Time {
	return s.now()
}

// Allow records an attempt for key in the shared store; on a store error it
// answers from the local fallback instead. A non-positive cap refuses without
// touching the store, matching Limiter.
func (s *Shared) Allow(key string, now time.Time) (bool, time.Duration) {
	if s.max <= 0 {
		return false, s.window
	}

	if s.bypassed(now) {
		return s.fallback.Allow(key, now)
	}

	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()

	allowed, retryAfter, err := s.store.Hit(ctx, key, s.max, s.window)
	if err != nil {
		s.trip(s.now())
		if s.onFailure != nil {
			s.onFailure(err)
		}
		return s.fallback.Allow(key, now)
	}
	return allowed, retryAfter
}

func (s *Shared) bypassed(at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return at.Before(s.skipUntil)
}

func (s *Shared) trip(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipUntil = at.Add(StoreBackoff)
}
