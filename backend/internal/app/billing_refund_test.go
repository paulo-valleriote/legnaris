package app_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"
	"legnaris/backend/internal/billing/mocks"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func postRefund(t *testing.T, e *echo.Echo) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/billing/refund", nil))
	return rec
}

func refundableSub(paidAgo time.Duration) billing.Subscription {
	paid := billingNow.Add(-paidAgo)
	return billing.Subscription{
		ID:          "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e01",
		UserID:      billingUserID,
		Status:      billing.StatusActive,
		FirstPaidAt: &paid,
	}
}

func TestBillingController_Refund(t *testing.T) {
	t.Run("Happy path: a refund inside the window answers 202 with the requested status", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		sub := refundableSub(24 * time.Hour)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(sub, true, nil)
		store.EXPECT().RequestRefund(mock.Anything, sub.ID, billingNow).Return(true, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postRefund(t, e)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		var envelope struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		assert.Equal(t, "requested", envelope.Data.Status)
	})

	t.Run("Edge: a request without an authenticated user is 401", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), "")

		rec := postRefund(t, e)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "UNAUTHORIZED", errorCodeOf(t, rec))
	})

	t.Run("Error: a refund after the 7-day window is 409 REFUND_NOT_ELIGIBLE with the reason", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(refundableSub(8*24*time.Hour), true, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postRefund(t, e)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "REFUND_NOT_ELIGIBLE", errorCodeOf(t, rec))
		assert.Contains(t, rec.Body.String(), billing.ReasonWindowExpired)
	})

	t.Run("Edge: a repeated request while the refund is in progress is 409 REFUND_NOT_ELIGIBLE", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		sub := refundableSub(time.Hour)
		sub.RefundState = billing.RefundRequested
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(sub, true, nil)
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postRefund(t, e)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "REFUND_NOT_ELIGIBLE", errorCodeOf(t, rec))
	})

	t.Run("Error: a store failure during the refund is 500 INTERNAL_ERROR", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FindLive(mock.Anything, billingUserID).Return(billing.Subscription{}, false, errors.New("store down"))
		e := billingEngine(t, billingService(store, billingtest.NewProvider()), billingUserID)

		rec := postRefund(t, e)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "INTERNAL_ERROR", errorCodeOf(t, rec))
	})
}
