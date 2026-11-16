package ratelimit

import (
	"context"
	"time"
)

type Allower interface {
	Now() time.Time
	Allow(key string, now time.Time) (bool, time.Duration)
}

type Store interface {
	Hit(ctx context.Context, key string, max int, window time.Duration) (bool, time.Duration, error)
}
