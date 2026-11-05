package worker

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProbeInterval = 60 * time.Second

// fakeClock is a manually advanced clock for the breaker's probe schedule.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// openBreaker feeds the breaker outages until it opens, and returns the
// transition of the last outage.
func openBreaker(t *testing.T, b *breaker, threshold int) breakerTransition {
	t.Helper()
	var last breakerTransition
	for i := 0; i < threshold; i++ {
		last = b.record(0, outcomeOutage)
	}
	return last
}

func TestBreaker_ClosedState(t *testing.T) {
	tests := []struct {
		name            string
		threshold       int
		outcomes        []breakerOutcome
		wantOpen        bool
		wantTransitions []breakerTransition
	}{
		{
			name:      "Happy path: a closed breaker admits deliveries",
			threshold: 5,
			outcomes:  nil,
			wantOpen:  false,
		},
		{
			name:      "Edge: four consecutive outages keep the breaker closed",
			threshold: 5,
			outcomes:  []breakerOutcome{outcomeOutage, outcomeOutage, outcomeOutage, outcomeOutage},
			wantOpen:  false,
			wantTransitions: []breakerTransition{
				transitionNone, transitionNone, transitionNone, transitionNone,
			},
		},
		{
			name:      "Edge: the fifth consecutive outage opens the breaker",
			threshold: 5,
			outcomes:  []breakerOutcome{outcomeOutage, outcomeOutage, outcomeOutage, outcomeOutage, outcomeOutage},
			wantOpen:  true,
			wantTransitions: []breakerTransition{
				transitionNone, transitionNone, transitionNone, transitionNone, transitionOpened,
			},
		},
		{
			name:      "Edge: a success between outages resets the run",
			threshold: 5,
			outcomes: []breakerOutcome{
				outcomeOutage, outcomeOutage, outcomeOutage, outcomeOutage,
				outcomeSuccess,
				outcomeOutage, outcomeOutage, outcomeOutage, outcomeOutage,
			},
			wantOpen: false,
		},
		{
			name:      "Edge: neutral outcomes never count toward opening",
			threshold: 5,
			outcomes: []breakerOutcome{
				outcomeNeutral, outcomeNeutral, outcomeNeutral, outcomeNeutral,
				outcomeNeutral, outcomeNeutral, outcomeNeutral, outcomeNeutral,
			},
			wantOpen: false,
		},
		{
			name:      "Edge: neutral outcomes do not reset the outage run",
			threshold: 3,
			outcomes: []breakerOutcome{
				outcomeOutage, outcomeNeutral, outcomeOutage, outcomeNeutral, outcomeOutage,
			},
			wantOpen: true,
		},
		{
			name:      "Edge: a threshold of one opens on the first outage",
			threshold: 1,
			outcomes:  []breakerOutcome{outcomeOutage},
			wantOpen:  true,
			wantTransitions: []breakerTransition{
				transitionOpened,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newBreaker(tc.threshold, testProbeInterval, newFakeClock().Now)
			for i, outcome := range tc.outcomes {
				transition := b.record(0, outcome)
				if tc.wantTransitions != nil {
					assert.Equal(t, tc.wantTransitions[i], transition, "outcome %d", i)
				}
			}
			assert.Equal(t, tc.wantOpen, b.state == breakerOpen)
		})
	}
}

