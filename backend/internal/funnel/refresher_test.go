package funnel_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/funnel"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	mu    sync.Mutex
	stats funnel.Stats
	err   error
	calls int
}

func (s *fakeStore) Stats(context.Context) (funnel.Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return funnel.Stats{}, s.err
	}
	return s.stats, nil
}

func (s *fakeStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type fakeObserver struct {
	mu    sync.Mutex
	stats []funnel.Stats
	calls chan struct{}
}

func (o *fakeObserver) FunnelStats(stats funnel.Stats) {
	o.mu.Lock()
	o.stats = append(o.stats, stats)
	o.mu.Unlock()
	if o.calls != nil {
		o.calls <- struct{}{}
	}
}

func (o *fakeObserver) reported() []funnel.Stats {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]funnel.Stats(nil), o.stats...)
}

func TestRefresherRunOnce(t *testing.T) {
	failure := errors.New("mongo unavailable")

	tests := []struct {
		name    string
		stats   funnel.Stats
		err     error
		want    []funnel.Stats
		wantErr error
	}{
		{
			name:  "Happy path: the stored milestones reach the observer",
			stats: funnel.Stats{Accounts: 12, FirstSheetWithin24h: 3},
			want:  []funnel.Stats{{Accounts: 12, FirstSheetWithin24h: 3}},
		},
		{
			name: "Edge: an empty account set is reported as zeroes",
			want: []funnel.Stats{{}},
		},
		{
			name:    "Error: a store failure is returned and nothing is reported",
			err:     failure,
			wantErr: failure,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{stats: tc.stats, err: tc.err}
			observer := &fakeObserver{}
			refresher := funnel.NewRefresher(store, observer, logger.NewLogger(0), time.Hour)

			err := refresher.RunOnce(t.Context())

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, observer.reported())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, observer.reported())
		})
	}
}

func TestRefresherRun(t *testing.T) {
	store := &fakeStore{stats: funnel.Stats{Accounts: 1}}
	observer := &fakeObserver{calls: make(chan struct{}, 1)}
	refresher := funnel.NewRefresher(store, observer, logger.NewLogger(0), time.Millisecond)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- refresher.Run(ctx) }()

	select {
	case <-observer.calls:
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("the refresher did not report within the deadline")
	}

	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, store.callCount(), 1)
}
