package metrics

import (
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

func TestGenerationCapMetrics(t *testing.T) {
	tests := []struct {
		name  string
		used  int
		limit int
		want  []string
	}{
		{
			name:  "Happy path: used, limit and ratio are exported",
			used:  160,
			limit: 200,
			want: []string{
				"legnaris_generation_cap_used 160",
				"legnaris_generation_cap_limit 200",
				"legnaris_generation_cap_used_ratio 0.8",
			},
		},
		{
			name:  "Edge: a zero limit reports a zero ratio, never a division by zero",
			used:  0,
			limit: 0,
			want: []string{
				"legnaris_generation_cap_limit 0",
				"legnaris_generation_cap_used_ratio 0",
			},
		},
		{
			name:  "Edge: a spent cap reports a ratio of one",
			used:  200,
			limit: 200,
			want:  []string{"legnaris_generation_cap_used_ratio 1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			m := New()
			m.Register(e)
			m.GenerationCapUsage(tc.used, tc.limit)

			body := scrapeMetrics(t, e)
			for _, want := range tc.want {
				assert.Containsf(t, body, want, "expected %q in /metrics output, got:\n%s", want, body)
			}
		})
	}
}