func TestBreaker_OpenStateSchedule(t *testing.T) {
	tests := []struct {
		name         string
		elapsed      time.Duration
		wantProceed  bool
		wantWait     time.Duration
		wantStateNow breakerState
	}{
		{
			name:         "Edge: right after opening the wait is the full interval",
			elapsed:      0,
			wantProceed:  false,
			wantWait:     testProbeInterval,
			wantStateNow: breakerOpen,
		},
		{
			name:         "Edge: one nanosecond before the interval still waits",
			elapsed:      testProbeInterval - time.Nanosecond,
			wantProceed:  false,
			wantWait:     time.Nanosecond,
			wantStateNow: breakerOpen,
		},
		{
			name:         "Happy path: exactly the interval admits one probe",
			elapsed:      testProbeInterval,
			wantProceed:  true,
			wantStateNow: breakerHalfOpen,
		},
		{
			name:         "Edge: long after opening still admits a single probe",
			elapsed:      10 * testProbeInterval,
			wantProceed:  true,
			wantStateNow: breakerHalfOpen,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := newBreaker(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(tc.elapsed)

			decision := b.admit()

			assert.Equal(t, tc.wantProceed, decision.proceed)
			if !tc.wantProceed {
				assert.Equal(t, tc.wantWait, decision.wait)
				assert.Zero(t, decision.probe)
			} else {
				assert.NotZero(t, decision.probe)
			}
			assert.Equal(t, tc.wantStateNow, b.state)
		})
	}
}

func TestBreaker_HalfOpenAdmitsOneProbeAtATime(t *testing.T) {
	clock := newFakeClock()
	b := newBreaker(5, testProbeInterval, clock.Now)
	openBreaker(t, b, 5)
	clock.Advance(testProbeInterval)

	probe := b.admit()
	require.True(t, probe.proceed)

	tests := []struct {
		name string
	}{
		{name: "Edge: a second admission waits while the probe is in flight"},
		{name: "Edge: a third admission also waits while the probe is in flight"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := b.admit()
			assert.False(t, decision.proceed)
			assert.Equal(t, breakerPollInterval, decision.wait)
			assert.Zero(t, decision.probe)
		})
	}
}

func TestBreaker_ProbeOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		probeOutcome   breakerOutcome
		wantTransition breakerTransition
		wantState      breakerState
		wantReprobe    bool
	}{
		{
			name:           "Happy path: a successful probe closes the breaker",
			probeOutcome:   outcomeSuccess,
			wantTransition: transitionClosed,
			wantState:      breakerClosed,
		},
		{
			name:           "Edge: an outage on the probe reopens and restarts the interval",
			probeOutcome:   outcomeOutage,
			wantTransition: transitionNone,
			wantState:      breakerOpen,
		},
		{
			name:           "Edge: a neutral probe result reopens instead of closing",
			probeOutcome:   outcomeNeutral,
			wantTransition: transitionNone,
			wantState:      breakerOpen,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := newBreaker(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(testProbeInterval)
			probe := b.admit().probe
			require.NotZero(t, probe)

			transition := b.record(probe, tc.probeOutcome)

			assert.Equal(t, tc.wantTransition, transition)
			assert.Equal(t, tc.wantState, b.state)
			if tc.wantState == breakerOpen {
				decision := b.admit()
				assert.False(t, decision.proceed)
				assert.Equal(t, testProbeInterval, decision.wait)
			}
		})
	}
}

func TestBreaker_ClosedAfterProbeCountsFromZero(t *testing.T) {
	clock := newFakeClock()
	b := newBreaker(3, testProbeInterval, clock.Now)
	openBreaker(t, b, 3)
	clock.Advance(testProbeInterval)
	probe := b.admit().probe
	require.Equal(t, transitionClosed, b.record(probe, outcomeSuccess))

	assert.Equal(t, transitionNone, b.record(0, outcomeOutage))
	assert.Equal(t, transitionNone, b.record(0, outcomeOutage))
	assert.Equal(t, transitionOpened, b.record(0, outcomeOutage))
}

func TestBreaker_NonProbeOutcomesWhileOpenAreIgnored(t *testing.T) {
	tests := []struct {
		name    string
		outcome breakerOutcome
	}{
		{name: "Edge: an in-flight success does not close an open breaker", outcome: outcomeSuccess},
		{name: "Edge: an in-flight outage does not reopen or re-count", outcome: outcomeOutage},
		{name: "Edge: an in-flight neutral leaves the breaker open", outcome: outcomeNeutral},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := newBreaker(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			openedAt := b.openedAt

			assert.Equal(t, transitionNone, b.record(0, tc.outcome))

			assert.Equal(t, breakerOpen, b.state)
			assert.Equal(t, openedAt, b.openedAt)
		})
	}
}

