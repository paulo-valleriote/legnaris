package billing_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	reconUserID     = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e20"
	reconOtherUser  = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e21"
	reconLocalSub   = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e22"
	reconOtherSub   = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e23"
	reconProviderID = "sub_recon_1"
	reconOtherPID   = "sub_recon_2"
	reconCustomer   = "cus_recon_1"
	reconClaimer    = "reconciler-test"
)

type recordingObserver struct {
	counts map[string]int
}

func (o *recordingObserver) Corrected(kind string, count int) {
	if o.counts == nil {
		o.counts = map[string]int{}
	}
	o.counts[kind] += count
}

type reconFixture struct {
	ledger   *billingtest.Ledger
	provider *billingtest.Provider
	observer *recordingObserver
	reconc   *billing.Reconciler
	clock    time.Time
	hook     db.StepHook
}

func newReconFixture(t *testing.T) *reconFixture {
	t.Helper()
	f := &reconFixture{
		ledger:   billingtest.NewLedger(),
		provider: billingtest.NewProvider(),
		observer: &recordingObserver{},
		clock:    t1,
	}
	periodEnd := t1.AddDate(0, 1, 0)
	f.ledger.AddAccount(reconUserID, billing.AccountPlan{Plan: "paid", PlanVersion: 1})
	f.ledger.SeedSubscription(billing.Subscription{
		ID:                 reconLocalSub,
		UserID:             reconUserID,
		PlanID:             "paid",
		Provider:           billing.ProviderStripe,
		ProviderCustomerID: reconCustomer,
		ProviderSubID:      reconProviderID,
		Status:             billing.StatusActive,
		CurrentPeriodEnd:   &periodEnd,
	})
	return f
}

func (f *reconFixture) build() {
	tx := billingtest.NewTransactor(f.ledger, f.hook)
	f.reconc = billing.NewReconciler(billing.ReconcileDeps{
		ConsumerDeps: billing.ConsumerDeps{
			Provider: f.provider,
			Ledger:   f.ledger,
			Tx:       tx,
			Outbox:   f.ledger.Outbox(),
			Now:      func() time.Time { return f.clock },
		},
		Claimer:  reconClaimer,
		Observer: f.observer,
		Log:      logger.NewLogger(slog.LevelError),
	})
}

func (f *reconFixture) providerSays(id string, status billing.Status) {
	f.provider.SetSubscription(billing.ProviderSubscription{
		ID:               id,
		CustomerID:       reconCustomer,
		Status:           status,
		CurrentPeriodEnd: t1.AddDate(0, 1, 0),
	})
}

func (f *reconFixture) seedEvent(t *testing.T, userID string, status billing.EventStatus, receivedAt, nextAt time.Time) string {
	t.Helper()
	id, err := idgen.New()
	require.NoError(t, err)
	require.NoError(t, f.ledger.InsertEvent(context.Background(), billing.Event{
		ID: id, Provider: billing.ProviderStripe, ProviderEventID: "evt_" + id,
		Kind: billing.KindSubscriptionChanged, UserID: userID, Status: status,
		Attempts: 3, NextAttemptAt: nextAt, ReceivedAt: receivedAt, ExpiresAt: receivedAt.Add(time.Hour),
	}))
	return id
}

func (f *reconFixture) planOf(t *testing.T, userID string) (string, int64) {
	t.Helper()
	acc, _ := f.ledger.Account(userID)
	return acc.Plan, acc.PlanVersion
}

