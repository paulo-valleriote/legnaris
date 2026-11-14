package breaker

import (
	"sync"
	"testing"
	"time"

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
func openBreaker(t *testing.T, b *Breaker, threshold int) Transition {
	t.Helper()
	var last Transition
	for i := 0; i < threshold; i++ {
		last = b.Record(0, OutcomeOutage)
	}
	return last
}

func TestBreaker_ClosedState(t *testing.T) {
	tests := []struct {
		name            string
		threshold       int
		outcomes        []Outcome
		wantOpen        bool
		wantTransitions []Transition
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
			outcomes:  []Outcome{OutcomeOutage, OutcomeOutage, OutcomeOutage, OutcomeOutage},
			wantOpen:  false,
			wantTransitions: []Transition{
				TransitionNone, TransitionNone, TransitionNone, TransitionNone,
			},
		},
		{
			name:      "Edge: the fifth consecutive outage opens the breaker",
			threshold: 5,
			outcomes:  []Outcome{OutcomeOutage, OutcomeOutage, OutcomeOutage, OutcomeOutage, OutcomeOutage},
			wantOpen:  true,
			wantTransitions: []Transition{
				TransitionNone, TransitionNone, TransitionNone, TransitionNone, TransitionOpened,
			},
		},
		{
			name:      "Edge: a success between outages resets the run",
			threshold: 5,
			outcomes: []Outcome{
				OutcomeOutage, OutcomeOutage, OutcomeOutage, OutcomeOutage,
				OutcomeSuccess,
				OutcomeOutage, OutcomeOutage, OutcomeOutage, OutcomeOutage,
			},
			wantOpen: false,
		},
		{
			name:      "Edge: neutral outcomes never count toward opening",
			threshold: 5,
			outcomes: []Outcome{
				OutcomeNeutral, OutcomeNeutral, OutcomeNeutral, OutcomeNeutral,
				OutcomeNeutral, OutcomeNeutral, OutcomeNeutral, OutcomeNeutral,
			},
			wantOpen: false,
		},
		{
			name:      "Edge: neutral outcomes do not reset the outage run",
			threshold: 3,
			outcomes: []Outcome{
				OutcomeOutage, OutcomeNeutral, OutcomeOutage, OutcomeNeutral, OutcomeOutage,
			},
			wantOpen: true,
		},
		{
			name:      "Edge: a threshold of one opens on the first outage",
			threshold: 1,
			outcomes:  []Outcome{OutcomeOutage},
			wantOpen:  true,
			wantTransitions: []Transition{
				TransitionOpened,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := New(tc.threshold, testProbeInterval, newFakeClock().Now)
			for i, outcome := range tc.outcomes {
				transition := b.Record(0, outcome)
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
			b := New(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(tc.elapsed)

			decision := b.Admit()

			assert.Equal(t, tc.wantProceed, decision.Proceed)
			if !tc.wantProceed {
				assert.Equal(t, tc.wantWait, decision.Wait)
				assert.Zero(t, decision.Probe)
			} else {
				assert.NotZero(t, decision.Probe)
			}
			assert.Equal(t, tc.wantStateNow, b.state)
		})
	}
}

func TestBreaker_HalfOpenAdmitsOneProbeAtATime(t *testing.T) {
	clock := newFakeClock()
	b := New(5, testProbeInterval, clock.Now)
	openBreaker(t, b, 5)
	clock.Advance(testProbeInterval)

	probe := b.Admit()
	require.True(t, probe.Proceed)

	tests := []struct {
		name string
	}{
		{name: "Edge: a second Admission waits while the probe is in flight"},
		{name: "Edge: a third Admission also waits while the probe is in flight"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := b.Admit()
			assert.False(t, decision.Proceed)
			assert.Equal(t, breakerPollInterval, decision.Wait)
			assert.Zero(t, decision.Probe)
		})
	}
}

func TestBreaker_ProbeOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		probeOutcome   Outcome
		wantTransition Transition
		wantState      breakerState
		wantReprobe    bool
	}{
		{
			name:           "Happy path: a successful probe closes the breaker",
			probeOutcome:   OutcomeSuccess,
			wantTransition: TransitionClosed,
			wantState:      breakerClosed,
		},
		{
			name:           "Edge: an outage on the probe reopens and restarts the interval",
			probeOutcome:   OutcomeOutage,
			wantTransition: TransitionNone,
			wantState:      breakerOpen,
		},
		{
			name:           "Edge: a neutral probe result reopens instead of closing",
			probeOutcome:   OutcomeNeutral,
			wantTransition: TransitionNone,
			wantState:      breakerOpen,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := New(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(testProbeInterval)
			probe := b.Admit().Probe
			require.NotZero(t, probe)

			transition := b.Record(probe, tc.probeOutcome)

			assert.Equal(t, tc.wantTransition, transition)
			assert.Equal(t, tc.wantState, b.state)
			if tc.wantState == breakerOpen {
				decision := b.Admit()
				assert.False(t, decision.Proceed)
				assert.Equal(t, testProbeInterval, decision.Wait)
			}
		})
	}
}

