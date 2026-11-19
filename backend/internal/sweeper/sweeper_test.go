package sweeper_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/sweeper"
	sweepermocks "legnaris/backend/internal/sweeper/mocks"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	fixedNow   = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	errHandler = errors.New("handler failed")
	errStore   = errors.New("store unavailable")
)

type fakeStore struct {
	jobs          []sweeper.Job[string]
	claimErr      error
	oldest        time.Time
	hasOldest     bool
	oldestErr     error
	rescheduleErr error
	markDeadErr   error
	claims        []claimCall
	rescheduled   []transition
	dead          []transition
}

type claimCall struct {
	claimer string
	now     time.Time
	lease   time.Duration
	limit   int
}

type transition struct {
	id     string
	now    time.Time
	nextAt time.Time
	reason string
}

func (s *fakeStore) Claim(_ context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[string], error) {
	s.claims = append(s.claims, claimCall{claimer: claimer, now: now, lease: lease, limit: limit})
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	jobs := s.jobs
	s.jobs = nil
	return jobs, nil
}

func (s *fakeStore) Reschedule(_ context.Context, id string, now, nextAt time.Time, reason string) error {
	s.rescheduled = append(s.rescheduled, transition{id: id, now: now, nextAt: nextAt, reason: reason})
	return s.rescheduleErr
}

func (s *fakeStore) MarkDead(_ context.Context, id string, now time.Time, reason string) error {
	s.dead = append(s.dead, transition{id: id, now: now, reason: reason})
	return s.markDeadErr
}

func (s *fakeStore) OldestPending(context.Context) (time.Time, bool, error) {
	return s.oldest, s.hasOldest, s.oldestErr
}

func testLogger() logger.StructuredLogger {
	return logger.NewLogger(slog.LevelError)
}

func validSpec(store sweeper.Store[string], handle sweeper.Handler[string]) sweeper.QueueSpec[string] {
	return sweeper.QueueSpec[string]{
		Name:       "q",
		Interval:   time.Minute,
		AttemptCap: 5,
		Batch:      10,
		Store:      store,
		Handle:     handle,
	}
}

func okHandler(context.Context, sweeper.Job[string], time.Time) error { return nil }

func failHandler(context.Context, sweeper.Job[string], time.Time) error { return errHandler }

