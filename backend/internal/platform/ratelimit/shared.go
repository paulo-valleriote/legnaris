package ratelimit

import (
	"context"
	"sync"
	"time"
)

const storeTimeout = 250 * time.Millisecond

const StoreBackoff = 2 * time.Second

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

func (s *Shared) Now() time.Time {
	return s.now()
}

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
