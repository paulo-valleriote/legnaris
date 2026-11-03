package ratelimit_test

import (
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/platform/ratelimit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const sharedWindow = time.Minute

func TestShared_Allow(t *testing.T) {
	now := time.Unix(1_000, 0)
	clock := func() time.Time { return now }

	t.Run("Happy path: the store's admit decision is returned", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "ip:1", 3, sharedWindow).Return(true, time.Duration(0), nil)
		limiter := ratelimit.NewShared(store, 3, sharedWindow, clock, nil)

		allowed, retryAfter := limiter.Allow("ip:1", now)

		assert.True(t, allowed)
		assert.Zero(t, retryAfter)
	})

	t.Run("Happy path: the store's refusal carries its retryAfter", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "ip:1", 3, sharedWindow).Return(false, 42*time.Second, nil)
		limiter := ratelimit.NewShared(store, 3, sharedWindow, clock, nil)

		allowed, retryAfter := limiter.Allow("ip:1", now)

		assert.False(t, allowed)
		assert.Equal(t, 42*time.Second, retryAfter)
	})

	t.Run("Edge: a store failure fails open to the local limiter and reports the error", func(t *testing.T) {
		boom := errors.New("redis down")
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "ip:1", 2, sharedWindow).Return(false, time.Duration(0), boom).Once()
		var reported []error
		limiter := ratelimit.NewShared(store, 2, sharedWindow, clock, func(err error) { reported = append(reported, err) })

		first, _ := limiter.Allow("ip:1", now)
		second, _ := limiter.Allow("ip:1", now)
		third, retryAfter := limiter.Allow("ip:1", now)

		assert.True(t, first, "the fallback admits under its own cap")
		assert.True(t, second)
		assert.False(t, third, "the fallback still enforces the per-pod cap")
		assert.Positive(t, retryAfter)
		require.Len(t, reported, 1, "the breaker skips the store after the first failure")
		assert.ErrorIs(t, reported[0], boom)
	})

	t.Run("Edge: a nil failure hook does not panic", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "k", 1, sharedWindow).Return(false, time.Duration(0), errors.New("x"))
		limiter := ratelimit.NewShared(store, 1, sharedWindow, clock, nil)

		assert.NotPanics(t, func() { limiter.Allow("k", now) })
	})

	t.Run("Edge: a non-positive cap refuses without consulting the store", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		limiter := ratelimit.NewShared(store, 0, sharedWindow, clock, nil)

		allowed, retryAfter := limiter.Allow("ip:1", now)

		assert.False(t, allowed)
		assert.Equal(t, sharedWindow, retryAfter)
	})

	t.Run("Edge: an empty key is forwarded as its own bucket", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "", 1, sharedWindow).Return(true, time.Duration(0), nil)
		limiter := ratelimit.NewShared(store, 1, sharedWindow, clock, nil)

		allowed, _ := limiter.Allow("", now)

		assert.True(t, allowed)
	})

	t.Run("Edge: Now reads the injected clock", func(t *testing.T) {
		limiter := ratelimit.NewShared(mocks.NewMockStore(t), 1, sharedWindow, clock, nil)

		assert.Equal(t, now, limiter.Now())
	})

	t.Run("Edge: after a failure the store is skipped until the backoff elapses, then retried", func(t *testing.T) {
		current := now
		store := mocks.NewMockStore(t)
		store.EXPECT().Hit(mock.Anything, "k", 5, sharedWindow).Return(false, time.Duration(0), errors.New("down")).Once()
		limiter := ratelimit.NewShared(store, 5, sharedWindow, func() time.Time { return current }, nil)

		limiter.Allow("k", current)
		current = now.Add(ratelimit.StoreBackoff - time.Millisecond)
		limiter.Allow("k", current)

		current = now.Add(ratelimit.StoreBackoff)
		store.EXPECT().Hit(mock.Anything, "k", 5, sharedWindow).Return(true, time.Duration(0), nil).Once()
		allowed, _ := limiter.Allow("k", current)

		assert.True(t, allowed, "the store is consulted again once the backoff elapsed")
	})
}