func TestNewQueue(t *testing.T) {
	store := &fakeStore{}

	tests := []struct {
		name    string
		mutate  func(spec *sweeper.QueueSpec[string])
		wantErr bool
	}{
		{name: "Happy path: a complete spec builds a queue", mutate: func(*sweeper.QueueSpec[string]) {}},
		{name: "Edge: an empty name is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Name = "" }, wantErr: true},
		{name: "Edge: a zero interval is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Interval = 0 }, wantErr: true},
		{name: "Edge: a negative interval is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Interval = -time.Second }, wantErr: true},
		{name: "Edge: a zero attempt cap is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.AttemptCap = 0 }, wantErr: true},
		{name: "Edge: a zero batch is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Batch = 0 }, wantErr: true},
		{name: "Edge: a nil store is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Store = nil }, wantErr: true},
		{name: "Edge: a nil handler is rejected", mutate: func(s *sweeper.QueueSpec[string]) { s.Handle = nil }, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := validSpec(store, okHandler)
			tc.mutate(&spec)

			queue, err := sweeper.NewQueue(spec)
			if tc.wantErr {
				assert.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "q", queue.Name)
			assert.Equal(t, time.Minute, queue.Interval)
		})
	}
}

func TestNew(t *testing.T) {
	store := &fakeStore{}
	obs := sweepermocks.NewMockObserver(t)
	queue := mustQueue(t, validSpec(store, okHandler))
	other := mustQueue(t, sweeper.QueueSpec[string]{
		Name: "other", Interval: time.Second, AttemptCap: 1, Batch: 1, Store: store, Handle: okHandler,
	})

	tests := []struct {
		name    string
		queues  []sweeper.Queue
		obs     sweeper.Observer
		wantErr bool
	}{
		{name: "Happy path: one queue builds a sweeper", queues: []sweeper.Queue{queue}, obs: obs},
		{name: "Happy path: distinct names share the table", queues: []sweeper.Queue{queue, other}, obs: obs},
		{name: "Edge: an empty table is rejected", queues: nil, obs: obs, wantErr: true},
		{name: "Edge: a duplicate name is rejected", queues: []sweeper.Queue{queue, queue}, obs: obs, wantErr: true},
		{name: "Edge: a nil observer is rejected", queues: []sweeper.Queue{queue}, obs: nil, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sw, err := sweeper.New(tc.queues, "relay-test", tc.obs, testLogger())
			if tc.wantErr {
				require.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				assert.Nil(t, sw)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, sw)
		})
	}
}

func mustQueue(t *testing.T, spec sweeper.QueueSpec[string]) sweeper.Queue {
	t.Helper()
	queue, err := sweeper.NewQueue(spec)
	require.NoError(t, err)
	return queue
}

func TestRunOnceHandling(t *testing.T) {
	t.Run("Happy path: a completed item gets no transition and the claim uses the lease and batch", func(t *testing.T) {
		store := &fakeStore{jobs: []sweeper.Job[string]{{ID: "a", Payload: "x"}}}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending("q", time.Duration(0)).Once()
		sw := newSweeper(t, validSpec(store, okHandler), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))

		assert.Empty(t, store.rescheduled)
		assert.Empty(t, store.dead)
		require.Len(t, store.claims, 1)
		assert.Equal(t, claimCall{claimer: "relay-test", now: fixedNow, lease: sweeper.Lease, limit: 10}, store.claims[0])
	})

	t.Run("Happy path: the handler receives the job and the pass time", func(t *testing.T) {
		store := &fakeStore{jobs: []sweeper.Job[string]{{ID: "a", Attempts: 2, Payload: "body"}}}
		var got sweeper.Job[string]
		var gotNow time.Time
		handle := func(_ context.Context, job sweeper.Job[string], now time.Time) error {
			got, gotNow = job, now
			return nil
		}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		sw := newSweeper(t, validSpec(store, handle), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))

		assert.Equal(t, sweeper.Job[string]{ID: "a", Attempts: 2, Payload: "body"}, got)
		assert.Equal(t, fixedNow, gotNow)
	})

	tests := []struct {
		name         string
		attempts     int
		attemptCap   int
		handleErr    error
		wantDead     bool
		wantResched  bool
		wantFailures int
	}{
		{name: "Happy path: a first failure below the cap is rescheduled", attempts: 0, attemptCap: 5, handleErr: errHandler, wantResched: true, wantFailures: 1},
		{name: "Edge: the attempt just below the cap is rescheduled", attempts: 3, attemptCap: 5, handleErr: errHandler, wantResched: true, wantFailures: 1},
		{name: "Edge: the attempt that reaches the cap is dead", attempts: 4, attemptCap: 5, handleErr: errHandler, wantDead: true, wantFailures: 1},
		{name: "Edge: an attempt past the cap is dead", attempts: 9, attemptCap: 5, handleErr: errHandler, wantDead: true, wantFailures: 1},
		{name: "Edge: a cap of one makes the first failure dead", attempts: 0, attemptCap: 1, handleErr: errHandler, wantDead: true, wantFailures: 1},
		{name: "Edge: a permanent failure is dead at once", attempts: 0, attemptCap: 5, handleErr: fmt.Errorf("%w: bad identity", sweeper.ErrPermanent), wantDead: true, wantFailures: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{jobs: []sweeper.Job[string]{{ID: "a", Attempts: tc.attempts}}}
			obs := sweepermocks.NewMockObserver(t)
			obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
			obs.EXPECT().Failed("q").Times(tc.wantFailures)

			spec := validSpec(store, func(context.Context, sweeper.Job[string], time.Time) error { return tc.handleErr })
			spec.AttemptCap = tc.attemptCap
			sw := newSweeper(t, spec, obs)

			require.NoError(t, sw.RunOnce(context.Background(), fixedNow))

			if tc.wantDead {
				require.Len(t, store.dead, 1)
				assert.Equal(t, transition{id: "a", now: fixedNow, reason: tc.handleErr.Error()}, store.dead[0])
				assert.Empty(t, store.rescheduled)
			}
			if tc.wantResched {
				require.Len(t, store.rescheduled, 1)
				assert.Equal(t, transition{id: "a", now: fixedNow, nextAt: fixedNow, reason: tc.handleErr.Error()}, store.rescheduled[0])
				assert.Empty(t, store.dead)
			}
		})
	}

	t.Run("Edge: a failing item in a batch does not stop the others", func(t *testing.T) {
		store := &fakeStore{jobs: []sweeper.Job[string]{{ID: "bad"}, {ID: "good"}}}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		obs.EXPECT().Failed("q").Once()
		handled := []string{}
		handle := func(_ context.Context, job sweeper.Job[string], _ time.Time) error {
			handled = append(handled, job.ID)
			if job.ID == "bad" {
				return errHandler
			}
			return nil
		}
		sw := newSweeper(t, validSpec(store, handle), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))

		assert.Equal(t, []string{"bad", "good"}, handled)
		require.Len(t, store.rescheduled, 1)
		assert.Equal(t, "bad", store.rescheduled[0].id)
	})

	t.Run("Edge: an empty claim runs no handler and reports zero age", func(t *testing.T) {
		store := &fakeStore{}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending("q", time.Duration(0)).Once()
		handled := false
		sw := newSweeper(t, validSpec(store, func(context.Context, sweeper.Job[string], time.Time) error {
			handled = true
			return nil
		}), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))

		assert.False(t, handled)
		assert.Empty(t, store.rescheduled)
		assert.Empty(t, store.dead)
	})

	t.Run("Error: a failed reschedule is logged and the pass still returns nil", func(t *testing.T) {
		store := &fakeStore{
			jobs:          []sweeper.Job[string]{{ID: "a"}, {ID: "b"}},
			rescheduleErr: errStore,
		}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		obs.EXPECT().Failed("q").Times(2)
		sw := newSweeper(t, validSpec(store, failHandler), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))
		assert.Len(t, store.rescheduled, 2, "a failed transition does not stop the batch")
	})

	t.Run("Error: a failed mark-dead is logged and the pass still returns nil", func(t *testing.T) {
		store := &fakeStore{
			jobs:        []sweeper.Job[string]{{ID: "a", Attempts: 4}},
			markDeadErr: errStore,
		}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		obs.EXPECT().Failed("q").Once()
		sw := newSweeper(t, validSpec(store, failHandler), obs)

		require.NoError(t, sw.RunOnce(context.Background(), fixedNow))
		assert.Len(t, store.dead, 1)
	})
}

