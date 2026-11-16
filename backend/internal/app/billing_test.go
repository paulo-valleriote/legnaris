package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/billing/mocks"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const billingUserID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

var billingNow = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

func billingEngine(t *testing.T, svc *billing.Service, userID string) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if userID != "" {
				c.Set(auth.UserIDContextKey, userID)
			}
			return next(c)
		}
	}
	app.NewBillingController(svc).RegisterRoutes(e.Group("/billing", inject))
	return e
}

func postCheckout(e *echo.Echo) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/billing/checkout", nil))
	return rec
}

func billingService(store billing.Store, provider billing.Provider) *billing.Service {
	return billing.NewService(store, provider, billing.Config{
		PriceID:    "price_test_usd",
		SuccessURL: "https://app.example/ok",
		CancelURL:  "https://app.example/cancel",
		Now:        func() time.Time { return billingNow },
	})
}

func TestBillingController_Checkout(t *testing.T) {
	t.Run("Happy path: a new checkout returns 200 with the session URL and expiry", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		store.EXPECT().AttachCheckout(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(true, nil)
		provider := billingtest.NewProvider()
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postCheckout(e)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data struct {
				URL       string    `json:"url"`
				ExpiresAt time.Time `json:"expires_at"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		assert.NotEmpty(t, envelope.Data.URL)
		assert.False(t, envelope.Data.ExpiresAt.IsZero())
	})

	t.Run("Edge: a request without an authenticated user is 401", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), "")

		rec := postCheckout(e)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "UNAUTHORIZED", errorCodeOf(t, rec))
	})

	t.Run("Error: a live subscription is 409 ALREADY_SUBSCRIBED", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{
			ID: "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e01", UserID: billingUserID, Status: billing.StatusActive,
		}, true, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postCheckout(e)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "ALREADY_SUBSCRIBED", errorCodeOf(t, rec))
	})

	t.Run("Error: a provider outage is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		provider := billingtest.NewProvider()
		provider.Unavailable = true
		e := billingEngine(t, billingService(store, provider), billingUserID)

		rec := postCheckout(e)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", errorCodeOf(t, rec))
	})

	t.Run("Edge: an unconfigured billing provider is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, nil)
		store.EXPECT().InsertPending(mock.Anything, mock.Anything).Return(nil)
		e := billingEngine(t, billingService(store, billing.UnconfiguredProvider{}), billingUserID)

		rec := postCheckout(e)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", errorCodeOf(t, rec))
	})
}
