package requestcache

import (
	"context"
	"sync"
)

type scopeKey struct{}

type scope struct {
	mu     sync.Mutex
	values map[any]any
}

func WithScope(ctx context.Context) context.Context {
	if _, ok := ctx.Value(scopeKey{}).(*scope); ok {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, &scope{values: map[any]any{}})
}

func Memo[T any](ctx context.Context, key any, load func(context.Context) (T, error)) (T, error) {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return load(ctx)
	}
	if value, found := s.get(key); found {
		if typed, ok := value.(T); ok {
			return typed, nil
		}
	}
	value, err := load(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	s.set(key, value)
	return value, nil
}

func Forget(ctx context.Context, key any) {
	if s, ok := ctx.Value(scopeKey{}).(*scope); ok {
		s.delete(key)
	}
}

func (s *scope) get(key any) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	return value, ok
}

func (s *scope) set(key, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

func (s *scope) delete(key any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
}
