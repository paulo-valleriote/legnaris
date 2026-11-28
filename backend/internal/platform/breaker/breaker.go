package breaker

import (
	"sync"
	"time"
)

const (
	DefaultBreakerThreshold     = 5
	DefaultBreakerProbeInterval = 60 * time.Second

	breakerPollInterval = 50 * time.Millisecond
)

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

type Outcome int

const (
	OutcomeSuccess Outcome = iota
	OutcomeOutage
	OutcomeNeutral
)

type Transition int

const (
	TransitionNone Transition = iota
	TransitionOpened
	TransitionClosed
)

type Admission struct {
	Proceed bool
	Probe   uint64
	Wait    time.Duration
}

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

func New(threshold int, probeInterval time.Duration, now func() time.Time) *Breaker {
	return &Breaker{threshold: threshold, probeInterval: probeInterval, now: now}
}

func (b *Breaker) IsOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state == breakerOpen
}

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
