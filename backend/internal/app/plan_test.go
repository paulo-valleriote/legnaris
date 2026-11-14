package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/response"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// planRouter mounts GET /me/plan behind a fixed owner, over a mocked service.
func planRouter(t *testing.T, svc sheet.Service) http.Handler {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(errorTestOwnerID))
	app.NewPlanController(svc).RegisterRoutes(e.Group("/me"))
	return e
}

func TestPlanController_Get(t *testing.T) {
	tests := []struct {
		name       string
		usage      sheet.PlanUsage
		svcErr     error
		wantStatus int
		wantCode   string
		wantBody   app.PlanDTO
	}{
		{
			name: "Happy path: a Free owner sees plan, usage and allowance per kind",
			usage: sheet.PlanUsage{PlanID: plan.IDFree, Kinds: []sheet.LimitStatus{
				{Kind: plan.KindCharacter, Used: 1, Limit: 1},
				{Kind: plan.KindNPC, Used: 0, Limit: 2},
				{Kind: plan.KindMonster, Used: 2, Limit: 2},
			}},
			wantStatus: http.StatusOK,
			wantBody: app.PlanDTO{Plan: plan.IDFree, Sheets: map[string]app.SheetUsageDTO{
				"character": {Used: 1, Limit: 1},
				"npc":       {Used: 0, Limit: 2},
				"monster":   {Used: 2, Limit: 2},
			}},
		},
		{
			name: "Edge: an owner with no Sheets reads zero usage",
			usage: sheet.PlanUsage{PlanID: plan.IDPaid, Kinds: []sheet.LimitStatus{
				{Kind: plan.KindCharacter, Limit: 10},
			}},
			wantStatus: http.StatusOK,
			wantBody: app.PlanDTO{Plan: plan.IDPaid, Sheets: map[string]app.SheetUsageDTO{
				"character": {Used: 0, Limit: 10},
			}},
		},
		{
			name:       "Error: a failed plan lookup is an internal error",
			svcErr:     apperr.NewError(apperr.CodeInternal, "failed to resolve plan", errors.New("db down")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   string(apperr.CodeInternal),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().PlanUsage(mock.Anything, errorTestOwnerID).Return(tc.usage, tc.svcErr).Once()

			req := httptest.NewRequest(http.MethodGet, "/me/plan", nil)
			rec := httptest.NewRecorder()
			planRouter(t, svc).ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantCode != "" {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Error.Code)
				return
			}
			var body response.Success[app.PlanDTO]
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantBody, body.Data)
		})
	}
}

// TestErrorMapping_CreateSheetLimitCarriesDetails pins the wire contract of a
// refused creation: 409 SHEET_LIMIT_REACHED with the kind and the allowance.
func TestErrorMapping_CreateSheetLimitCarriesDetails(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		reserveErr  error
		wantStatus  int
		wantCode    string
		wantDetails map[string]any
	}{
		{
			name:        "Error: a full character allowance is a 409 with kind and limit",
			body:        `{"template_id":"fantasy_v1","kind":"character","sheet_name":"Second"}`,
			reserveErr:  sheet.ErrSheetLimitReached,
			wantStatus:  http.StatusConflict,
			wantCode:    string(apperr.CodeSheetLimitReached),
			wantDetails: map[string]any{"kind": "character", "limit": float64(1)},
		},
		{
			name:       "Error: a missing kind is a 400 before any write",
			body:       `{"template_id":"fantasy_v1","sheet_name":"NoKind"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   string(apperr.CodeInvalidRequest),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.reserveErr != nil {
				repo.EXPECT().ReserveSlot(mock.Anything, errorTestOwnerID, plan.KindCharacter, 1).Return(tc.reserveErr).Once()
			}
			templates := templatemocks.NewMockService(t)
			templates.EXPECT().
				FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
				Return(&template.Template{ID: "fantasy_v1", Kinds: []plan.SheetKind{plan.KindCharacter}}, nil).
				Maybe()

			txRunner := mocks.NewMockTransactionRunner(t)
			txRunner.EXPECT().WithTransaction(mock.Anything, mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
				Maybe()
			plans := mocks.NewMockPlanSource(t)
			plans.EXPECT().PlanOf(mock.Anything, errorTestOwnerID).Return(plan.Free, nil).Maybe()

			svc := sheet.NewService(repo, templates, newErrorTestDAG(t), testLog, sheet.WithPlanLimits(plans, txRunner))
			e := echo.New()
			e.HTTPErrorHandler = app.HTTPErrorHandler
			e.Use(withOwner(errorTestOwnerID))
			app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))

			req := httptest.NewRequest(http.MethodPost, "/sheets", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			var body struct {
				Error struct {
					Code    string         `json:"code"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantDetails, body.Error.Details)
		})
	}
}
