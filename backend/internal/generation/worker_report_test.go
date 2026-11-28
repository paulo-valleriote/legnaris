package generation

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/llm"
	"legnaris/backend/internal/platform/breaker"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reportClock struct {
	now time.Time
}

func (c *reportClock) Now() time.Time {
	return c.now
}

type reportObserver struct {
	states []bool
	opened int
}

func (o *reportObserver) GenerationAttempt(string, string, time.Duration, llm.Usage, string) {}

func (o *reportObserver) GenerationInvalidOutput(string, string) {}

func (o *reportObserver) GenerationSchemaRetry(string, string) {}

func (o *reportObserver) GenerationBreakerState(open bool) {
	o.states = append(o.states, open)
}

func (o *reportObserver) GenerationBreakerOpened() {
	o.opened++
}

func newReportWorker(clock *reportClock, observer *reportObserver) (*Worker, *breaker.Breaker) {
	b := breaker.New(1, time.Minute, clock.Now)
	return &Worker{
		breaker:  b,
		logger:   logger.NewLogger(slog.LevelError),
		observer: observer,
	}, b
}

func TestWorkerReportCountsFailedProbeAsOpening(t *testing.T) {
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: a failed probe reports the reopening again", func(t *testing.T) {
		clock := &reportClock{now: now}
		observer := &reportObserver{}
		w, b := newReportWorker(clock, observer)
		require.Equal(t, breaker.TransitionOpened, b.Record(0, breaker.OutcomeOutage))

		clock.now = clock.now.Add(time.Minute)
		probe := b.Admit()
		require.True(t, probe.Proceed)
		require.NotZero(t, probe.Probe)

		w.report(context.Background(), probe.Probe, breaker.OutcomeOutage)

		assert.True(t, b.IsOpen())
		assert.Equal(t, []bool{true}, observer.states)
		assert.Equal(t, 1, observer.opened)
	})

	t.Run("Edge: a successful probe reports the closing without an opening", func(t *testing.T) {
		clock := &reportClock{now: now}
		observer := &reportObserver{}
		w, b := newReportWorker(clock, observer)
		require.Equal(t, breaker.TransitionOpened, b.Record(0, breaker.OutcomeOutage))

		clock.now = clock.now.Add(time.Minute)
		probe := b.Admit()
		require.True(t, probe.Proceed)
		require.NotZero(t, probe.Probe)

		w.report(context.Background(), probe.Probe, breaker.OutcomeSuccess)

		assert.False(t, b.IsOpen())
		assert.Equal(t, []bool{false}, observer.states)
		assert.Zero(t, observer.opened)
	})
}
