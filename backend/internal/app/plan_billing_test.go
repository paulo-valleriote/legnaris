package app

import (
	"encoding/json"
	"testing"
	"time"

	"legnaris/backend/internal/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyBillingSummary(t *testing.T) {
	end := time.Date(2026, 12, 16, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		summary billing.Summary
		want    map[string]any
	}{
		{
			name:    "Edge: a never-subscribed user gets none of the billing keys",
			summary: billing.Summary{},
			want:    map[string]any{},
		},
		{
			name:    "Happy path: an active subscription exposes status, provider, period end and cancellation",
			summary: billing.Summary{Provider: billing.ProviderStripe, Status: billing.StatusActive, CurrentPeriodEnd: &end, CancelAtPeriodEnd: true},
			want: map[string]any{
				"subscription_status":   "active",
				"subscription_provider": "stripe",
				"current_period_end":    "2026-12-16T10:00:00Z",
				"cancel_at_period_end":  true,
				"payment_issue":         false,
			},
		},
		{
			name:    "Happy path: a past_due subscription raises payment_issue",
			summary: billing.Summary{Status: billing.StatusPastDue, CurrentPeriodEnd: &end, PaymentIssue: true},
			want: map[string]any{
				"subscription_status":  "past_due",
				"current_period_end":   "2026-12-16T10:00:00Z",
				"cancel_at_period_end": false,
				"payment_issue":        true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			view := planView{Plan: "free"}
			applyBillingSummary(&view, tc.summary)
			raw, err := json.Marshal(view)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(raw, &got))
			for _, key := range []string{"subscription_status", "subscription_provider", "current_period_end", "cancel_at_period_end", "payment_issue"} {
				want, wanted := tc.want[key]
				value, present := got[key]
				if !wanted {
					assert.False(t, present, "key %s must be absent", key)
					continue
				}
				assert.True(t, present, "key %s must be present", key)
				assert.Equal(t, want, value, key)
			}
		})
	}
}
