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

const logErrorKey = "error"

const limiterFailureLogInterval = time.Minute

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

func newLimiter(client redis.Scripter, namespace string, max int, window time.Duration, onFailure func(error)) ratelimit.Allower {
	if client == nil {
		return ratelimit.New(max, window, time.Now)
	}
	return ratelimit.NewShared(ratelimit.NewRedisStore(client, namespace), max, window, time.Now, onFailure)
}

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
			logger.Attribute{Key: logErrorKey, Value: err.Error()},
		)
	}
}
