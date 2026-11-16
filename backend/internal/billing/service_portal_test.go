package billing_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const providerSubID = "sub_test_1"

func portalSubscription(status billing.Status, customer string) billing.Subscription {
	return billing.Subscription{ID: subID, UserID: userID, Status: status, ProviderCustomerID: customer}
}

func activeSubscription(providerSubscriptionID string) billing.Subscription {
	return billing.Subscription{
		ID: subID, UserID: userID, Status: billing.StatusActive,
		ProviderCustomerID: customerID, ProviderSubID: providerSubscriptionID,
	}
}

func TestService_Portal(t *testing.T) {
	const portalURL = "https://billing.example/portal/1"

	t.Run("Happy path: a live subscription with a provider customer returns the portal URL", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(portalSubscription(billing.StatusActive, customerID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CreatePortalSession(mock.Anything, billing.PortalRequest{CustomerID: customerID}).
			Return(billing.PortalSession{URL: portalURL}, nil)

		got, err := newService(store, provider).Portal(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, portalURL, got)
	})

	t.Run("Edge: a user without a live subscription gets NO_SUBSCRIPTION and no provider call", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		provider := mocks.NewMockProvider(t)

		_, err := newService(store, provider).Portal(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeNoSubscription)
	})

	t.Run("Edge: a live subscription without a provider customer gets NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(portalSubscription(billing.StatusPending, ""), true, nil)
		provider := mocks.NewMockProvider(t)

		_, err := newService(store, provider).Portal(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeNoSubscription)
	})

	t.Run("Edge: a malformed user id is INVALID_REQUEST without touching the store", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)

		_, err := newService(store, provider).Portal(context.Background(), "not-a-uuid")
		requireAppErr(t, err, apperr.CodeInvalidRequest)
	})

	t.Run("Error: a store failure is INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, errors.New("db down"))
		provider := mocks.NewMockProvider(t)

		_, err := newService(store, provider).Portal(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: a provider outage is BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(portalSubscription(billing.StatusActive, customerID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CreatePortalSession(mock.Anything, mock.Anything).
			Return(billing.PortalSession{}, billing.ErrProviderUnavailable)

		_, err := newService(store, provider).Portal(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeBillingUnavailable)
	})

	t.Run("Error: a provider rejection is INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(portalSubscription(billing.StatusActive, customerID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CreatePortalSession(mock.Anything, mock.Anything).
			Return(billing.PortalSession{}, billing.ErrProviderRejected)

		_, err := newService(store, provider).Portal(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_CancelAtPeriodEnd(t *testing.T) {
	t.Run("Happy path: an active subscription is scheduled to cancel at period end at the provider", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(activeSubscription(providerSubID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CancelAtPeriodEnd(mock.Anything, billing.CancelRequest{ProviderSubscriptionID: providerSubID}).
			Return(nil)

		require.NoError(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID))
	})

	t.Run("Edge: repeating the cancel request calls the provider again and succeeds", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(activeSubscription(providerSubID), true, nil).Times(2)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CancelAtPeriodEnd(mock.Anything, billing.CancelRequest{ProviderSubscriptionID: providerSubID}).
			Return(nil).Times(2)
		svc := newService(store, provider)

		require.NoError(t, svc.CancelAtPeriodEnd(context.Background(), userID))
		require.NoError(t, svc.CancelAtPeriodEnd(context.Background(), userID))
	})

	t.Run("Edge: a user without a live subscription gets NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		provider := mocks.NewMockProvider(t)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeNoSubscription)
	})

	t.Run("Edge: a pending subscription cannot be canceled and gets NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(portalSubscription(billing.StatusPending, customerID), true, nil)
		provider := mocks.NewMockProvider(t)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeNoSubscription)
	})

	t.Run("Edge: an active subscription not yet synced from the provider gets NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(activeSubscription(""), true, nil)
		provider := mocks.NewMockProvider(t)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeNoSubscription)
	})

	t.Run("Edge: a malformed user id is INVALID_REQUEST without touching the store", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), ""), apperr.CodeInvalidRequest)
	})

	t.Run("Error: a store failure is INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, errors.New("db down"))
		provider := mocks.NewMockProvider(t)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeInternal)
	})

	t.Run("Error: a provider outage is BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(activeSubscription(providerSubID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CancelAtPeriodEnd(mock.Anything, mock.Anything).Return(billing.ErrProviderUnavailable)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeBillingUnavailable)
	})

	t.Run("Error: a provider rejection is INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(activeSubscription(providerSubID), true, nil)
		provider := mocks.NewMockProvider(t)
		provider.EXPECT().CancelAtPeriodEnd(mock.Anything, mock.Anything).Return(billing.ErrProviderRejected)

		requireAppErr(t, newService(store, provider).CancelAtPeriodEnd(context.Background(), userID), apperr.CodeInternal)
	})
}
