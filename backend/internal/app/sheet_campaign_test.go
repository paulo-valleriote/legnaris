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

const campaignCreateRequestID = "01900000-0000-7000-8000-0000000000cc"

func newCampaignCreateRouter(t *testing.T, state template.State, scope sheet.CampaignScope) (http.Handler, *mocks.MockRepository) {
	t.Helper()
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(&template.Template{
			ID:    "fantasy_v1",
			State: state,
			Kinds: []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy},
		}, nil).
		Maybe()
	tx := mocks.NewMockTransactionRunner(t)
	tx.EXPECT().
		WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		Maybe()
	plans := mocks.NewMockPlanSource(t)
	plans.EXPECT().PlanOf(mock.Anything, errorTestOwnerID).Return(plan.Free, nil).Maybe()

	svc := sheet.NewService(repo, templates, newErrorTestDAG(t), testLog, events.NoOp{},
		sheet.WithPlanLimits(plans, tx),
		sheet.WithCampaignScope(scope),
	)
	e := echo.New()
	e.HTTPErrorHandler = app.HTTPErrorHandler
	e.Use(withOwner(errorTestOwnerID))
	app.NewSheetController(svc).RegisterRoutes(e.Group("/sheets"))
	return e, repo
}

func TestCreateSheet_RetiredTemplateCampaignRefusalIs400(t *testing.T) {
	scope := mocks.NewMockCampaignScope(t)
	scope.EXPECT().
		AllowsTemplate(mock.Anything, errorTestOwnerID, campaignCreateRequestID, "fantasy_v1").
		Return(false, nil).
		Once()
	router, repo := newCampaignCreateRouter(t, template.StateRetired, scope)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sheets",
		strings.NewReader(`{"template_id":"fantasy_v1","kind":"character","sheet_name":"Hero","campaign_id":"`+campaignCreateRequestID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, string(apperr.CodeInvalidRequest), body.Error.Code)
	assert.Equal(t, sheet.MsgCampaignScopeRefused, body.Error.Message)
	repo.AssertNotCalled(t, "ReserveSlot", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateSheet_RetiredTemplateInsideCampaignCreatesUnlinkedSheet(t *testing.T) {
	scope := mocks.NewMockCampaignScope(t)
	scope.EXPECT().
		AllowsTemplate(mock.Anything, errorTestOwnerID, campaignCreateRequestID, "fantasy_v1").
		Return(true, nil).
		Once()
	router, repo := newCampaignCreateRouter(t, template.StateRetired, scope)
	repo.EXPECT().
		ReserveSlot(mock.Anything, errorTestOwnerID, plan.KindCharacter, plan.Free.SheetLimit(plan.KindCharacter)).
		Return(nil).
		Once()
	repo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
			return created.CampaignID == nil && created.TemplateID == "fantasy_v1"
		})).
		Return(nil).
		Once()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sheets",
		strings.NewReader(`{"template_id":"fantasy_v1","kind":"character","sheet_name":"Hero","campaign_id":"`+campaignCreateRequestID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	_, linked := body.Data["campaign_id"]
	assert.False(t, linked)
}
