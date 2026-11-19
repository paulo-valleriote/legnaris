package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/billing/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const billingProviderSubID = "sub_test_1"

func postBilling(t *testing.T, e http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
	return rec
}

func activeBillingSubscription() billing.Subscription {
	return billing.Subscription{
		ID:                 "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e01",
		UserID:             billingUserID,
		Status:             billing.StatusActive,
		ProviderCustomerID: "cus_test_1",
		ProviderSubID:      billingProviderSubID,
	}
}

func TestBillingController_Portal(t *testing.T) {
	t.Run("Happy path: a subscribed user gets 200 with the portal URL", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(activeBillingSubscription(), true, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postBilling(t, e, "/billing/portal")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		assert.NotEmpty(t, envelope.Data.URL)
	})

	t.Run("Edge: a request without an authenticated user is 401", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), "")

		rec := postBilling(t, e, "/billing/portal")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "UNAUTHORIZED", errorCodeOf(t, rec))
	})

	t.Run("Edge: a user without a provider customer is 404 NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postBilling(t, e, "/billing/portal")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NO_SUBSCRIPTION", errorCodeOf(t, rec))
	})

	t.Run("Error: a provider outage is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(activeBillingSubscription(), true, nil)
		provider := billingtest.NewProvider()
		provider.Unavailable = true
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postBilling(t, e, "/billing/portal")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", errorCodeOf(t, rec))
	})

	t.Run("Error: an unconfigured provider is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(activeBillingSubscription(), true, nil)
		e := billingEngine(t, billingService(store, billing.UnconfiguredProvider{}), billingUserID)

		rec := postBilling(t, e, "/billing/portal")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", errorCodeOf(t, rec))
	})
}

func TestBillingController_Cancel(t *testing.T) {
	t.Run("Happy path: an active subscription is accepted with 202 and scheduled at the provider", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(activeBillingSubscription(), true, nil)
		provider := billingtest.NewProvider()
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postBilling(t, e, "/billing/cancel")
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		var envelope struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		assert.Equal(t, "cancel_requested", envelope.Data.Status)
		assert.Equal(t, []string{billingProviderSubID}, provider.CancelsScheduled())
	})

	t.Run("Edge: a request without an authenticated user is 401", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), "")

		rec := postBilling(t, e, "/billing/cancel")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "UNAUTHORIZED", errorCodeOf(t, rec))
	})

	t.Run("Edge: a user without an active subscription is 404 NO_SUBSCRIPTION", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, nil)
		provider := billingtest.NewProvider()
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postBilling(t, e, "/billing/cancel")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "NO_SUBSCRIPTION", errorCodeOf(t, rec))
		assert.Empty(t, provider.CancelsScheduled())
	})

	t.Run("Error: a provider outage is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(activeBillingSubscription(), true, nil)
		provider := billingtest.NewProvider()
		provider.Unavailable = true
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postBilling(t, e, "/billing/cancel")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", errorCodeOf(t, rec))
	})
}