func TestRunOnceErrors(t *testing.T) {
	t.Run("Error: a claim failure is returned and runs no handler", func(t *testing.T) {
		store := &fakeStore{claimErr: errStore}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		handled := false
		sw := newSweeper(t, validSpec(store, func(context.Context, sweeper.Job[string], time.Time) error {
			handled = true
			return nil
		}), obs)

		err := sw.RunOnce(context.Background(), fixedNow)
		require.ErrorIs(t, err, errStore)
		assert.False(t, handled)
	})

	t.Run("Error: an oldest-pending failure is returned and the claim still runs", func(t *testing.T) {
		store := &fakeStore{oldestErr: errStore, jobs: []sweeper.Job[string]{{ID: "a"}}}
		obs := sweepermocks.NewMockObserver(t)
		handled := false
		sw := newSweeper(t, validSpec(store, func(context.Context, sweeper.Job[string], time.Time) error {
			handled = true
			return nil
		}), obs)

		err := sw.RunOnce(context.Background(), fixedNow)
		require.ErrorIs(t, err, errStore)
		assert.True(t, handled)
	})

	t.Run("Error: one failing queue does not stop the others", func(t *testing.T) {
		failing := &fakeStore{claimErr: errStore}
		healthy := &fakeStore{jobs: []sweeper.Job[string]{{ID: "a"}}}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Times(2)

		badSpec := validSpec(failing, okHandler)
		badSpec.Name = "failing"
		goodSpec := validSpec(healthy, okHandler)
		goodSpec.Name = "healthy"
		sw, err := sweeper.New([]sweeper.Queue{mustQueue(t, badSpec), mustQueue(t, goodSpec)}, "relay-test", obs, testLogger())
		require.NoError(t, err)

		runErr := sw.RunOnce(context.Background(), fixedNow)
		require.ErrorIs(t, runErr, errStore)
		assert.Len(t, healthy.claims, 1, "the healthy queue still ran")
	})
}

