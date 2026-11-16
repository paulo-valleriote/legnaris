package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/billingtest"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webhookEngine(provider billing.Provider, ledger *billingtest.Ledger) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	inbox := billing.NewInbox(provider, ledger, func() time.Time { return billingNow })
	e.POST("/billing/webhook", app.NewBillingWebhookController(inbox).Receive)
	return e
}

func postWebhook(e *echo.Echo, body []byte, signature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", bytes.NewReader(body))
	if signature != "" {
		req.Header.Set("Stripe-Signature", signature)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func webhookErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Error.Code
}

func TestBillingWebhookController_Receive(t *testing.T) {
	validBody := []byte(`{"id":"evt_http_1","kind":"ignored"}`)

	t.Run("Happy path: a verified webhook is stored and answered with 200", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		rec := postWebhook(webhookEngine(billingtest.NewProvider(), ledger), validBody, billingtest.ValidSignature)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Len(t, ledger.Events(), 1)
	})

	t.Run("Edge: a duplicate delivery is answered with 200 and stored once", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		e := webhookEngine(billingtest.NewProvider(), ledger)
		assert.Equal(t, http.StatusOK, postWebhook(e, validBody, billingtest.ValidSignature).Code)
		assert.Equal(t, http.StatusOK, postWebhook(e, validBody, billingtest.ValidSignature).Code)
		assert.Len(t, ledger.Events(), 1)
	})

	t.Run("Error: a missing signature is 400 INVALID_SIGNATURE and stores nothing", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		rec := postWebhook(webhookEngine(billingtest.NewProvider(), ledger), validBody, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_SIGNATURE", webhookErrorCode(t, rec))
		assert.Empty(t, ledger.Events())
	})

	t.Run("Error: an unconfigured provider is 503 BILLING_UNAVAILABLE", func(t *testing.T) {
		rec := postWebhook(webhookEngine(billing.UnconfiguredProvider{}, billingtest.NewLedger()), validBody, billingtest.ValidSignature)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "BILLING_UNAVAILABLE", webhookErrorCode(t, rec))
	})

	t.Run("Edge: a body over the size cap is rejected as 400 INVALID_REQUEST", func(t *testing.T) {
		ledger := billingtest.NewLedger()
		oversized := bytes.Repeat([]byte("a"), 65*1024)
		rec := postWebhook(webhookEngine(billingtest.NewProvider(), ledger), oversized, billingtest.ValidSignature)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_REQUEST", webhookErrorCode(t, rec))
		assert.Empty(t, ledger.Events())
	})

	t.Run("Error: a malformed body with a valid signature is 400 INVALID_REQUEST", func(t *testing.T) {
		rec := postWebhook(webhookEngine(billingtest.NewProvider(), billingtest.NewLedger()), []byte("{nope"), billingtest.ValidSignature)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "INVALID_REQUEST", webhookErrorCode(t, rec))
	})
}
