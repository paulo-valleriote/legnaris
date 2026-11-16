package metrics

import (
	"strings"
	"testing"

	"legnaris/backend/internal/billing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
)

func TestBillingReconcileCorrectionMetrics(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		count   int
		want    string
		notWant string
	}{
		{
			name:  "Happy path: a subscription correction is exported under its kind",
			kind:  billing.CorrectionSubscription,
			count: 1,
			want:  `legnaris_billing_reconcile_corrections_total{kind="subscription"} 1`,
		},
		{
			name:  "Edge: a batch of requeued events adds its full count",
			kind:  billing.CorrectionEvent,
			count: 3,
			want:  `legnaris_billing_reconcile_corrections_total{kind="event"} 3`,
		},
		{
			name:    "Edge: a zero count never creates a series",
			kind:    billing.CorrectionPlan,
			count:   0,
			notWant: `kind="plan"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			m := New()
			m.Register(e)
			m.Corrected(tc.kind, tc.count)

			body := scrapeMetrics(t, e)
			if tc.want != "" {
				assert.Truef(t, strings.Contains(body, tc.want), "expected %q in /metrics output, got:\n%s", tc.want, body)
			}
			if tc.notWant != "" {
				assert.NotContains(t, body, tc.notWant)
			}
		})
	}
}
