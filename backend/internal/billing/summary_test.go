package billing_test

import (
	"context"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusReader_Summary(t *testing.T) {
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	period := base.AddDate(0, 1, 0)
	sub := func(id string, status billing.Status, updated time.Time) billing.Subscription {
		return billing.Subscription{ID: id, UserID: consumerUserID, Status: status, UpdatedAt: updated, CurrentPeriodEnd: &period}
	}

	tests := []struct {
		name         string
		subs         []billing.Subscription
		wantStatus   billing.Status
		wantIssue    bool
		wantEmptyEnd bool
	}{
		{name: "Edge: never subscribed has no status and no issue", subs: nil},
		{name: "Happy path: an active subscription is reported as active", subs: []billing.Subscription{sub("a", billing.StatusActive, base)}, wantStatus: billing.StatusActive},
		{name: "Happy path: a past_due subscription raises the payment issue", subs: []billing.Subscription{sub("a", billing.StatusPastDue, base)}, wantStatus: billing.StatusPastDue, wantIssue: true},
		{name: "Edge: an active one beats an older canceled one", subs: []billing.Subscription{sub("old", billing.StatusCanceled, base.Add(time.Hour)), sub("new", billing.StatusActive, base)}, wantStatus: billing.StatusActive},
		{name: "Edge: past_due beats pending", subs: []billing.Subscription{sub("p", billing.StatusPending, base.Add(time.Hour)), sub("d", billing.StatusPastDue, base)}, wantStatus: billing.StatusPastDue, wantIssue: true},
		{name: "Edge: only canceled reports the most recent canceled row", subs: []billing.Subscription{sub("c1", billing.StatusCanceled, base), sub("c2", billing.StatusCanceled, base.Add(time.Hour))}, wantStatus: billing.StatusCanceled},
		{name: "Edge: a pending checkout alone is reported as pending", subs: []billing.Subscription{sub("p", billing.StatusPending, base)}, wantStatus: billing.StatusPending},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ledger := billingtest.NewLedger()
			for _, s := range tc.subs {
				ledger.SeedSubscription(s)
			}
			got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantIssue, got.PaymentIssue)
			if tc.wantStatus == "" {
				assert.Nil(t, got.CurrentPeriodEnd)
			}
		})
	}

	t.Run("Happy path: the summary carries the period end and the cancellation flag", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		s := sub("a", billing.StatusActive, base)
		s.CancelAtPeriodEnd = true
		ledger.SeedSubscription(s)
		got, err := billing.NewStatusReader(ledger).Summary(context.Background(), consumerUserID)
		require.NoError(t, err)
		require.NotNil(t, got.CurrentPeriodEnd)
		assert.True(t, got.CurrentPeriodEnd.Equal(period))
		assert.True(t, got.CancelAtPeriodEnd)
	})
}
