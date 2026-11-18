package billing_test

import (
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/plan"

	"github.com/stretchr/testify/assert"
)

var deriveNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func compSub(end *time.Time) billing.Subscription {
	return billing.Subscription{
		Provider:         billing.ProviderComp,
		Status:           billing.StatusActive,
		CurrentPeriodEnd: end,
	}
}

func TestDerivePlanID(t *testing.T) {
	before := deriveNow.Add(time.Hour)
	at := deriveNow
	after := deriveNow.Add(-time.Hour)

	tests := []struct {
		name string
		subs []billing.Subscription
		want string
	}{
		{name: "Edge: no subscriptions is free", subs: nil, want: plan.IDFree},
		{name: "Edge: only pending is free", subs: []billing.Subscription{{Status: billing.StatusPending}}, want: plan.IDFree},
		{name: "Edge: past_due is free", subs: []billing.Subscription{{Status: billing.StatusPastDue}}, want: plan.IDFree},
		{name: "Edge: only canceled is free", subs: []billing.Subscription{{Status: billing.StatusCanceled}}, want: plan.IDFree},
		{name: "Happy path: one active is paid", subs: []billing.Subscription{{Status: billing.StatusActive}}, want: plan.IDPaid},
		{name: "Happy path: an active beside a canceled one is paid", subs: []billing.Subscription{{Status: billing.StatusCanceled}, {Status: billing.StatusActive}}, want: plan.IDPaid},
		{name: "Edge: an active beside a pending one is paid", subs: []billing.Subscription{{Status: billing.StatusPending}, {Status: billing.StatusActive}}, want: plan.IDPaid},
		{name: "Happy path: an active comp before its expiry is paid", subs: []billing.Subscription{compSub(&before)}, want: plan.IDPaid},
		{name: "Edge: an active comp with no expiry is paid", subs: []billing.Subscription{compSub(nil)}, want: plan.IDPaid},
		{name: "Edge: an active comp expiring exactly now is free", subs: []billing.Subscription{compSub(&at)}, want: plan.IDFree},
		{name: "Edge: an expired comp is free", subs: []billing.Subscription{compSub(&after)}, want: plan.IDFree},
		{name: "Edge: a canceled comp is free", subs: []billing.Subscription{{Provider: billing.ProviderComp, Status: billing.StatusCanceled, CurrentPeriodEnd: &before}}, want: plan.IDFree},
		{name: "Happy path: an expired comp beside an active paid is paid", subs: []billing.Subscription{compSub(&after), {Status: billing.StatusActive}}, want: plan.IDPaid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, billing.DerivePlanID(tc.subs, deriveNow))
		})
	}
}
