package requestcache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"legnaris/backend/shared/requestcache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errLoad = errors.New("load failed")

func TestMemo(t *testing.T) {
	tests := []struct {
		name       string
		scoped     bool
		loadErr    error
		calls      int
		wantLoads  int32
		wantValues []string
		wantErr    error
	}{
		{name: "Edge: without a scope every read loads", scoped: false, calls: 3, wantLoads: 3, wantValues: []string{"v", "v", "v"}},
		{name: "Happy path: a scoped key loads once and serves later reads", scoped: true, calls: 3, wantLoads: 1, wantValues: []string{"v", "v", "v"}},
		{name: "Error: a failed load is returned and not stored", scoped: true, loadErr: errLoad, calls: 2, wantLoads: 2, wantErr: errLoad},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.scoped {
				ctx = requestcache.WithScope(ctx)
			}
			var loads atomic.Int32
			load := func(context.Context) (string, error) {
				loads.Add(1)
				if tc.loadErr != nil {
					return "", tc.loadErr
				}
				return "v", nil
			}

			for i := range tc.calls {
				value, err := requestcache.Memo(ctx, "key", load)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr, "call %d", i)
					assert.Empty(t, value)
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, tc.wantValues[i], value)
			}
			assert.Equal(t, tc.wantLoads, loads.Load())
		})
	}
}

func TestMemo_KeysAreIndependent(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		wantLoad int32
	}{
		{name: "Edge: an empty key set loads nothing", keys: nil, wantLoad: 0},
		{name: "Edge: distinct keys each load once", keys: []string{"a", "b", "a", "b"}, wantLoad: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestcache.WithScope(context.Background())
			var loads atomic.Int32
			for _, key := range tc.keys {
				_, err := requestcache.Memo(ctx, key, func(context.Context) (int, error) {
					loads.Add(1)
					return len(key), nil
				})
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantLoad, loads.Load())
		})
	}
}

func TestMemo_ConcurrentReadsAreSafe(t *testing.T) {
	tests := []struct {
		name    string
		readers int
	}{
		{name: "Edge: many concurrent readers get the stored value", readers: 32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestcache.WithScope(context.Background())
			var wg sync.WaitGroup
			results := make([]string, tc.readers)
			for i := range tc.readers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					value, err := requestcache.Memo(ctx, "key", func(context.Context) (string, error) { return "v", nil })
					if err == nil {
						results[i] = value
					}
				}()
			}
			wg.Wait()
			for _, value := range results {
				assert.Equal(t, "v", value)
			}
		})
	}
}

func TestWithScope(t *testing.T) {
	tests := []struct {
		name      string
		nested    bool
		wantLoads int32
	}{
		{name: "Happy path: a scope opened once memoizes for the request", nested: false, wantLoads: 1},
		{name: "Edge: a nested scope keeps the outer memo", nested: true, wantLoads: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := requestcache.WithScope(context.Background())
			if tc.nested {
				ctx = requestcache.WithScope(ctx)
			}
			var loads atomic.Int32
			load := func(context.Context) (string, error) {
				loads.Add(1)
				return "v", nil
			}
			_, err := requestcache.Memo(ctx, "key", load)
			require.NoError(t, err)
			_, err = requestcache.Memo(ctx, "key", load)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLoads, loads.Load())
		})
	}
}

func TestForget(t *testing.T) {
	tests := []struct {
		name      string
		scoped    bool
		forget    bool
		wantLoads int32
	}{
		{name: "Happy path: forgetting a key makes the next read load again", scoped: true, forget: true, wantLoads: 2},
		{name: "Edge: forgetting without a read changes nothing", scoped: true, forget: false, wantLoads: 1},
		{name: "Edge: forgetting outside a request is a no-op", scoped: false, forget: true, wantLoads: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.scoped {
				ctx = requestcache.WithScope(ctx)
			}
			var loads atomic.Int32
			load := func(context.Context) (string, error) {
				loads.Add(1)
				return "v", nil
			}
			_, err := requestcache.Memo(ctx, "key", load)
			require.NoError(t, err)
			if tc.forget {
				requestcache.Forget(ctx, "key")
			}
			_, err = requestcache.Memo(ctx, "key", load)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLoads, loads.Load())
		})
	}
}
