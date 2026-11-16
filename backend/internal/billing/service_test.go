package billing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	userID      = "0190a1b2-0000-7000-8000-000000000001"
	subID       = "0190a1b2-0000-7000-8000-0000000000aa"
	customerID  = "cus_test_1"
	sessionURL  = "https://checkout.example/session/1"
	priceID     = "price_test_usd"
	successURL  = "https://app.example/billing/success"
	cancelURL   = "https://app.example/billing/cancel"
	fixedNowRFC = "2026-11-16T10:00:00Z"
)

var fixedNow = mustTime(fixedNowRFC)

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func newService(store billing.Store, provider billing.Provider) *billing.Service {
	return billing.NewService(store, provider, billing.Config{
		PriceID:    priceID,
		SuccessURL: successURL,
		CancelURL:  cancelURL,
		Now:        func() time.Time { return fixedNow },
	})
}

func appErrOf(t *testing.T, err error, code apperr.Code) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
	return appErr
}

func expectCustomer(provider *mocks.MockProvider, returned billing.Customer, err error) *mocks.MockProvider_CreateCustomer_Call {
	return provider.EXPECT().CreateCustomer(mock.Anything, mock.MatchedBy(func(r billing.CustomerRequest) bool {
		return r.IdempotencyKey == "customer:"+userID && r.UserID == userID
	})).Return(returned, err)
}

func expectSession(provider *mocks.MockProvider, id string, returned billing.CheckoutSession, err error) {
	provider.EXPECT().CreateCheckoutSession(mock.Anything, mock.MatchedBy(func(r billing.CheckoutRequest) bool {
		return r.IdempotencyKey == "checkout:"+id &&
			r.SubscriptionID == id &&
			r.CustomerID == customerID &&
			r.PriceID == priceID &&
			r.SuccessURL == successURL &&
			r.CancelURL == cancelURL &&
			r.UserID == userID
	})).Return(returned, err)
}

func liveSession() billing.CheckoutSession {
	return billing.CheckoutSession{ID: "cs_1", URL: sessionURL, ExpiresAt: fixedNow.Add(24 * time.Hour)}
}

