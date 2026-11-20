package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/quota/mocks"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const planUserID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

var planNow = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

func planEngine(t *testing.T, store *mocks.MockStore, resolver app.PlanResolver, userID string) *echo.Echo {
	t.Helper()
	svc := quota.NewService(store, func() time.Time { return planNow })
	ctrl := app.NewPlanController(svc, resolver)

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
	ctrl.RegisterRoutes(e.Group("/me", inject))
	return e
}

func getPlan(t *testing.T, e *echo.Echo) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me/plan", nil))
	return rec
}

func planBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func staticResolver(paid bool) app.PlanResolver {
	return func(context.Context, string) (bool, error) { return paid, nil }
}

func TestPlanController_Get(t *testing.T) {
	t.Run("Happy path: a Free account reports only its plan and no generation allowance", func(t *testing.T) {
		store := mocks.NewMockStore(t)

		rec := getPlan(t, planEngine(t, store, staticResolver(false), planUserID))

		require.Equal(t, http.StatusOK, rec.Code)
		body := planBody(t, rec)
		assert.Equal(t, "free", body["plan"])
		assert.NotContains(t, body, "trial")
		assert.NotContains(t, body, "last_free_generation")
		assert.NotContains(t, body, "generations")
	})

	t.Run("Happy path: a paid account reports its daily quota and the next reset", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FetchDaily(mock.Anything, planUserID, "2026-11-14").Return(5, nil).Once()

		rec := getPlan(t, planEngine(t, store, staticResolver(true), planUserID))

		require.Equal(t, http.StatusOK, rec.Code)
		body := planBody(t, rec)
		assert.Equal(t, "paid", body["plan"])
		generations, ok := body["generations"].(map[string]any)
		require.True(t, ok, "generations must be an object, got %v", body["generations"])
		assert.InDelta(t, float64(5), generations["used"], 1e-9)
		assert.InDelta(t, float64(25), generations["remaining"], 1e-9)
		assert.Equal(t, "2026-11-15T00:00:00Z", generations["resets_at"])
		assert.NotContains(t, body, "trial", "a paid account must not expose the Free trial")
		assert.NotContains(t, body, "last_free_generation")
	})

	t.Run("Edge: a paid account with a full day reports zero remaining", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FetchDaily(mock.Anything, planUserID, "2026-11-14").Return(quota.PaidDailyLimit, nil).Once()

		rec := getPlan(t, planEngine(t, store, staticResolver(true), planUserID))

		generations := planBody(t, rec)["generations"].(map[string]any)
		assert.InDelta(t, float64(0), generations["remaining"], 1e-9)
	})

	t.Run("Edge: a request without an authenticated user is 401 UNAUTHORIZED", func(t *testing.T) {
		store := mocks.NewMockStore(t)

		rec := getPlan(t, planEngine(t, store, staticResolver(false), ""))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `"UNAUTHORIZED"`)
	})

	t.Run("Edge: a malformed user id is 401 before any read", func(t *testing.T) {
		store := mocks.NewMockStore(t)

		rec := getPlan(t, planEngine(t, store, staticResolver(false), "not-a-uuid"))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Error: a Plan resolver failure is 500 and reads no allowance", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		resolver := func(context.Context, string) (bool, error) { return false, errors.New("plan source down") }

		rec := getPlan(t, planEngine(t, store, resolver, planUserID))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), `"INTERNAL_ERROR"`)
	})

	t.Run("Error: a daily read failure is 500", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().FetchDaily(mock.Anything, planUserID, "2026-11-14").Return(0, errors.New("timeout")).Once()

		rec := getPlan(t, planEngine(t, store, staticResolver(true), planUserID))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestFreePlanResolver(t *testing.T) {
	t.Run("Happy path: every account is Free in F1a", func(t *testing.T) {
		paid, err := app.FreePlanResolver(context.Background(), planUserID)
		require.NoError(t, err)
		assert.False(t, paid)
	})
}
