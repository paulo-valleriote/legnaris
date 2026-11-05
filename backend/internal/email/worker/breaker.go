package worker

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

// breakerOutcome is the classification of one provider send for the breaker.
type breakerOutcome int

const (
	// outcomeSuccess is a send the provider accepted.
	outcomeSuccess breakerOutcome = iota
	// outcomeOutage is a failure that counts toward opening the breaker: a 5xx
	// or a timeout. Quota and rate limits never count (LGN-156).
	outcomeOutage
	// outcomeNeutral is any other result. It proves nothing about the provider
	// being down, so it neither opens nor closes the breaker.
	outcomeNeutral
)

// breakerTransition reports a change of breaker state caused by one outcome.
type breakerTransition int

const (
	transitionNone breakerTransition = iota
	transitionOpened
	transitionClosed
)

// admission is the breaker's answer to a worker that wants to take a delivery.
// When proceed is false, the worker waits wait before asking again. A probe
// admission carries the probe token that its outcome must present.
type admission struct {
	proceed bool
	probe   uint64
	wait    time.Duration
}

// breaker is the circuit breaker that pauses consumption while the provider is
// failing. It is safe for concurrent use; the clock is injected so the probe
// schedule is testable without sleeping.
type breaker struct {
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

func newBreaker(threshold int, probeInterval time.Duration, now func() time.Time) *breaker {
	return &breaker{threshold: threshold, probeInterval: probeInterval, now: now}
}

// admit decides whether a worker may take one delivery now.
func (b *breaker) admit() admission {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		return admission{proceed: true}
	case breakerHalfOpen:
		return admission{wait: breakerPollInterval}
	}

	elapsed := b.now().Sub(b.openedAt)
	if elapsed < b.probeInterval {
		return admission{wait: b.probeInterval - elapsed}
	}
	b.state = breakerHalfOpen
	b.probeSeq++
	b.activeProbe = b.probeSeq
	return admission{proceed: true, probe: b.probeSeq}
}

// record applies the outcome of one send. Only the probe's outcome can change an
// open or half-open breaker; a delivery admitted before the breaker opened
// reports into the void.
func (b *breaker) record(probe uint64, outcome breakerOutcome) breakerTransition {
	b.mu.Lock()
	defer b.mu.Unlock()

	if probe != 0 {
		return b.recordProbe(probe, outcome)
	}
	if b.state != breakerClosed {
		return transitionNone
	}
	switch outcome {
	case outcomeSuccess:
		b.failures = 0
	case outcomeOutage:
		b.failures++
		if b.failures >= b.threshold {
			b.open()
			return transitionOpened
		}
	}
	return transitionNone
}

func (b *breaker) recordProbe(probe uint64, outcome breakerOutcome) breakerTransition {
	if b.state != breakerHalfOpen || probe != b.activeProbe {
		return transitionNone
	}
	if outcome == outcomeSuccess {
		b.state = breakerClosed
		b.failures = 0
		return transitionClosed
	}
	b.open()
	return transitionNone
}

// release gives up a probe that never reached the provider, so the next probe
// can be admitted after another interval. It is a no-op for a probe already
// settled by record, and for the zero token of an ordinary delivery.
func (b *breaker) release(probe uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if probe != 0 && b.state == breakerHalfOpen && probe == b.activeProbe {
		b.open()
	}
}

func (b *breaker) open() {
	b.state = breakerOpen
	b.openedAt = b.now()
}
