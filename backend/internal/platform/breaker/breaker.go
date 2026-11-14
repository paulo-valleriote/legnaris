// Package breaker is the circuit breaker shared by the workers that call an external
// provider. It pauses or fails fast while a provider is down and probes it again after
// an interval, so one failing dependency never holds the other consumers hostage.
package breaker

import (
	"sync"
	"time"
)

// Default breaker settings; Config fills any zero value from them (LGN-149).
const (
	// DefaultBreakerThreshold is the run of consecutive provider outages that
	// opens the breaker.
	DefaultBreakerThreshold = 5
	// DefaultBreakerProbeInterval is how long an open breaker waits before it
	// lets one delivery through to test the provider.
	DefaultBreakerProbeInterval = 60 * time.Second

	// breakerPollInterval is how often a worker waiting on an in-flight probe
	// re-checks the breaker. It is short because the probe's outcome decides
	// the next state and the waiting worker must not stall after it.
	breakerPollInterval = 50 * time.Millisecond
)

// breakerState is the phase of the provider circuit breaker.
type breakerState int

const (
	// breakerClosed lets every delivery through.
	breakerClosed breakerState = iota
	// breakerOpen stops consumption until the probe interval has elapsed.
	breakerOpen
	// breakerHalfOpen has one probe delivery in flight; its outcome decides
	// whether the breaker closes or opens again.
	breakerHalfOpen
)

// Outcome is the classification of one provider send for the breaker.
type Outcome int

const (
	// OutcomeSuccess is a send the provider accepted.
	OutcomeSuccess Outcome = iota
	// OutcomeOutage is a failure that counts toward opening the breaker: a 5xx
	// or a timeout. Quota and rate limits never count (LGN-156).
	OutcomeOutage
	// OutcomeNeutral is any other result. Outside a probe it neither counts toward
	// opening a closed breaker nor resets the outage run. As a probe outcome it
	// is not a recovery: the provider has not proven itself healthy, so the
	// breaker reopens and the next probe waits a full probe interval.
	OutcomeNeutral
)

// Transition reports a change of breaker state caused by one outcome.
type Transition int

const (
	TransitionNone Transition = iota
	TransitionOpened
	TransitionClosed
)

// Admission is the breaker's answer to a worker that wants to take a delivery.
// When Proceed is false, the worker waits wait before asking again. A probe
// Admission carries the probe token that its outcome must present.
type Admission struct {
	Proceed bool
	Probe   uint64
	Wait    time.Duration
}

// Breaker is the circuit breaker that pauses consumption while the provider is
// failing. It is safe for concurrent use; the clock is injected so the probe
// schedule is testable without sleeping.
type Breaker struct {
	mu            sync.Mutex
	threshold     int
	probeInterval time.Duration
	now           func() time.Time

	state       breakerState
	failures    int
	openedAt    time.Time
	probeSeq    uint64
	activeProbe uint64
}

// New returns a Breaker that opens after threshold consecutive outages and admits a
// probe once probeInterval has passed since it opened.
func New(threshold int, probeInterval time.Duration, now func() time.Time) *Breaker {
	return &Breaker{threshold: threshold, probeInterval: probeInterval, now: now}
}

// Admit decides whether a worker may take one delivery now.
func (b *Breaker) Admit() Admission {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		return Admission{Proceed: true}
	case breakerHalfOpen:
		return Admission{Wait: breakerPollInterval}
	}

	elapsed := b.now().Sub(b.openedAt)
	if elapsed < b.probeInterval {
		return Admission{Wait: b.probeInterval - elapsed}
	}
	b.state = breakerHalfOpen
	b.probeSeq++
	b.activeProbe = b.probeSeq
	return Admission{Proceed: true, Probe: b.probeSeq}
}

// Record applies the outcome of one send and reports the resulting transition.
// A probe token routes the outcome to the half-open probe: only OutcomeSuccess
// closes the breaker, while any other outcome reopens it. A zero token is an
// ordinary delivery: it can only open a closed breaker, through consecutive
// outages, and a delivery admitted before the breaker opened reports into the void.
func (b *Breaker) Record(probe uint64, outcome Outcome) Transition {
	b.mu.Lock()
	defer b.mu.Unlock()

	if probe != 0 {
		return b.recordProbe(probe, outcome)
	}
	if b.state != breakerClosed {
		return TransitionNone
	}
	switch outcome {
	case OutcomeSuccess:
		b.failures = 0
	case OutcomeOutage:
		b.failures++
		if b.failures >= b.threshold {
			b.open()
			return TransitionOpened
		}
	}
	return TransitionNone
}

func (b *Breaker) recordProbe(probe uint64, outcome Outcome) Transition {
	if b.state != breakerHalfOpen || probe != b.activeProbe {
		return TransitionNone
	}
	if outcome == OutcomeSuccess {
		b.state = breakerClosed
		b.failures = 0
		return TransitionClosed
	}
	b.open()
	return TransitionNone
}

// Release gives up a probe that never reached the provider, so the next probe
// can be admitted after another interval. It is a no-op for a probe already
// settled by record, and for the zero token of an ordinary delivery.
func (b *Breaker) Release(probe uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if probe != 0 && b.state == breakerHalfOpen && probe == b.activeProbe {
		b.open()
	}
}

func (b *Breaker) open() {
	b.state = breakerOpen
	b.openedAt = b.now()
}
