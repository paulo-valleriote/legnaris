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

func fifthAcceptTemplate(t *testing.T, id string, state template.State) (template.Template, sheettest.DAGMap) {
	t.Helper()
	for _, candidate := range template.DefaultTemplates() {
		if candidate.ID != id {
			continue
		}
		candidate.State = state
		dag, err := template.BuildDAGWithActions(candidate.ID, candidate.Fields, candidate.Actions, engine.NewFormulaEngine())
		require.NoError(t, err)
		return candidate, sheettest.DAGMap{candidate.ID: dag}
	}
	require.FailNow(t, "missing default template", id)
	return template.Template{}, nil
}

func TestService_CreateWithValuesAcceptsASucceededRetiredFantasyJob(t *testing.T) {
	tmpl, dags := fifthAcceptTemplate(t, "fantasy_v1", template.StateRetired)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(&tmpl, nil).
		Once()

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, mock.Anything).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
		return created.TemplateID == "fantasy_v1"
	})).Return(nil).Once()

	svc := sheet.NewService(repo, templates, dags, logger.NewLogger(slog.LevelError), events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
	)

	created, err := svc.CreateWithValues(context.Background(), sheet.CreateWithValuesCommand{
		TemplateID: "fantasy_v1",
		Kind:       "character",
		SheetName:  "Aria",
		OwnerID:    ownerUserID,
		Values:     map[string]any{"identity": map[string]any{"characterName": "Aria"}},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "fantasy_v1", created.TemplateID)
	identity, ok := created.Data["identity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Aria", identity["characterName"])
}

func TestService_CreateWithValuesFifthEditionTemplateLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		templateID    string
		kind          string
		state         template.State
		draftsAllowed bool
		wantCreated   bool
	}{
		{name: "Happy path: a succeeded 5E Character job still accepts after retirement", templateID: "fifth_character_v1", kind: "character", state: template.StateRetired, wantCreated: true},
		{name: "Happy path: a succeeded 5E Stat Block job still accepts after retirement", templateID: "fifth_stat_block_v1", kind: "npc", state: template.StateRetired, wantCreated: true},
		{name: "Happy path: a 5E Character draft accepts in development", templateID: "fifth_character_v1", kind: "character", state: template.StateDraft, draftsAllowed: true, wantCreated: true},
		{name: "Edge: a 5E Character draft is refused outside development", templateID: "fifth_character_v1", kind: "character", state: template.StateDraft},
		{name: "Edge: a 5E Stat Block draft is refused outside development", templateID: "fifth_stat_block_v1", kind: "enemy", state: template.StateDraft},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, dags := fifthAcceptTemplate(t, tc.templateID, tc.state)
			templates := templatemocks.NewMockService(t)
			templates.EXPECT().
				FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: tc.templateID}).
				Return(&tmpl, nil).
				Maybe()

			repo := mocks.NewMockRepository(t)
			if tc.wantCreated {
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.SheetKind(tc.kind), mock.Anything).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			}

			svc := sheet.NewService(repo, templates, dags, logger.NewLogger(slog.LevelError), events.NoOp{},
				sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
				sheet.WithDraftTemplates(tc.draftsAllowed),
			)

			created, err := svc.CreateWithValues(context.Background(), sheet.CreateWithValuesCommand{
				TemplateID: tc.templateID,
				Kind:       tc.kind,
				SheetName:  "Brenna",
				OwnerID:    ownerUserID,
				Values:     map[string]any{"identity": map[string]any{"name": "Brenna"}},
			}, nil)

			if tc.wantCreated {
				require.NoError(t, err)
				require.NotNil(t, created)
				assert.Equal(t, tc.templateID, created.TemplateID)
				identity, ok := created.Data["identity"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "Brenna", identity["name"])
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, sheet.MsgTemplateStateRefused, appErr.Message)
			repo.AssertNotCalled(t, "ReserveSlot")
			repo.AssertNotCalled(t, "Create")
		})
	}
}
