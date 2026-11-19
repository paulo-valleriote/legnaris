package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"legnaris/backend/internal/app"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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

			svc := sheet.NewService(repo, templates, newErrorTestDAG(t), testLog, events.NoOp{}, sheet.WithPlanLimits(plans, txRunner))
			e := echo.New()
			e.HTTPErrorHandler = app.HTTPErrorHandler
			e.Use(withOwner(errorTestOwnerID))
			app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sheets", strings.NewReader(tc.body))
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
