package billing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sweeper"
)

const deletionUser = "01900000-0000-7000-8000-0000000000d1"

func liveSub(id, providerSubID, customerID string, status billing.Status) billing.Subscription {
	return billing.Subscription{
		ID:                 id,
		UserID:             deletionUser,
		PlanID:             plan.IDPaid,
		Provider:           billing.ProviderStripe,
		ProviderCustomerID: customerID,
		ProviderSubID:      providerSubID,
		Status:             status,
	}
}

func remote(id, customerID string, status billing.Status) billing.ProviderSubscription {
	return billing.ProviderSubscription{ID: id, CustomerID: customerID, Status: status}
}

func TestDeletion_Settle(t *testing.T) {
	tests := []struct {
		name          string
		subs          []billing.Subscription
		remotes       []billing.ProviderSubscription
		provider      func(p *billingtest.Provider)
		wantErr       error
		wantPermanent bool
		wantCancels   []billing.CancelRequest
		wantScrubs    []billing.ScrubRequest
		wantRemote    map[string]billing.Status
	}{
		{
			name:    "Happy path: an active subscription is canceled now and its customer is scrubbed",
			subs:    []billing.Subscription{liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusActive)},
			remotes: []billing.ProviderSubscription{remote("sub_1", "cus_1", billing.StatusActive)},
			wantCancels: []billing.CancelRequest{
				{IdempotencyKey: "cancel:sub_local_1", ProviderSubscriptionID: "sub_1"},
			},
			wantScrubs: []billing.ScrubRequest{{IdempotencyKey: "scrub:cus_1", CustomerID: "cus_1"}},
			wantRemote: map[string]billing.Status{"sub_1": billing.StatusCanceled},
		},
		{
			name:    "Happy path: a past_due subscription is canceled too",
			subs:    []billing.Subscription{liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusPastDue)},
			remotes: []billing.ProviderSubscription{remote("sub_1", "cus_1", billing.StatusPastDue)},
			wantCancels: []billing.CancelRequest{
				{IdempotencyKey: "cancel:sub_local_1", ProviderSubscriptionID: "sub_1"},
			},
			wantScrubs: []billing.ScrubRequest{{IdempotencyKey: "scrub:cus_1", CustomerID: "cus_1"}},
			wantRemote: map[string]billing.Status{"sub_1": billing.StatusCanceled},
		},
		{
			name:       "Happy path: a pending checkout with no provider subscription is not canceled at the provider, only its customer is scrubbed",
			subs:       []billing.Subscription{liveSub("sub_local_1", "", "cus_1", billing.StatusPending)},
			wantScrubs: []billing.ScrubRequest{{IdempotencyKey: "scrub:cus_1", CustomerID: "cus_1"}},
		},
		{
			name: "Edge: a user with no subscriptions makes no provider call",
		},
		{
			name: "Edge: a subscription already canceled locally is not read or canceled again",
			subs: []billing.Subscription{liveSub("sub_local_1", "sub_1", "", billing.StatusCanceled)},
		},
		{
			name:       "Edge: a subscription already canceled at the provider is not canceled again, its customer is still scrubbed",
			subs:       []billing.Subscription{liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusActive)},
			remotes:    []billing.ProviderSubscription{remote("sub_1", "cus_1", billing.StatusCanceled)},
			wantScrubs: []billing.ScrubRequest{{IdempotencyKey: "scrub:cus_1", CustomerID: "cus_1"}},
		},
		{
			name: "Edge: two subscriptions of one customer scrub that customer once",
			subs: []billing.Subscription{
				liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusActive),
				liveSub("sub_local_2", "", "cus_1", billing.StatusCanceled),
			},
			remotes: []billing.ProviderSubscription{remote("sub_1", "cus_1", billing.StatusActive)},
			wantCancels: []billing.CancelRequest{
				{IdempotencyKey: "cancel:sub_local_1", ProviderSubscriptionID: "sub_1"},
			},
			wantScrubs: []billing.ScrubRequest{{IdempotencyKey: "scrub:cus_1", CustomerID: "cus_1"}},
			wantRemote: map[string]billing.Status{"sub_1": billing.StatusCanceled},
		},
		{
			name:    "Error: a provider subscription the provider does not know is a permanent failure",
			subs:    []billing.Subscription{liveSub("sub_local_1", "sub_missing", "cus_1", billing.StatusActive)},
			wantErr: billing.ErrProviderRejected, wantPermanent: true,
		},
		{
			name:     "Error: a provider outage while reading the subscription is retryable",
			subs:     []billing.Subscription{liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusActive)},
			remotes:  []billing.ProviderSubscription{remote("sub_1", "cus_1", billing.StatusActive)},
			provider: func(p *billingtest.Provider) { p.Unavailable = true },
			wantErr:  billing.ErrProviderUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ledger := billingtest.NewLedger()
			for _, sub := range tc.subs {
				ledger.SeedSubscription(sub)
			}
			provider := billingtest.NewProvider()
			for _, r := range tc.remotes {
				provider.SetSubscription(r)
			}
			if tc.provider != nil {
				tc.provider(provider)
			}

			err := billing.NewDeletion(billing.DeletionDeps{Provider: provider, Ledger: ledger}).Settle(context.Background(), deletionUser)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, tc.wantPermanent, errors.Is(err, sweeper.ErrPermanent))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantCancels, provider.Cancels())
			assert.Equal(t, tc.wantScrubs, provider.Scrubs())
			for id, want := range tc.wantRemote {
				got, err := provider.GetSubscription(context.Background(), id)
				require.NoError(t, err)
				assert.Equal(t, want, got.Status, id)
			}
		})
	}
}

func TestDeletion_SettleIsIdempotent(t *testing.T) {
	t.Run("Edge: a repeat after a successful settle cancels nothing more", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		ledger.SeedSubscription(liveSub("sub_local_1", "sub_1", "cus_1", billing.StatusActive))
		provider := billingtest.NewProvider()
		provider.SetSubscription(remote("sub_1", "cus_1", billing.StatusActive))
		deletion := billing.NewDeletion(billing.DeletionDeps{Provider: provider, Ledger: ledger})

		require.NoError(t, deletion.Settle(context.Background(), deletionUser))
		require.NoError(t, deletion.Settle(context.Background(), deletionUser))

		assert.Len(t, provider.Cancels(), 1)
	})
}
