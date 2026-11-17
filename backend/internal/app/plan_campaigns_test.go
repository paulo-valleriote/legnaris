package app_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/campaign"
	campaignmocks "legnaris/backend/internal/campaign/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/quota"
	quotamocks "legnaris/backend/internal/quota/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ownedCampaignsEngine(t *testing.T, store *quotamocks.MockStore, limiter *campaign.OwnershipLimiter) *echo.Echo {
	t.Helper()
	allowance := quota.NewService(store, func() time.Time { return planNow })
	ctrl := app.NewPlanController(allowance, staticResolver(false), app.WithOwnedCampaigns(limiter))

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

func TestPlanController_OwnedCampaigns(t *testing.T) {
	boom := errors.New("db down")
	tests := []struct {
		name       string
		withLimit  bool
		plan       plan.Plan
		planErr    error
		count      int
		countErr   error
		wantStatus int
		wantOwned  bool
		wantUsed   float64
		wantLimit  float64
	}{
		{
			name:       "Happy path: the Free view carries owned Campaigns used against one",
			withLimit:  true,
			plan:       plan.Free,
			count:      1,
			wantStatus: http.StatusOK,
			wantOwned:  true,
			wantUsed:   1,
			wantLimit:  1,
		},
		{
			name:       "Happy path: the Paid view carries owned Campaigns used against five",
			withLimit:  true,
			plan:       plan.Paid,
			count:      2,
			wantStatus: http.StatusOK,
			wantOwned:  true,
			wantUsed:   2,
			wantLimit:  5,
		},
		{
			name:       "Edge: a controller without the limiter omits the owned_campaigns key",
			withLimit:  false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "Error: a failed plan read is an error, not a partial plan",
			withLimit:  true,
			planErr:    boom,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "Error: a failed count is an error, not a partial plan",
			withLimit:  true,
			plan:       plan.Free,
			countErr:   apperr.NewError(apperr.CodeInternal, "failed to read", boom),
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := quotamocks.NewMockStore(t)
			store.EXPECT().FetchTrial(mock.Anything, planUserID).
				Return(quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 1}, nil).Maybe()

			var e *echo.Echo
			if tc.withLimit {
				plans := campaignmocks.NewMockPlanSource(t)
				counters := campaignmocks.NewMockOwnershipCounter(t)
				if tc.planErr != nil {
					plans.EXPECT().PlanOf(mock.Anything, planUserID).Return(plan.Plan{}, tc.planErr).Once()
				} else {
					plans.EXPECT().PlanOf(mock.Anything, planUserID).Return(tc.plan, nil).Once()
					counters.EXPECT().Count(mock.Anything, planUserID).Return(tc.count, tc.countErr).Maybe()
				}
				e = ownedCampaignsEngine(t, store, campaign.NewOwnershipLimiter(plans, counters))
			} else {
				e = planEngine(t, store, staticResolver(false), planUserID)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me/plan", nil))
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			if tc.wantStatus != http.StatusOK {
				return
			}

			body := planBody(t, rec)
			if !tc.wantOwned {
				assert.NotContains(t, body, "owned_campaigns")
				return
			}
			owned, ok := body["owned_campaigns"].(map[string]any)
			require.True(t, ok, "owned_campaigns must be an object")
			assert.Equal(t, tc.wantUsed, owned["used"])
			assert.Equal(t, tc.wantLimit, owned["limit"])
		})
	}
}
