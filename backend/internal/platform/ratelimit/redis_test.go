package ratelimit_test

import (
	"context"
	"os"
	"testing"
	"time"

	"legnaris/backend/internal/platform/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRedisClient returns a client for the test Redis. REDIS_TEST_URL selects a
// real server (skipping when it is unreachable); otherwise an in-process
// miniredis, which runs the same Lua script, keeps the suite hermetic.
func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	opts := &redis.Options{}
	if url := os.Getenv("REDIS_TEST_URL"); url != "" {
		parsed, err := redis.ParseURL(url)
		require.NoError(t, err)
		if parsed.DB == 0 {
			t.Skip("REDIS_TEST_URL must select a non-zero DB: the suite flushes it")
		}
		opts = parsed
	} else {
		opts.Addr = miniredis.RunT(t).Addr()
	}

	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable — skipping integration test: %v", err)
	}
	require.NoError(t, client.FlushDB(ctx).Err())
	return client
}

func TestRedisStore_Hit(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: the cap-th attempt is admitted and the next is refused", func(t *testing.T) {
		store := ratelimit.NewRedisStore(newRedisClient(t), "test")

		for i := 0; i < 2; i++ {
			allowed, retryAfter, err := store.Hit(ctx, "ip:1", 2, time.Minute)
			require.NoError(t, err)
			assert.True(t, allowed)
			assert.Zero(t, retryAfter)
		}

		allowed, retryAfter, err := store.Hit(ctx, "ip:1", 2, time.Minute)
		require.NoError(t, err)
		assert.False(t, allowed)
		assert.Positive(t, retryAfter)
		assert.LessOrEqual(t, retryAfter, time.Minute)
	})

	t.Run("Happy path: two stores on one Redis share a single budget", func(t *testing.T) {
		client := newRedisClient(t)
		podA := ratelimit.NewRedisStore(client, "test")
		podB := ratelimit.NewRedisStore(client, "test")

		first, _, err := podA.Hit(ctx, "ip:1", 2, time.Minute)
		require.NoError(t, err)
		second, _, err := podB.Hit(ctx, "ip:1", 2, time.Minute)
		require.NoError(t, err)
		third, _, err := podA.Hit(ctx, "ip:1", 2, time.Minute)
		require.NoError(t, err)
		fourth, _, err := podB.Hit(ctx, "ip:1", 2, time.Minute)
		require.NoError(t, err)

		assert.True(t, first)
		assert.True(t, second)
		assert.False(t, third, "the third attempt across pods exceeds the cap of 2")
		assert.False(t, fourth)
	})

	t.Run("Edge: the window expiring frees the key", func(t *testing.T) {
		store := ratelimit.NewRedisStore(newRedisClient(t), "test")
		window := 300 * time.Millisecond

		allowed, _, err := store.Hit(ctx, "ip:1", 1, window)
		require.NoError(t, err)
		require.True(t, allowed)
		refused, _, err := store.Hit(ctx, "ip:1", 1, window)
		require.NoError(t, err)
		require.False(t, refused)

		time.Sleep(window + 150*time.Millisecond)

		again, _, err := store.Hit(ctx, "ip:1", 1, window)
		require.NoError(t, err)
		assert.True(t, again)
	})

	t.Run("Edge: keys are independent", func(t *testing.T) {
		store := ratelimit.NewRedisStore(newRedisClient(t), "test")

		_, _, err := store.Hit(ctx, "ip:1", 1, time.Minute)
		require.NoError(t, err)
		other, _, err := store.Hit(ctx, "ip:2", 1, time.Minute)
		require.NoError(t, err)

		assert.True(t, other)
	})

	t.Run("Edge: an empty key is a valid bucket of its own", func(t *testing.T) {
		store := ratelimit.NewRedisStore(newRedisClient(t), "test")

		first, _, err := store.Hit(ctx, "", 1, time.Minute)
		require.NoError(t, err)
		second, _, err := store.Hit(ctx, "", 1, time.Minute)
		require.NoError(t, err)

		assert.True(t, first)
		assert.False(t, second)
	})

	t.Run("Edge: namespaces do not share budgets", func(t *testing.T) {
		client := newRedisClient(t)
		auth := ratelimit.NewRedisStore(client, "auth")
		user := ratelimit.NewRedisStore(client, "user")

		_, _, err := auth.Hit(ctx, "k", 1, time.Minute)
		require.NoError(t, err)
		allowed, _, err := user.Hit(ctx, "k", 1, time.Minute)
		require.NoError(t, err)

		assert.True(t, allowed)
	})

	t.Run("Edge: a refused attempt is not recorded", func(t *testing.T) {
		store := ratelimit.NewRedisStore(newRedisClient(t), "test")
		window := 400 * time.Millisecond

		_, _, err := store.Hit(ctx, "k", 1, window)
		require.NoError(t, err)
		for i := 0; i < 5; i++ {
			refused, _, err := store.Hit(ctx, "k", 1, window)
			require.NoError(t, err)
			require.False(t, refused)
		}

		time.Sleep(window + 150*time.Millisecond)

		allowed, _, err := store.Hit(ctx, "k", 1, window)
		require.NoError(t, err)
		assert.True(t, allowed, "refusals must not extend the window")
	})

	t.Run("Error: an unreachable Redis returns an error", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 100 * time.Millisecond, MaxRetries: -1})
		t.Cleanup(func() { _ = client.Close() })
		store := ratelimit.NewRedisStore(client, "test")

		_, _, err := store.Hit(ctx, "k", 1, time.Minute)

		assert.Error(t, err)
	})
}

func TestShared_AcrossInstances(t *testing.T) {
	t.Run("Happy path: two limiter instances enforce one cap", func(t *testing.T) {
		client := newRedisClient(t)
		podA := ratelimit.NewShared(ratelimit.NewRedisStore(client, "auth"), 3, time.Minute, nil, nil)
		podB := ratelimit.NewShared(ratelimit.NewRedisStore(client, "auth"), 3, time.Minute, nil, nil)

		admitted := 0
		for i := 0; i < 3; i++ {
			if ok, _ := podA.Allow("ip:1", podA.Now()); ok {
				admitted++
			}
			if ok, _ := podB.Allow("ip:1", podB.Now()); ok {
				admitted++
			}
		}

		assert.Equal(t, 3, admitted, "six attempts across two pods admit exactly the cap")
	})
}
