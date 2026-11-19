package billingtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
)

var compNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func compSub(id, user, provider string, status billing.Status, end *time.Time) billing.Subscription {
	return billing.Subscription{ID: id, UserID: user, PlanID: "paid", Provider: provider, Status: status, CurrentPeriodEnd: end}
}

func at(d time.Duration) *time.Time {
	v := compNow.Add(d)
	return &v
}

func seedComps(t *testing.T) *billingtest.Ledger {
	t.Helper()
	ledger := billingtest.NewLedger()
	ctx := context.Background()
	for _, sub := range []billing.Subscription{
		compSub("c-late", "u1", billing.ProviderComp, billing.StatusActive, at(-time.Hour)),
		compSub("c-early", "u2", billing.ProviderComp, billing.StatusActive, at(-48*time.Hour)),
		compSub("c-tie-b", "u3", billing.ProviderComp, billing.StatusActive, at(-2*time.Hour)),
		compSub("c-tie-a", "u4", billing.ProviderComp, billing.StatusActive, at(-2*time.Hour)),
		compSub("c-boundary", "u5", billing.ProviderComp, billing.StatusActive, at(0)),
		compSub("c-future", "u6", billing.ProviderComp, billing.StatusActive, at(time.Hour)),
		compSub("c-open", "u7", billing.ProviderComp, billing.StatusActive, nil),
		compSub("c-canceled", "u8", billing.ProviderComp, billing.StatusCanceled, at(-time.Hour)),
		compSub("s-paid", "u9", billing.ProviderStripe, billing.StatusActive, at(-time.Hour)),
	} {
		require.NoError(t, ledger.InsertSubscription(ctx, sub))
	}
	return ledger
}

func ids(subs []billing.Subscription) []string {
	out := make([]string, 0, len(subs))
	for _, sub := range subs {
		out = append(out, sub.ID)
	}
	return out
}

func TestLedgerExpiredComps(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{name: "Happy path: active comps at or past their end, oldest first, ties by id", limit: 0, want: []string{"c-early", "c-tie-a", "c-tie-b", "c-late", "c-boundary"}},
		{name: "Edge: the limit keeps the oldest", limit: 2, want: []string{"c-early", "c-tie-a"}},
		{name: "Edge: a negative limit is no limit", limit: -1, want: []string{"c-early", "c-tie-a", "c-tie-b", "c-late", "c-boundary"}},
		{name: "Edge: a limit above the count returns all", limit: 50, want: []string{"c-early", "c-tie-a", "c-tie-b", "c-late", "c-boundary"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := seedComps(t).ExpiredComps(context.Background(), compNow, tc.limit)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ids(got))
		})
	}

	t.Run("Edge: an empty ledger returns nothing", func(t *testing.T) {
		got, err := billingtest.NewLedger().ExpiredComps(context.Background(), compNow, 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestLedgerCancelComp(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "Happy path: an active comp is canceled", id: "c-late", want: true},
		{name: "Edge: an unknown id is a no-op", id: "missing", want: false},
		{name: "Edge: an already canceled comp is a no-op", id: "c-canceled", want: false},
		{name: "Edge: a paid subscription is never canceled as a comp", id: "s-paid", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ledger := seedComps(t)
			ok, err := ledger.CancelComp(context.Background(), tc.id, compNow)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
		})
	}

	t.Run("Edge: a second cancel is idempotent and the comp leaves the expired set", func(t *testing.T) {
		ledger := seedComps(t)
		first, err := ledger.CancelComp(context.Background(), "c-late", compNow)
		require.NoError(t, err)
		require.True(t, first)
		second, err := ledger.CancelComp(context.Background(), "c-late", compNow)
		require.NoError(t, err)
		assert.False(t, second)

		subs := ledger.Subscriptions("u1")
		require.Len(t, subs, 1)
		assert.Equal(t, billing.StatusCanceled, subs[0].Status)
		assert.Equal(t, compNow, subs[0].UpdatedAt)

		expired, err := ledger.ExpiredComps(context.Background(), compNow, 0)
		require.NoError(t, err)
		assert.NotContains(t, ids(expired), "c-late")
	})
}