func TestBreaker_ClosedAfterProbeCountsFromZero(t *testing.T) {
	clock := newFakeClock()
	b := New(3, testProbeInterval, clock.Now)
	openBreaker(t, b, 3)
	clock.Advance(testProbeInterval)
	probe := b.Admit().Probe
	require.Equal(t, TransitionClosed, b.Record(probe, OutcomeSuccess))

	assert.Equal(t, TransitionNone, b.Record(0, OutcomeOutage))
	assert.Equal(t, TransitionNone, b.Record(0, OutcomeOutage))
	assert.Equal(t, TransitionOpened, b.Record(0, OutcomeOutage))
}

func TestBreaker_NonProbeOutcomesWhileOpenAreIgnored(t *testing.T) {
	tests := []struct {
		name    string
		outcome Outcome
	}{
		{name: "Edge: an in-flight success does not close an open breaker", outcome: OutcomeSuccess},
		{name: "Edge: an in-flight outage does not reopen or re-count", outcome: OutcomeOutage},
		{name: "Edge: an in-flight neutral leaves the breaker open", outcome: OutcomeNeutral},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := New(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			openedAt := b.openedAt

			assert.Equal(t, TransitionNone, b.Record(0, tc.outcome))

			assert.Equal(t, breakerOpen, b.state)
			assert.Equal(t, openedAt, b.openedAt)
		})
	}
}

func TestBreaker_StaleProbeCannotSettle(t *testing.T) {
	tests := []struct {
		name    string
		staleBy func(b *Breaker, probe uint64)
	}{
		{
			name: "Edge: a probe settled by an outage cannot close the breaker later",
			staleBy: func(b *Breaker, probe uint64) {
				b.Record(probe, OutcomeOutage)
			},
		},
		{
			name: "Edge: a probe released before its result cannot close the breaker later",
			staleBy: func(b *Breaker, probe uint64) {
				b.Release(probe)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := New(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			clock.Advance(testProbeInterval)
			stale := b.Admit().Probe
			tc.staleBy(b, stale)

			clock.Advance(testProbeInterval)
			fresh := b.Admit().Probe
			require.NotZero(t, fresh)
			require.NotEqual(t, stale, fresh)

			assert.Equal(t, TransitionNone, b.Record(stale, OutcomeSuccess))
			assert.Equal(t, breakerHalfOpen, b.state)
			assert.Equal(t, TransitionClosed, b.Record(fresh, OutcomeSuccess))
		})
	}
}

func TestBreaker_Release(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(b *Breaker, clock *fakeClock) uint64
		wantState breakerState
	}{
		{
			name: "Edge: releasing the active probe reopens for another interval",
			setup: func(b *Breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				return b.Admit().Probe
			},
			wantState: breakerOpen,
		},
		{
			name: "Edge: releasing the zero token of an ordinary delivery is a no-op",
			setup: func(b *Breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				b.Admit()
				return 0
			},
			wantState: breakerHalfOpen,
		},
		{
			name: "Edge: releasing a zero token on an open breaker is a no-op",
			setup: func(b *Breaker, clock *fakeClock) uint64 {
				return 0
			},
			wantState: breakerOpen,
		},
		{
			name: "Edge: releasing an already-settled probe is a no-op",
			setup: func(b *Breaker, clock *fakeClock) uint64 {
				clock.Advance(testProbeInterval)
				probe := b.Admit().Probe
				b.Record(probe, OutcomeSuccess)
				return probe
			},
			wantState: breakerClosed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			b := New(5, testProbeInterval, clock.Now)
			openBreaker(t, b, 5)
			probe := tc.setup(b, clock)

			b.Release(probe)

			assert.Equal(t, tc.wantState, b.state)
		})
	}
}

func TestBreaker_ConcurrentOutagesOpenExactlyOnce(t *testing.T) {
	const goroutines = 50
	clock := newFakeClock()
	b := New(5, testProbeInterval, clock.Now)

	var wg sync.WaitGroup
	var mu sync.Mutex
	openings := 0
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Record(0, OutcomeOutage) == TransitionOpened {
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
