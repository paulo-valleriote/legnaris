package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_WithDefaults(t *testing.T) {
	fixed := func() time.Time { return time.Unix(0, 0) }

	tests := []struct {
		name string
		in   Config
		want Config
	}{
		{
			name: "Edge: the zero value is filled from the documented defaults",
			in:   Config{},
			want: Config{Prefetch: 10, MaxAttempts: 5, DrainTimeout: 10 * time.Second, RegisterRetryDelay: 5 * time.Second},
		},
		{
			name: "Edge: negative values are replaced by the defaults",
			in:   Config{Prefetch: -1, MaxAttempts: -4, DrainTimeout: -time.Second, RegisterRetryDelay: -time.Second},
			want: Config{Prefetch: 10, MaxAttempts: 5, DrainTimeout: 10 * time.Second, RegisterRetryDelay: 5 * time.Second},
		},
		{
			name: "Happy path: explicit values are kept",
			in:   Config{Prefetch: 3, MaxAttempts: 2, DrainTimeout: time.Second, RegisterRetryDelay: time.Millisecond, Now: fixed},
			want: Config{Prefetch: 3, MaxAttempts: 2, DrainTimeout: time.Second, RegisterRetryDelay: time.Millisecond, Now: fixed},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.withDefaults()

			assert.NotNil(t, got.Now, "a missing clock is replaced by the default")
			assert.Equal(t, tc.want.Prefetch, got.Prefetch)
			assert.Equal(t, tc.want.MaxAttempts, got.MaxAttempts)
			assert.Equal(t, tc.want.DrainTimeout, got.DrainTimeout)
			assert.Equal(t, tc.want.RegisterRetryDelay, got.RegisterRetryDelay)
		})
	}
}

func TestTierFor(t *testing.T) {
	tests := []struct {
		attempt int
		want    string
	}{
		{attempt: -1, want: "retry_30s"},
		{attempt: 0, want: "retry_30s"},
		{attempt: 1, want: "retry_2m"},
		{attempt: 2, want: "retry_5m"},
		{attempt: 100, want: "retry_5m"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, string(tierFor(tc.attempt)), "attempt %d", tc.attempt)
	}
}