func TestBreaker_StaleProbeCannotSettle(t *testing.T) {
	tests := []struct {
		name    string
		staleBy func(b *breaker, probe uint64)
	}{
		{
			name: "Edge: a probe settled by an outage cannot close the breaker later",
			staleBy: func(b *breaker, probe uint64) {
				b.record(probe, outcomeOutage)
			},
		},
		{
			name: "Edge: a probe released before its result cannot close the breaker later",
			staleBy: func(b *breaker, probe uint64) {
				b.release(probe)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := newBreaker(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(testProbeInterval)
			stale := b.admit().probe
			tc.staleBy(b, stale)

			clock.Advance(testProbeInterval)
			fresh := b.admit().probe
			require.NotZero(t, fresh)
			require.NotEqual(t, stale, fresh)

			assert.Equal(t, transitionNone, b.record(stale, outcomeSuccess))
			assert.Equal(t, breakerHalfOpen, b.state)
			assert.Equal(t, transitionClosed, b.record(fresh, outcomeSuccess))
		})
	}
}

func TestBreaker_Release(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(b *breaker, clock *fakeClock) uint64
		wantState breakerState
	}{
		{
			name: "Edge: releasing the active probe reopens for another interval",
			setup: func(b *breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				return b.admit().probe
			},
			wantState: breakerOpen,
		},
		{
			name: "Edge: releasing the zero token of an ordinary delivery is a no-op",
			setup: func(b *breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				b.admit()
				return 0
			},
			wantState: breakerHalfOpen,
		},
		{
			name: "Edge: releasing a zero token on an open breaker is a no-op",
			setup: func(b *breaker, clock *fakeClock) uint64 {
				return 0
			},
			wantState: breakerOpen,
		},
		{
			name: "Edge: releasing an already-settled probe is a no-op",
			setup: func(b *breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				probe := b.admit().probe
				b.record(probe, outcomeSuccess)
				return probe
			},
			wantState: breakerClosed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := newBreaker(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			probe := tc.setup(b, clock)

			b.release(probe)

			assert.Equal(t, tc.wantState, b.state)
		})
	}
}

func TestBreaker_ConcurrentOutagesOpenExactlyOnce(t *testing.T) {
	const goroutines = 50
	clock := newFakeClock()
	b := newBreaker(5, testProbeInterval, clock.Now)

	var wg sync.WaitGroup
	var mu sync.Mutex
	openings := 0
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.record(0, outcomeOutage) == transitionOpened {
				mu.Lock()
				openings++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, openings)
	assert.Equal(t, breakerOpen, b.state)
}

func TestClassifyOutcome(t *testing.T) {
	transient := fmt.Errorf("%w: 503", email.ErrTransient)
	tests := []struct {
		name string
		err  error
		want breakerOutcome
	}{
		{name: "Happy path: a delivered send is a success", err: nil, want: outcomeSuccess},
		{name: "Happy path: a transient 5xx or timeout is an outage", err: transient, want: outcomeOutage},
		{
			name: "Edge: a daily or monthly quota is never an outage",
			err:  fmt.Errorf("%w: %w", email.ErrQuotaExhausted, email.ErrTransient),
			want: outcomeNeutral,
		},
		{
			name: "Edge: a per-second rate limit is never an outage",
			err:  fmt.Errorf("%w: %w", email.ErrRateLimited, email.ErrTransient),
			want: outcomeNeutral,
		},
		{
			name: "Edge: a permanent validation failure is neutral",
			err:  fmt.Errorf("%w: 422", email.ErrPermanent),
			want: outcomeNeutral,
		},
		{
			name: "Edge: an unclassified error is neutral, not an outage",
			err:  errors.New("unclassified"),
			want: outcomeNeutral,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyOutcome(tc.err))
		})
	}
}