func TestReconciler_RunOnce(t *testing.T) {
	t.Run("Happy path: a drifted stored subscription is corrected from the provider and counted", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, ok := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		require.True(t, ok)
		assert.Equal(t, billing.StatusCanceled, sub.Status)
		require.NotNil(t, sub.ProviderSyncedAt)
		assert.True(t, sub.ProviderSyncedAt.Equal(t1))

		plan, version := f.planOf(t, reconUserID)
		assert.Equal(t, "free", plan)
		assert.Equal(t, int64(2), version)
		assert.Equal(t, map[string]int{billing.CorrectionSubscription: 1, billing.CorrectionPlan: 1}, f.observer.counts)
		require.Len(t, f.ledger.OutboxEntries(), 1)
	})

	t.Run("Happy path: an in-sync subscription is re-read but corrects nothing", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusActive)
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		plan, version := f.planOf(t, reconUserID)
		assert.Equal(t, "paid", plan)
		assert.Equal(t, int64(1), version)
		assert.Empty(t, f.observer.counts)
		assert.Empty(t, f.ledger.OutboxEntries())
		assert.Equal(t, 1, f.provider.Reads())
	})

	t.Run("Edge: a second pass on the same provider read is a no-op", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.build()
		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))
		f.observer.counts = nil

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		assert.Empty(t, f.observer.counts)
		assert.Len(t, f.ledger.OutboxEntries(), 1)
	})

	t.Run("Edge: a stored subscription synced after the read is not regressed", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		synced := t2
		seeded, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		seeded.ProviderSyncedAt = &synced
		f.ledger.SeedSubscription(seeded)
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Edge: a deleting account is skipped without a provider read", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.ledger.AddAccount(reconUserID, billing.AccountPlan{Plan: "paid", Deleting: true})
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Equal(t, 0, f.provider.Reads())
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Edge: a user whose lease a consumer holds is skipped until the next pass", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.seedEvent(t, reconUserID, billing.EventReceived, t1, t1)
		claimed, err := f.ledger.ClaimEvents(context.Background(), "consumer", t1, time.Hour, 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Equal(t, 0, f.provider.Reads())
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Edge: no live subscriptions and no stuck events is a silent no-op", func(t *testing.T) {
		f := newReconFixture(t)
		f.ledger.RemoveAccount(reconUserID)
		f.build()

		require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

		assert.Equal(t, 0, f.provider.Reads())
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Edge: a crash after the provider read rolls the correction back", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.hook = failAt(db.StepAfterEntityWrite)
		f.build()

		require.Error(t, f.reconc.RunOnce(context.Background(), t1))

		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Empty(t, f.observer.counts)
		plan, _ := f.planOf(t, reconUserID)
		assert.Equal(t, "paid", plan)
	})

	t.Run("Error: provider unavailable leaves subscriptions untouched and reports the failure", func(t *testing.T) {
		f := newReconFixture(t)
		f.provider.Unavailable = true
		f.build()

		err := f.reconc.RunOnce(context.Background(), t1)

		require.ErrorIs(t, err, billing.ErrProviderUnavailable)
		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusActive, sub.Status)
		assert.Empty(t, f.observer.counts)
	})

	t.Run("Error: a provider rejection of one subscription does not stop the others", func(t *testing.T) {
		f := newReconFixture(t)
		f.providerSays(reconProviderID, billing.StatusCanceled)
		f.ledger.AddAccount(reconOtherUser, billing.AccountPlan{Plan: "paid"})
		f.ledger.SeedSubscription(billing.Subscription{
			ID: reconOtherSub, UserID: reconOtherUser, PlanID: "paid", Provider: billing.ProviderStripe,
			ProviderSubID: reconOtherPID, Status: billing.StatusActive,
		})
		f.build()

		err := f.reconc.RunOnce(context.Background(), t1)

		require.ErrorIs(t, err, billing.ErrProviderRejected)
		sub, _ := findSub(f.ledger.Subscriptions(reconUserID), reconLocalSub)
		assert.Equal(t, billing.StatusCanceled, sub.Status)
		assert.Equal(t, map[string]int{billing.CorrectionSubscription: 1, billing.CorrectionPlan: 1}, f.observer.counts)
	})
}

func TestReconciler_RequeueEvents(t *testing.T) {
	stuckAt := t1.Add(-2 * time.Hour)
	recentAt := t1.Add(-time.Minute)

	tests := []struct {
		name       string
		status     billing.EventStatus
		receivedAt time.Time
		nextAt     time.Time
		wantStatus billing.EventStatus
		wantNext   time.Time
		wantCount  int
	}{
		{
			name:       "Happy path: a failed event returns to received",
			status:     billing.EventFailed,
			receivedAt: recentAt,
			nextAt:     recentAt,
			wantStatus: billing.EventReceived,
			wantNext:   t1,
			wantCount:  1,
		},
		{
			name:       "Edge: a received event stuck with a future attempt is pulled forward",
			status:     billing.EventReceived,
			receivedAt: stuckAt,
			nextAt:     t1.Add(time.Hour),
			wantStatus: billing.EventReceived,
			wantNext:   t1,
			wantCount:  1,
		},
		{
			name:       "Edge: a recent received event with a future attempt is left alone",
			status:     billing.EventReceived,
			receivedAt: recentAt,
			nextAt:     t1.Add(time.Hour),
			wantStatus: billing.EventReceived,
			wantNext:   t1.Add(time.Hour),
			wantCount:  0,
		},
		{
			name:       "Edge: a processed event is never requeued",
			status:     billing.EventProcessed,
			receivedAt: stuckAt,
			nextAt:     stuckAt,
			wantStatus: billing.EventProcessed,
			wantNext:   stuckAt,
			wantCount:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newReconFixture(t)
			f.ledger.RemoveAccount(reconUserID)
			id := f.seedEvent(t, reconUserID, tc.status, tc.receivedAt, tc.nextAt)
			f.build()

			require.NoError(t, f.reconc.RunOnce(context.Background(), t1))

			ev, ok := f.ledger.Event(id)
			require.True(t, ok)
			assert.Equal(t, tc.wantStatus, ev.Status)
			assert.True(t, ev.NextAttemptAt.Equal(tc.wantNext), "next attempt %s", ev.NextAttemptAt)
			assert.Equal(t, tc.wantCount, f.observer.counts[billing.CorrectionEvent])
		})
	}

}

func TestReconciler_Run(t *testing.T) {
	t.Run("Edge: a cancelled context returns after the first pass", func(t *testing.T) {
		f := newReconFixture(t)
		f.build()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.NoError(t, f.reconc.Run(ctx))
	})
}