func TestRunOnceDue(t *testing.T) {
	type step struct {
		offset  time.Duration
		wantRun map[string]bool
	}

	tests := []struct {
		name  string
		steps []step
	}{
		{
			name: "Happy path: a queue never run is due on the first pass",
			steps: []step{
				{offset: 0, wantRun: map[string]bool{"fast": true, "slow": true}},
			},
		},
		{
			name: "Edge: a pass before the interval elapses skips the queue",
			steps: []step{
				{offset: 0, wantRun: map[string]bool{"fast": true, "slow": true}},
				{offset: 10*time.Second - time.Nanosecond, wantRun: map[string]bool{"fast": false, "slow": false}},
			},
		},
		{
			name: "Edge: a pass exactly at the interval runs the queue",
			steps: []step{
				{offset: 0, wantRun: map[string]bool{"fast": true, "slow": true}},
				{offset: 10 * time.Second, wantRun: map[string]bool{"fast": true, "slow": false}},
			},
		},
		{
			name: "Edge: the slow queue runs only once its own interval has elapsed",
			steps: []step{
				{offset: 0, wantRun: map[string]bool{"fast": true, "slow": true}},
				{offset: 10 * time.Second, wantRun: map[string]bool{"fast": true, "slow": false}},
				{offset: 5 * time.Minute, wantRun: map[string]bool{"fast": true, "slow": true}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fast := &fakeStore{}
			slow := &fakeStore{}
			obs := sweepermocks.NewMockObserver(t)
			obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Maybe()
			sw, err := sweeper.New([]sweeper.Queue{
				mustQueue(t, sweeper.QueueSpec[string]{Name: "fast", Interval: 10 * time.Second, AttemptCap: 1, Batch: 1, Store: fast, Handle: okHandler}),
				mustQueue(t, sweeper.QueueSpec[string]{Name: "slow", Interval: 5 * time.Minute, AttemptCap: 1, Batch: 1, Store: slow, Handle: okHandler}),
			}, "relay-test", obs, testLogger())
			require.NoError(t, err)

			for _, st := range tc.steps {
				fastBefore, slowBefore := len(fast.claims), len(slow.claims)
				require.NoError(t, sw.RunOnce(context.Background(), fixedNow.Add(st.offset)))

				assert.Equal(t, st.wantRun["fast"], len(fast.claims) > fastBefore, "fast queue at offset %s", st.offset)
				assert.Equal(t, st.wantRun["slow"], len(slow.claims) > slowBefore, "slow queue at offset %s", st.offset)
			}
		})
	}
}

func TestRunCancelledContext(t *testing.T) {
	t.Run("Happy path: a cancelled context runs one pass and returns nil", func(t *testing.T) {
		store := &fakeStore{}
		obs := sweepermocks.NewMockObserver(t)
		obs.EXPECT().OldestPending(mock.Anything, mock.Anything).Once()
		sw := newSweeper(t, validSpec(store, okHandler), obs)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.NoError(t, sw.Run(ctx))
		assert.Len(t, store.claims, 1)
	})
}

func newSweeper(t *testing.T, spec sweeper.QueueSpec[string], obs sweeper.Observer) *sweeper.Sweeper {
	t.Helper()
	sw, err := sweeper.New([]sweeper.Queue{mustQueue(t, spec)}, "relay-test", obs, testLogger())
	require.NoError(t, err)
	return sw
}
