package app_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/quota"
	quotamocks "legnaris/backend/internal/quota/mocks"
	"legnaris/backend/internal/sheet"
	sheetmocks "legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func sheetUsageEngine(t *testing.T, store *quotamocks.MockStore, svc sheet.Service) *echo.Echo {
	t.Helper()
	allowance := quota.NewService(store, func() time.Time { return planNow })
	ctrl := app.NewPlanController(allowance, staticResolver(false), app.WithSheetUsage(svc))

	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(auth.UserIDContextKey, planUserID)
			return next(c)
		}
	}
	ctrl.RegisterRoutes(e.Group("/me", inject))
	return e
}

func TestPlanController_SheetUsage(t *testing.T) {
	tests := []struct {
		name       string
		withUsage  bool
		usage      sheet.PlanUsage
		usageErr   error
		wantStatus int
		wantSheets bool
	}{
		{
			name:      "Happy path: the Free view carries the usage of every kind",
			withUsage: true,
			usage: sheet.PlanUsage{PlanID: "free", Kinds: []sheet.LimitStatus{
				{Kind: "character", Used: 1, Limit: 1},
				{Kind: "npc", Used: 0, Limit: 2},
			}},
			wantStatus: http.StatusOK,
			wantSheets: true,
		},
		{
			name:       "Edge: a controller without the usage reader omits the sheets key",
			withUsage:  false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "Error: a failed usage read is an error, not a partial plan",
			withUsage:  true,
			usageErr:   apperr.NewError(apperr.CodeInternal, "failed to count", errors.New("db down")),
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := quotamocks.NewMockStore(t)

			var e *echo.Echo
			if tc.withUsage {
				svc := sheetmocks.NewMockService(t)
				svc.EXPECT().PlanUsage(mock.Anything, planUserID).Return(tc.usage, tc.usageErr).Once()
				e = sheetUsageEngine(t, store, svc)
			} else {
				e = planEngine(t, store, staticResolver(false), planUserID)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me/plan", nil))
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantStatus != http.StatusOK {
				return
			}

			body := planBody(t, rec)
			if !tc.wantSheets {
				assert.NotContains(t, body, "sheets")
				return
			}
			sheets, ok := body["sheets"].(map[string]any)
			require.True(t, ok, "sheets must be an object")
			assert.Equal(t, map[string]any{"used": float64(1), "limit": float64(1)}, sheets["character"])
			assert.Equal(t, map[string]any{"used": float64(0), "limit": float64(2)}, sheets["npc"])
		})
	}
}
