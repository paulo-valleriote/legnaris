package metrics

import (
	"testing"

	"legnaris/backend/internal/funnel"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

func TestFunnelHomeCounters(t *testing.T) {
	tests := []struct {
		name      string
		visits    int
		ctaClicks int
		want      []string
	}{
		{
			name:      "Happy path: one visit and one hero click are exported",
			visits:    1,
			ctaClicks: 1,
			want: []string{
				"legnaris_funnel_home_visits_total 1",
				"legnaris_funnel_home_cta_clicks_total 1",
			},
		},
		{
			name:   "Happy path: repeated visits accumulate",
			visits: 3,
			want:   []string{"legnaris_funnel_home_visits_total 3"},
		},
		{
			name: "Edge: untouched counters export zero",
			want: []string{
				"legnaris_funnel_home_visits_total 0",
				"legnaris_funnel_home_cta_clicks_total 0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			m := New()
			m.Register(e)
			for range tc.visits {
				m.HomeVisited()
			}
			for range tc.ctaClicks {
				m.HomeCTAClicked()
			}

			body := scrapeMetrics(t, e)
			for _, want := range tc.want {
				assert.Containsf(t, body, want, "expected %q in /metrics output, got:\n%s", want, body)
			}
		})
	}
}

func TestFunnelStats(t *testing.T) {
	tests := []struct {
		name  string
		stats funnel.Stats
		want  []string
	}{
		{
			name:  "Happy path: accounts and the 24h milestone are exported with their ratio",
			stats: funnel.Stats{Accounts: 40, FirstSheetWithin24h: 10},
			want: []string{
				"legnaris_funnel_accounts 40",
				"legnaris_funnel_first_sheet_within_24h 10",
				"legnaris_funnel_first_sheet_within_24h_ratio 0.25",
			},
		},
		{
			name:  "Edge: no accounts reports a zero ratio, never a division by zero",
			stats: funnel.Stats{},
			want: []string{
				"legnaris_funnel_accounts 0",
				"legnaris_funnel_first_sheet_within_24h 0",
				"legnaris_funnel_first_sheet_within_24h_ratio 0",
			},
		},
		{
			name:  "Edge: every account reached the milestone",
			stats: funnel.Stats{Accounts: 4, FirstSheetWithin24h: 4},
			want:  []string{"legnaris_funnel_first_sheet_within_24h_ratio 1"},
		},
		{
			name:  "Edge: an inconsistent milestone count never reports a ratio above one",
			stats: funnel.Stats{Accounts: 2, FirstSheetWithin24h: 3},
			want:  []string{"legnaris_funnel_first_sheet_within_24h_ratio 1"},
		},
		{
			name:  "Edge: negative counts are clamped to zero",
			stats: funnel.Stats{Accounts: -2, FirstSheetWithin24h: -1},
			want: []string{
				"legnaris_funnel_accounts 0",
				"legnaris_funnel_first_sheet_within_24h 0",
				"legnaris_funnel_first_sheet_within_24h_ratio 0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			m := New()
			m.Register(e)
			m.FunnelStats(tc.stats)

			body := scrapeMetrics(t, e)
			for _, want := range tc.want {
				assert.Containsf(t, body, want, "expected %q in /metrics output, got:\n%s", want, body)
			}
		})
	}
}
