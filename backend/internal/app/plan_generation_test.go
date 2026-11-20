package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/llmconsent"
	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/quota/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeConsent struct {
	err       error
	record    llmconsent.Record
	statusErr error
}

func (f fakeConsent) Gate(context.Context, string) error { return f.err }

func (f fakeConsent) Status(context.Context, string) (llmconsent.Record, error) {
	return f.record, f.statusErr
}

type fakeActive struct {
	id     string
	active bool
	err    error
}

func (f fakeActive) ActiveGenerationID(context.Context, string) (string, bool, error) {
	return f.id, f.active, f.err
}

var consentMissing = apperr.NewError(apperr.CodeLLMConsentRequired, "llm consent is required", nil)

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

func TestPlanController_GenerationStatus(t *testing.T) {
	const jobID = "0196e4f2-0000-7000-8000-0000000000aa"
	tests := []struct {
		name        string
		paid        bool
		consent     app.ConsentChecker
		active      app.ActiveGenerationReader
		wantConsent bool
		wantActive  any
		wantErr     bool
	}{
		{
			name:        "Happy path: a Free account without consent is never asked for it",
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: false,
			wantActive:  nil,
		},
		{
			name:        "Edge: a Free account that already consented is not asked again",
			consent:     fakeConsent{},
			active:      fakeActive{},
			wantConsent: false,
			wantActive:  nil,
		},
		{
			name:        "Happy path: a paid account without consent is asked for it",
			paid:        true,
			consent:     fakeConsent{err: consentMissing},
			active:      fakeActive{},
			wantConsent: true,
		},
		{
			name:       "Happy path: a queued generation is reported by id",
			consent:    fakeConsent{},
			active:     fakeActive{id: jobID, active: true},
			wantActive: jobID,
		},
		{
			name:    "Error: a consent read that fails for another reason is a 500",
			consent: fakeConsent{statusErr: errors.New("store down")},
			active:  fakeActive{},
			wantErr: true,
		},
		{
			name:    "Error: an active-generation lookup failure is a 500",
			consent: fakeConsent{},
			active:  fakeActive{err: errors.New("store down")},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := mocks.NewMockStore(t)
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

func TestPlanController_GenerationStatusAbsentWithoutOption(t *testing.T) {
	store := mocks.NewMockStore(t)

	rec := getPlan(t, planEngine(t, store, staticResolver(false), planUserID))

	require.Equal(t, http.StatusOK, rec.Code)
	body := planBody(t, rec)
	assert.Equal(t, false, body["llm_consent_required"], "the field is always present")
	assert.Nil(t, body["active_generation_id"], "the field is always present, null when idle")
}

func TestPlanController_ConsentRecord(t *testing.T) {
	consentedAt := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)
	storeErr := errors.New("store down")
	tests := []struct {
		name        string
		consent     fakeConsent
		wantAt      *time.Time
		wantVersion any
		wantStatus  int
	}{
		{
			name:        "Happy path: a current consent reports its date and version",
			consent:     fakeConsent{record: llmconsent.Record{At: &consentedAt, Version: "1"}},
			wantAt:      &consentedAt,
			wantVersion: "1",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Edge: an outdated consent is reported as stored, not as current",
			consent:     fakeConsent{record: llmconsent.Record{At: &consentedAt, Version: "0"}, err: consentMissing},
			wantAt:      &consentedAt,
			wantVersion: "0",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Edge: a player who never consented has null date and version",
			consent:     fakeConsent{err: consentMissing},
			wantVersion: nil,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Edge: a withdrawn consent has null date and version",
			consent:     fakeConsent{record: llmconsent.Record{Version: "1"}, err: consentMissing},
			wantVersion: nil,
			wantStatus:  http.StatusOK,
		},
		{
			name:       "Error: a consent read that fails is a 500, never a guessed answer",
			consent:    fakeConsent{statusErr: storeErr},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := mocks.NewMockStore(t)

			rec := getPlan(t, planGenerationEngine(t, store, false, tc.consent, fakeActive{}))
			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus != http.StatusOK {
				return
			}

			body := planBody(t, rec)
			assert.Equal(t, tc.wantVersion, body["llm_consent_version"])
			if tc.wantAt == nil {
				assert.Nil(t, body["llm_consent_at"])
				return
			}
			raw, ok := body["llm_consent_at"].(string)
			require.True(t, ok, "llm_consent_at must be a timestamp string")
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			require.NoError(t, err)
			assert.True(t, tc.wantAt.Equal(parsed), "consent date must round-trip: got %s", raw)
		})
	}
}
