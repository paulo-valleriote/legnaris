package sheet_test

import (
	"context"
	"log/slog"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	retiredStartTemplateID = "fantasy_v1"
	defaultStartTemplateID = "fifth_character_v1"
)

func startFallbackDAGs(t *testing.T) sheettest.DAGMap {
	t.Helper()
	dag, err := template.BuildDAG(defaultStartTemplateID, limitFields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{
		retiredStartTemplateID: dag,
		defaultStartTemplateID: dag,
	}
}

func TestService_CreateStartSheetFallsBackToCatalogDefault(t *testing.T) {
	tests := []struct {
		name        string
		kind        plan.SheetKind
		listed      []template.Template
		wantCreated bool
		wantMessage string
	}{
		{
			name: "Happy path: a stale character sign-up falls back to the catalog default",
			kind: plan.KindCharacter,
			listed: []template.Template{{
				ID: defaultStartTemplateID, Name: "5E Character", State: template.StateActive, Default: true,
				Kinds: []plan.SheetKind{plan.KindCharacter},
			}},
			wantCreated: true,
		},
		{
			name: "Edge: no catalog default accepts the requested kind and the retired template is refused",
			kind: plan.KindNPC,
			listed: []template.Template{{
				ID: defaultStartTemplateID, Name: "5E Character", State: template.StateActive, Default: true,
				Kinds: []plan.SheetKind{plan.KindCharacter},
			}},
			wantMessage: sheet.MsgTemplateRetiredRefused,
		},
		{
			name:        "Edge: an empty catalog leaves the retired template refused",
			kind:        plan.KindCharacter,
			wantMessage: sheet.MsgTemplateRetiredRefused,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmplSvc := templatemocks.NewMockService(t)
			tmplSvc.EXPECT().
				FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: retiredStartTemplateID}).
				Return(&template.Template{
					ID: retiredStartTemplateID, Name: "Fantasy Template", State: template.StateRetired,
					Kinds: []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy},
				}, nil).
				Once()
			tmplSvc.EXPECT().List(mock.Anything).Return(tc.listed, nil).Once()

			repo := mocks.NewMockRepository(t)
			if tc.wantCreated {
				repo.EXPECT().
					ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, plan.Free.SheetLimit(plan.KindCharacter)).
					Return(nil).
					Once()
				repo.EXPECT().
					Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
						return created.TemplateID == defaultStartTemplateID && created.Kind == plan.KindCharacter
					})).
					Return(nil).
					Once()
			}

			svc := sheet.NewService(repo, tmplSvc, startFallbackDAGs(t),
				logger.NewLogger(slog.LevelError), events.NoOp{},
				sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)))

			created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: retiredStartTemplateID,
				Kind:       string(tc.kind),
				SheetName:  "Hero",
				OwnerID:    ownerUserID,
			})

			if tc.wantCreated {
				require.NoError(t, err)
				require.NotNil(t, created)
				assert.Equal(t, defaultStartTemplateID, created.TemplateID)
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, tc.wantMessage, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot")
			repo.AssertNotCalled(t, "Create")
		})
	}
}

func TestService_CreateStartSheetCatalogDefaultFailureIsInternal(t *testing.T) {
	tmplSvc := templatemocks.NewMockService(t)
	tmplSvc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: retiredStartTemplateID}).
		Return(&template.Template{
			ID: retiredStartTemplateID, Name: "Fantasy Template", State: template.StateRetired,
			Kinds: []plan.SheetKind{plan.KindCharacter},
		}, nil).
		Once()
	tmplSvc.EXPECT().List(mock.Anything).Return(nil, assert.AnError).Once()

	repo := mocks.NewMockRepository(t)
	svc := sheet.NewService(repo, tmplSvc, startFallbackDAGs(t),
		logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: retiredStartTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "Hero",
		OwnerID:    ownerUserID,
	})

	require.Error(t, err)
	assert.Nil(t, created)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	assert.Equal(t, 500, appErr.HttpStatus())
	assert.Equal(t, sheet.MsgCatalogDefaultUnavailable, appErr.Message)
	repo.AssertNotCalled(t, "ReserveSlot")
	repo.AssertNotCalled(t, "Create")
}

func TestService_CreateStartSheetKeepsARetiredCampaignTemplate(t *testing.T) {
	resolved := template.Template{
		ID: retiredStartTemplateID, Name: "Fantasy Template", State: template.StateRetired,
		Kinds: []plan.SheetKind{plan.KindCharacter},
	}
	tmplSvc := templatemocks.NewMockService(t)
	tmplSvc.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: retiredStartTemplateID}).
		Return(&resolved, nil).
		Once()

	scope := mocks.NewMockCampaignScope(t)
	scope.EXPECT().
		AllowsTemplate(mock.Anything, ownerUserID, campaignCreateID, retiredStartTemplateID).
		Return(true, nil).
		Once()

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().
		ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, plan.Free.SheetLimit(plan.KindCharacter)).
		Return(nil).
		Once()
	repo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
			return created.TemplateID == retiredStartTemplateID
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(repo, tmplSvc, startFallbackDAGs(t),
		logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
		sheet.WithCampaignScope(scope))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: retiredStartTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "Aria",
		CampaignID: campaignCreateID,
		OwnerID:    ownerUserID,
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, retiredStartTemplateID, created.TemplateID)
	tmplSvc.AssertNotCalled(t, "List")
}
