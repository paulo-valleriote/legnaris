package billing_test

import (
	"testing"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
)

func TestDerivePlanID(t *testing.T) {
	tests := []struct {
		name     string
		statuses []billing.Status
		want     string
	}{
		{name: "Edge: no subscriptions is free", statuses: nil, want: plan.IDFree},
		{name: "Edge: only pending is free", statuses: []billing.Status{billing.StatusPending}, want: plan.IDFree},
		{name: "Edge: past_due is free", statuses: []billing.Status{billing.StatusPastDue}, want: plan.IDFree},
		{name: "Edge: only canceled is free", statuses: []billing.Status{billing.StatusCanceled}, want: plan.IDFree},
		{name: "Happy path: one active is paid", statuses: []billing.Status{billing.StatusActive}, want: plan.IDPaid},
		{name: "Happy path: an active beside a canceled one is paid", statuses: []billing.Status{billing.StatusCanceled, billing.StatusActive}, want: plan.IDPaid},
		{name: "Edge: an active beside a pending one is paid", statuses: []billing.Status{billing.StatusPending, billing.StatusActive}, want: plan.IDPaid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subs := make([]billing.Subscription, 0, len(tc.statuses))
			for _, s := range tc.statuses {
				subs = append(subs, billing.Subscription{Status: s})
			}
			assert.Equal(t, tc.want, billing.DerivePlanID(subs))
		})
	}
}
