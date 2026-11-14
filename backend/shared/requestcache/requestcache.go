// Package requestcache memoizes reads for the lifetime of one HTTP request. A
// request opens a scope with WithScope; Memo then runs its loader once per key
// and serves the stored value to every later read in the same request. Without a
// scope every call runs its loader, so code outside a request is unaffected.
package requestcache

import (
	"context"
	"sync"
)

type scopeKey struct{}

// scope holds the memoized values of one request. Its mutex guards values, since
// a request may read from concurrent goroutines.
type scope struct {
	mu     sync.Mutex
	values map[any]any
}

// WithScope returns ctx carrying a new memo scope. A context that already carries
// a scope keeps it, so an inner middleware never empties the outer request's memo.
func WithScope(ctx context.Context) context.Context {
	if _, ok := ctx.Value(scopeKey{}).(*scope); ok {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, &scope{values: map[any]any{}})
}

// Memo returns the value stored under key in the request scope. On a miss it runs
// load and stores the value only when load succeeds, so a failed read is retried
// by the next caller. key must be comparable. Values are shared by every reader of
// the request, so callers must not mutate them.
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

// Forget drops the value stored under key, so the next Memo for it loads again.
// Writers call it after they change what the key reads, within the same request.
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
