package metrics

import (
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

func TestSweeperMetrics(t *testing.T) {
	tests := []struct {
		name  string
		drive func(m *Metrics)
		want  []string
		avoid []string
	}{
		{
			name: "Happy path: the oldest age is exposed in seconds per queue",
			drive: func(m *Metrics) {
				m.OldestPending("event_outbox", 90*time.Second)
			},
			want: []string{`legnaris_sweeper_oldest_pending_seconds{queue="event_outbox"} 90`},
		},
		{
			name: "Edge: an empty queue reports a zero age",
			drive: func(m *Metrics) {
				m.OldestPending("event_outbox", 5*time.Minute)
				m.OldestPending("event_outbox", 0)
			},
			want: []string{`legnaris_sweeper_oldest_pending_seconds{queue="event_outbox"} 0`},
		},
		{
			name: "Edge: a sub-second age keeps its fraction",
			drive: func(m *Metrics) {
				m.OldestPending("event_outbox", 1500*time.Millisecond)
			},
			want: []string{`legnaris_sweeper_oldest_pending_seconds{queue="event_outbox"} 1.5`},
		},
		{
			name: "Happy path: failures are counted per queue",
			drive: func(m *Metrics) {
				m.Failed("event_outbox")
				m.Failed("event_outbox")
				m.Failed("generation")
			},
			want: []string{
				`legnaris_sweeper_failed_total{queue="event_outbox"} 2`,
				`legnaris_sweeper_failed_total{queue="generation"} 1`,
			},
		},
		{
			name: "Edge: an untouched queue exposes no failure series",
			drive: func(m *Metrics) {
				m.OldestPending("event_outbox", 0)
			},
			avoid: []string{`legnaris_sweeper_failed_total{queue="event_outbox"}`},
		},
		{
			name: "Edge: each queue keeps its own age",
			drive: func(m *Metrics) {
				m.OldestPending("event_outbox", time.Minute)
				m.OldestPending("generation", 10*time.Second)
			},
			want: []string{
				`legnaris_sweeper_oldest_pending_seconds{queue="event_outbox"} 60`,
				`legnaris_sweeper_oldest_pending_seconds{queue="generation"} 10`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			m := New()
			m.Register(e)
			tc.drive(m)

			body := scrapeMetrics(t, e)
			for _, want := range tc.want {
				assert.Containsf(t, body, want, "expected %q in /metrics output, got:\n%s", want, body)
			}
			for _, avoid := range tc.avoid {
				assert.NotContainsf(t, body, avoid, "did not expect %q in /metrics output", avoid)
			}
		})
	}
}
