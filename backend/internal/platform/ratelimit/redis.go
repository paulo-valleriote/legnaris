package ratelimit

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/shared/idgen"

	"github.com/redis/go-redis/v9"
)

var hitScript = redis.NewScript(`
local t = redis.call('TIME')
local now = t[1] * 1000000 + t[2]
local window = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
if redis.call('ZCARD', KEYS[1]) >= max then
  local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  local wait = tonumber(oldest[2]) + window - now
  if wait < 0 then wait = 0 end
  return {0, wait}
end
redis.call('ZADD', KEYS[1], now, ARGV[3])
redis.call('PEXPIRE', KEYS[1], math.ceil(window / 1000))
return {1, 0}
`)

type RedisStore struct {
	client    redis.Scripter
	namespace string
}

func NewRedisStore(client redis.Scripter, namespace string) *RedisStore {
	return &RedisStore{client: client, namespace: namespace}
}

func (s *RedisStore) Hit(ctx context.Context, key string, max int, window time.Duration) (bool, time.Duration, error) {
	redisKey := fmt.Sprintf("rl:%s:%s", s.namespace, key)

	member, err := idgen.New()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit member: %w", err)
	}

	res, err := hitScript.Run(ctx, s.client, []string{redisKey}, window.Microseconds(), max, member).Int64Slice()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit hit %q: %w", s.namespace, err)
	}
	if len(res) != 2 {
		return false, 0, fmt.Errorf("rate limit hit %q: unexpected reply length %d", s.namespace, len(res))
	}
	return res[0] == 1, time.Duration(res[1]) * time.Microsecond, nil
}
