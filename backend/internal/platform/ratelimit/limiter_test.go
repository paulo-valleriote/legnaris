package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWindow = time.Minute

func TestLimiter_Allow(t *testing.T) {
	t.Run("Happy path: repeated attempts under the cap are admitted", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(3, testWindow, func() time.Time { return now })

		for i := 0; i < 3; i++ {
			allowed, retryAfter := limiter.Allow("ip:1", now)
			assert.True(t, allowed, "attempt %d is under the cap", i+1)
			assert.Zero(t, retryAfter)
		}
	})

	t.Run("Edge: a fresh key is admitted", func(t *testing.T) {
		limiter := New(1, testWindow, nil)

		allowed, retryAfter := limiter.Allow("ip:fresh", time.Unix(0, 0))
		assert.True(t, allowed)
		assert.Zero(t, retryAfter)
	})

	t.Run("Edge: the cap-th attempt is admitted and the next is refused with a positive retryAfter", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(2, testWindow, func() time.Time { return now })

		first, _ := limiter.Allow("ip:1", now)
		second, _ := limiter.Allow("ip:1", now)
		require.True(t, first)
		require.True(t, second)

		third, retryAfter := limiter.Allow("ip:1", now)
		assert.False(t, third)
		assert.Positive(t, retryAfter)
		assert.LessOrEqual(t, retryAfter, testWindow)

		// The refused attempt is not recorded, so the window still frees when the
		// oldest admitted attempt ages out.
		later := now.Add(30 * time.Second)
		refused, retryAfterLater := limiter.Allow("ip:1", later)
		assert.False(t, refused)
		assert.Equal(t, 30*time.Second, retryAfterLater)
	})

	t.Run("Edge: exhausting one key leaves another admitted", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(1, testWindow, func() time.Time { return now })

		first, _ := limiter.Allow("ip:1", now)
		require.True(t, first)
		blocked, _ := limiter.Allow("ip:1", now)
		require.False(t, blocked)

		other, _ := limiter.Allow("ip:2", now)
		assert.True(t, other, "a different key is independent of an exhausted one")
	})

	t.Run("Edge: the window frees once the oldest attempt ages out", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(1, testWindow, func() time.Time { return now })

		allowed, _ := limiter.Allow("ip:1", now)
		require.True(t, allowed)
		blocked, _ := limiter.Allow("ip:1", now)
		require.False(t, blocked)

		after := now.Add(testWindow + time.Second)
		freed, retryAfter := limiter.Allow("ip:1", after)
		assert.True(t, freed)
		assert.Zero(t, retryAfter)
		assert.Len(t, limiter.entries["ip:1"], 1, "attempts older than the window must be dropped")
	})

	t.Run("Edge: a key idle for a full window is dropped from the map", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(1, testWindow, func() time.Time { return now })

		_, _ = limiter.Allow("ip:1", now)
		_, _ = limiter.Allow("ip:2", now)
		require.Len(t, limiter.entries, 2)

		after := now.Add(testWindow + time.Second)
		allowed, _ := limiter.Allow("ip:3", after)
		require.True(t, allowed)

		assert.Len(t, limiter.entries, 1, "every key idle for a full window must be dropped")
		_, stale := limiter.entries["ip:1"]
		assert.False(t, stale)
	})

	t.Run("Edge: a non-positive cap refuses every attempt", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(0, testWindow, func() time.Time { return now })

		allowed, retryAfter := limiter.Allow("ip:1", now)
		assert.False(t, allowed)
		assert.Equal(t, testWindow, retryAfter)
	})

	t.Run("Edge: concurrent attempts admit exactly the cap", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		const cap = 5
		limiter := New(cap, testWindow, func() time.Time { return now })

		const workers = 50
		var wg sync.WaitGroup
		var mu sync.Mutex
		admitted := 0

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if ok, _ := limiter.Allow("ip:shared", now); ok {
					mu.Lock()
					admitted++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, cap, admitted, "the mutex must serialise the admission to exactly the cap")
	})
}

func TestLimiter_ExpiredAttemptsOfActiveKey(t *testing.T) {
	t.Run("Error: a steady client under the cap is never refused", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(3, testWindow, func() time.Time { return now })

		for i := 0; i < 30; i++ {
			allowed, _ := limiter.Allow("user:steady", now)
			require.True(t, allowed, "request %d is 1 per 30s, under 3 per minute", i+1)
			now = now.Add(30 * time.Second)
		}
	})

	t.Run("Edge: a burst frees up as its oldest attempts age out while newer ones remain", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(3, testWindow, func() time.Time { return now })

		for _, offset := range []time.Duration{0, 20 * time.Second, 40 * time.Second} {
			allowed, _ := limiter.Allow("user:burst", now.Add(offset))
			require.True(t, allowed)
		}

		at := now.Add(70 * time.Second)
		allowed, _ := limiter.Allow("user:burst", at)
		assert.True(t, allowed, "the attempt at +0s left the window, leaving room for one")

		allowed, retryAfter := limiter.Allow("user:burst", at)
		assert.False(t, allowed, "attempts at +20s, +40s and +70s fill the cap again")
		assert.Equal(t, 10*time.Second, retryAfter, "the +20s attempt leaves the window at +80s")
	})

	t.Run("Edge: an attempt exactly at the cutoff has left the window", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(1, testWindow, func() time.Time { return now })

		first, _ := limiter.Allow("user:edge", now)
		require.True(t, first)

		allowed, _ := limiter.Allow("user:edge", now.Add(testWindow))
		assert.True(t, allowed)
	})

	t.Run("Edge: memory of an active key stays bounded by the cap", func(t *testing.T) {
		now := time.Unix(1_000, 0)
		limiter := New(3, testWindow, func() time.Time { return now })

		for i := 0; i < 100; i++ {
			limiter.Allow("user:bounded", now)
			now = now.Add(30 * time.Second)
		}

		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		assert.LessOrEqual(t, len(limiter.entries["user:bounded"]), 3)
	})
}
