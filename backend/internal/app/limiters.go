package app

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/shared/logger"

	"github.com/redis/go-redis/v9"
)

// limiterErrorKey is the log field carrying a store failure.
const limiterErrorKey = "error"

// limiterFailureLogInterval spaces the store-outage warnings of one limiter.
const limiterFailureLogInterval = time.Minute

// newLimiterClient opens the shared-store client when REDIS_URL is set. An
// empty URL returns a nil client, which selects the in-process limiters. The
// connection is lazy, so an unreachable Redis does not block the boot: the
// limiters fail open until it answers.
func newLimiterClient(redisURL string) (redis.Scripter, []io.Closer, error) {
	if redisURL == "" {
		return nil, nil, nil
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	return client, []io.Closer{client}, nil
}

// newLimiter builds the limiter for one guard namespace. Without a Redis client
// it is the in-process limiter, so the effective cap is the configured value
// per replica; with one, the cap is shared across every replica and degrades
// to the per-pod limiter only while Redis is unreachable (ADR 0003).
func newLimiter(client redis.Scripter, namespace string, max int, window time.Duration, onFailure func(error)) ratelimit.Allower {
	if client == nil {
		return ratelimit.New(max, window, time.Now)
	}
	return ratelimit.NewShared(ratelimit.NewRedisStore(client, namespace), max, window, time.Now, onFailure)
}

// storeFailureReporter logs a limiter's store failure at most once per
// interval: during a Redis outage every guarded request fails the store call,
// and one line per request would drown the log.
func storeFailureReporter(log logger.StructuredLogger, namespace string, interval time.Duration, now func() time.Time) func(error) {
	var (
		mu   sync.Mutex
		last time.Time
	)
	return func(err error) {
		mu.Lock()
		defer mu.Unlock()

		at := now()
		if !last.IsZero() && at.Sub(last) < interval {
			return
		}
		last = at
		log.Warn(context.Background(), "rate limit store unavailable; falling back to the per-pod limiter",
			logger.Attribute{Key: "limiter", Value: namespace},
			logger.Attribute{Key: limiterErrorKey, Value: err.Error()},
		)
	}
}
