package ratelimit

import (
	"context"
	"time"
)

// Allower is what the guards need from a limiter: the clock reading and an
// admit/refuse decision per key. Both the in-process Limiter and the
// Redis-backed Shared limiter satisfy it, so a guard never knows where the
// counters live.
type Allower interface {
	Now() time.Time
	Allow(key string, now time.Time) (bool, time.Duration)
}

// Store is a counter store shared between API replicas. Hit records an attempt
// for key and reports whether it is admitted, with the wait until the oldest
// attempt ages out when it is not. An error means the store could not decide.
type Store interface {
	Hit(ctx context.Context, key string, max int, window time.Duration) (bool, time.Duration, error)
}
