package app

import (
	"context"
	"log/slog"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/generation"
	genmocks "legnaris/backend/internal/generation/mocks"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	sheetmocks "legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/env"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const draftTestOwnerID = "01900000-0000-7000-8000-0000000000aa"

func draftTemplate() *template.Template {
	return &template.Template{
		ID:    "fantasy_v1",
		Name:  "Fantasy Template",
		State: template.StateDraft,
		Kinds: []plan.SheetKind{plan.KindCharacter, plan.KindNPC, plan.KindEnemy},
	}
}

func TestSheetService_RefusesDraftWhenRegistryDisallowsDrafts(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine(), template.WithDraftsAllowed(false))
	require.NoError(t, err)
	require.False(t, registry.DraftsAllowed())

	templates := templatemocks.NewMockService(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(draftTemplate(), nil).
		Once()
	svc := sheet.NewService(sheetmocks.NewMockRepository(t), templates, sheettest.DAGMap{},
		logger.NewLogger(slog.LevelError), events.NoOp{}, sheetDraftTemplateOption(registry))

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: "fantasy_v1", Kind: string(plan.KindCharacter), SheetName: "Hero", OwnerID: draftTestOwnerID,
	})

	require.Nil(t, created)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, sheet.MsgTemplateStateRefused, appErr.Message)
}

func TestGenerationService_RefusesDraftWhenRegistryDisallowsDrafts(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine(), template.WithDraftsAllowed(false))
	require.NoError(t, err)
	require.False(t, registry.DraftsAllowed())

	templates := genmocks.NewMockTemplateSource(t)
	templates.EXPECT().
		FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fantasy_v1"}).
		Return(draftTemplate(), nil).
		Once()
	cfg := env.Env{Variables: env.Variables{GenerationEnabled: true, GenerationDailyCap: 10}}
	svc := newGenerationService(cfg, registry, moduleOptions{}, generation.Deps{Templates: templates})

	id, err := svc.Create(context.Background(), generation.CreateCommand{
		UserID: draftTestOwnerID, TemplateID: "fantasy_v1", Kind: string(plan.KindCharacter),
		Concept: "A grizzled dwarf smith who forgot his name",
	})

	assert.Empty(t, id)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, generation.MsgTemplateState, appErr.Message)
}
