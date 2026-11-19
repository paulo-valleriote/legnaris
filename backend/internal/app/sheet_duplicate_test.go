package app_test

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/app"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	sheetmocks "legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/shared/apperr"
)

func newMockSheetRouter(svc sheet.Service) http.Handler {
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(campaignTestUserID))
	app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))
	return e
}

func duplicatedSheetFixture() *sheet.Sheet {
	return &sheet.Sheet{
		ID:         campaignTestSheetID,
		TemplateID: "dnd5e",
		OwnerID:    campaignTestUserID,
		Kind:       plan.KindNPC,
		Name:       "Brom (copy)",
		Data:       map[string]any{"stats": map[string]any{"hp": 42.0}},
	}
}

func TestSheetRoutes_Duplicate(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		wantCode string
	}{
		{name: "Happy path: a valid duplicate answers 201 with the copy", status: http.StatusCreated},
		{name: "Error: a plan limit refusal is 403 PLAN_LIMIT_REACHED", err: apperr.NewError(apperr.CodePlanLimitReached, "limit", nil), status: http.StatusForbidden, wantCode: "PLAN_LIMIT_REACHED"},
		{name: "Error: a character sheet is 422 SHEET_KIND_MISMATCH", err: apperr.NewError(apperr.CodeSheetKindMismatch, "kind", nil), status: http.StatusUnprocessableEntity, wantCode: "SHEET_KIND_MISMATCH"},
		{name: "Error: a foreign sheet is 404 NOT_FOUND", err: apperr.NewError(apperr.CodeNotFound, "sheet not found", nil), status: http.StatusNotFound, wantCode: "NOT_FOUND"},
		{name: "Error: an unexpected failure is 500", err: apperr.NewError(apperr.CodeInternal, "boom", nil), status: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := sheetmocks.NewMockService(t)
			svc.EXPECT().Duplicate(mock.Anything, sheet.DuplicateSheetCommand{
				SheetID: campaignTestSheetID,
				OwnerID: campaignTestUserID,
			}).Return(duplicatedSheetFixture(), tc.err).Once()

			rec := doCampaignRequest(t, newMockSheetRouter(svc), http.MethodPost, "/sheets/"+campaignTestSheetID+"/duplicate", "")
			assert.Equal(t, tc.status, rec.Code)
			if tc.wantCode != "" {
				assert.Equal(t, tc.wantCode, campaignErrorCode(t, rec))
				return
			}
			require.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"name":"Brom (copy)"`)
			assert.Contains(t, rec.Body.String(), `"kind":"npc"`)
		})
	}
}
