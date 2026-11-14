package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/quota/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeConsent answers the consent gate from a fixed outcome.
type fakeConsent struct {
	err error
}

func (f fakeConsent) Gate(context.Context, string) error { return f.err }

// fakeActive answers the active-generation lookup from a fixed outcome.
type fakeActive struct {
	id     string
	active bool
	err    error
}

func (f fakeActive) ActiveGenerationID(context.Context, string) (string, bool, error) {
	return f.id, f.active, f.err
}

// consentMissing is the refusal the gate returns when no current consent exists.
var consentMissing = apperr.NewError(apperr.CodeLLMConsentRequired, "llm consent is required", nil)

// planGenerationEngine mounts the plan controller with the generation status on.
func planGenerationEngine(t *testing.T, store *mocks.MockStore, paid bool, consent app.ConsentChecker, active app.ActiveGenerationReader) *echo.Echo {
	t.Helper()
	svc := quota.NewService(store, func() time.Time { return planNow })
	ctrl := app.NewPlanController(svc, staticResolver(paid), app.WithGenerationStatus(consent, active))

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

// TestPlanController_GenerationStatus pins the additive LGN-186 fields: the consent
// prompt is asked only of an account that can generate, and the active generation
// is null unless one is queued or running.
func TestPlanController_GenerationStatus(t *testing.T) {
	const jobID = "0196e4f2-0000-7000-8000-0000000000aa"
	tests := []struct {
		name        string
		paid        bool
		trial       quota.TrialRow
		daily       bool
		consent     app.ConsentChecker
		active      app.ActiveGenerationReader
		wantConsent bool
		wantActive  any
		wantErr     bool
	}{
		{
			name:        "Happy path: a Free account with allowance and no consent is asked for it",
			trial:       quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 0},
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: true,
			wantActive:  nil,
		},
		{
			name:        "Edge: a Free account with exhausted allowance is never asked for consent",
			trial:       quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 3},
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: false,
			wantActive:  nil,
		},
		{
			name:        "Edge: a Free account with no trial row is never asked for consent",
			trial:       quota.TrialRow{},
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: false,
		},
		{
			name:        "Edge: a Free account that already consented is not asked again",
			trial:       quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 1},
			consent:     fakeConsent{},
			active:      fakeActive{},
			wantConsent: false,
			wantActive:  nil,
		},
		{
			name:        "Happy path: a paid account without consent is asked for it",
			paid:        true,
			daily:       true,
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: true,
		},
		{
			name:       "Happy path: a queued generation is reported by id",
			trial:      quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 1},
			consent:    fakeConsent{},
			active:     fakeActive{id: jobID, active: true},
			wantActive: jobID,
		},
		{
			name:    "Error: a consent check that fails for another reason is a 500",
			trial:   quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 0},
			consent: fakeConsent{err: errors.New("store down")},
			active:  fakeActive{},
			wantErr: true,
		},
		{
			name:    "Error: an active-generation lookup failure is a 500",
			trial:   quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 1},
			consent: fakeConsent{},
			active:  fakeActive{err: errors.New("store down")},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := mocks.NewMockStore(t)
			store.EXPECT().FetchTrial(mock.Anything, planUserID).Return(tc.trial, nil).Maybe()
			store.EXPECT().FetchDaily(mock.Anything, planUserID, mock.Anything).Return(0, nil).Maybe()

			rec := getPlan(t, planGenerationEngine(t, store, tc.paid, tc.consent, tc.active))
			if tc.wantErr {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code)

			body := planBody(t, rec)
			assert.Equal(t, tc.wantConsent, body["llm_consent_required"])
			assert.Equal(t, tc.wantActive, body["active_generation_id"])
		})
	}
}

// TestPlanController_GenerationStatusAbsentWithoutOption pins that the fields are
// only computed when the controller is wired for them, so the older plan view keeps
// its shape for every other caller.
func TestPlanController_GenerationStatusAbsentWithoutOption(t *testing.T) {
	store := mocks.NewMockStore(t)
	store.EXPECT().FetchTrial(mock.Anything, planUserID).
		Return(quota.TrialRow{UserID: planUserID, Allowance: 3, Used: 0}, nil).Once()

	rec := getPlan(t, planEngine(t, store, staticResolver(false), planUserID))

	require.Equal(t, http.StatusOK, rec.Code)
	body := planBody(t, rec)
	assert.Equal(t, false, body["llm_consent_required"], "the field is always present")
	assert.Nil(t, body["active_generation_id"], "the field is always present, null when idle")
}