func TestService_Checkout(t *testing.T) {
	t.Run("Happy path: a user without a live subscription gets a pending row and a provider session", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		var inserted billing.Subscription

		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, sub billing.Subscription) error {
			inserted = sub
			return nil
		})
		expectCustomer(provider, billing.Customer{ID: customerID}, nil)
		provider.EXPECT().CreateCheckoutSession(mock.Anything, mock.MatchedBy(func(r billing.CheckoutRequest) bool {
			return r.IdempotencyKey == "checkout:"+r.SubscriptionID && r.CustomerID == customerID && r.PriceID == priceID
		})).Return(liveSession(), nil)
		store.EXPECT().AttachCheckout(mock.Anything, mock.Anything, customerID, sessionURL, liveSession().ExpiresAt).
			RunAndReturn(func(_ context.Context, id, _, _ string, _ time.Time) (bool, error) {
				return id == inserted.ID, nil
			})

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
		assert.Equal(t, liveSession().ExpiresAt, got.ExpiresAt)
		assert.Equal(t, userID, inserted.UserID)
		assert.Equal(t, billing.StatusPending, inserted.Status)
		assert.Equal(t, "paid", inserted.PlanID)
		assert.Equal(t, billing.ProviderStripe, inserted.Provider)
	})

	t.Run("Happy path: a second request returns the stored session URL without calling the provider", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: subID, UserID: userID, Status: billing.StatusPending,
			CheckoutURL: sessionURL, CheckoutExpiresAt: fixedNow.Add(time.Hour),
		}, true, nil)

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
	})

	t.Run("Happy path: a pending row interrupted before its session was stored is completed with the same keys", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: subID, UserID: userID, Status: billing.StatusPending,
		}, true, nil)
		expectCustomer(provider, billing.Customer{ID: customerID}, nil)
		expectSession(provider, subID, liveSession(), nil)
		store.EXPECT().AttachCheckout(mock.Anything, subID, customerID, sessionURL, liveSession().ExpiresAt).Return(true, nil)

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
	})

	t.Run("Edge: an expired pending session is canceled and replaced by a new pending row", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		oldID := "0190a1b2-0000-7000-8000-0000000000bb"
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: oldID, UserID: userID, Status: billing.StatusPending,
			CheckoutURL: "https://checkout.example/old", CheckoutExpiresAt: fixedNow.Add(-time.Minute),
		}, true, nil).Once()
		store.EXPECT().Cancel(mock.Anything, oldID).Return(nil)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil).Once()
		var inserted billing.Subscription
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, sub billing.Subscription) error {
			inserted = sub
			return nil
		})
		expectCustomer(provider, billing.Customer{ID: customerID}, nil)
		provider.EXPECT().CreateCheckoutSession(mock.Anything, mock.MatchedBy(func(r billing.CheckoutRequest) bool {
			return r.SubscriptionID != oldID && r.IdempotencyKey == "checkout:"+r.SubscriptionID
		})).Return(liveSession(), nil)
		store.EXPECT().AttachCheckout(mock.Anything, mock.Anything, customerID, sessionURL, liveSession().ExpiresAt).Return(true, nil)

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
		assert.NotEqual(t, oldID, inserted.ID)
	})

	t.Run("Edge: a live subscription is rejected with ALREADY_SUBSCRIBED and the provider is not called", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: subID, UserID: userID, Status: billing.StatusActive,
		}, true, nil)

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		appErr := appErrOf(t, err, apperr.CodeAlreadySubscribed)
		assert.Equal(t, 409, appErr.HttpStatus())
	})

	t.Run("Edge: a concurrent insert that loses the unique race is re-read and reports ALREADY_SUBSCRIBED", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil).Once()
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(billing.ErrLiveExists).Once()
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: subID, UserID: userID, Status: billing.StatusActive,
		}, true, nil).Once()

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeAlreadySubscribed)
	})

	t.Run("Edge: a concurrent insert that loses the unique race returns the winner's session URL", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil).Once()
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(billing.ErrLiveExists).Once()
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{
			ID: subID, UserID: userID, Status: billing.StatusPending,
			CheckoutURL: sessionURL, CheckoutExpiresAt: fixedNow.Add(time.Hour),
		}, true, nil).Once()

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
	})

	t.Run("Edge: a session attach that finds the row canceled meanwhile starts over", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil).Twice()
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil).Twice()
		expectCustomer(provider, billing.Customer{ID: customerID}, nil).Times(2)
		provider.EXPECT().CreateCheckoutSession(mock.Anything, mock.Anything).Return(liveSession(), nil).Times(2)
		store.EXPECT().AttachCheckout(mock.Anything, mock.Anything, customerID, sessionURL, liveSession().ExpiresAt).
			Return(false, nil).Once()
		store.EXPECT().AttachCheckout(mock.Anything, mock.Anything, customerID, sessionURL, liveSession().ExpiresAt).
			Return(true, nil).Once()

		got, err := newService(store, provider).Checkout(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, sessionURL, got.URL)
	})

	t.Run("Error: a malformed user id is rejected before any store or provider call", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)

		_, err := newService(store, provider).Checkout(context.Background(), "not-a-uuid")
		requireAppErr(t, err, apperr.CodeInvalidRequest)
	})

	t.Run("Error: the provider being unavailable at customer creation maps to BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		expectCustomer(provider, billing.Customer{}, billing.ErrProviderUnavailable)

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		appErr := appErrOf(t, err, apperr.CodeBillingUnavailable)
		assert.Equal(t, 503, appErr.HttpStatus())
	})

	t.Run("Error: the provider being unavailable at session creation keeps the pending row for a retry", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		expectCustomer(provider, billing.Customer{ID: customerID}, nil)
		provider.EXPECT().CreateCheckoutSession(mock.Anything, mock.Anything).Return(billing.CheckoutSession{}, billing.ErrProviderUnavailable)

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeBillingUnavailable)
	})

	t.Run("Error: a provider rejection maps to INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		expectCustomer(provider, billing.Customer{}, billing.ErrProviderRejected)

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: a store failure on lookup maps to INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		provider := mocks.NewMockProvider(t)
		store.EXPECT().FindLive(mock.Anything, userID).Return(billing.Subscription{}, false, errors.New("db down"))

		_, err := newService(store, provider).Checkout(context.Background(), userID)
		requireAppErr(t, err, apperr.CodeInternal)
	})
}

func requireAppErr(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
}
