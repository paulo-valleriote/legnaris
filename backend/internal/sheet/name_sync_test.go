package sheet_test

import (
	"context"
	"strings"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const fifthNameSyncTemplateID = "fifth_character_v1"

func fifthNameSyncDeps(t *testing.T) (*template.Template, sheettest.DAGMap) {
	t.Helper()
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	tmpl, ok := registry.FetchByID(fifthNameSyncTemplateID)
	require.True(t, ok)
	dag, ok := registry.DAG(fifthNameSyncTemplateID)
	require.True(t, ok)
	return tmpl, sheettest.DAGMap{fifthNameSyncTemplateID: dag}
}

func newFifthNameSyncService(t *testing.T, repo *mocks.MockRepository, templates *templatemocks.MockService, dags sheettest.DAGMap) sheet.Service {
	t.Helper()
	return sheet.NewService(repo, templates, dags, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
	)
}

func TestService_Create_SeedsIdentityName(t *testing.T) {
	tmpl, dags := fifthNameSyncDeps(t)
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: fifthNameSyncTemplateID}).Return(tmpl, nil).Once()
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
		identity, ok := created.Data["identity"].(map[string]any)
		return ok && identity["name"] == "Aria" && created.Name == "Aria"
	})).Return(nil).Once()

	svc := newFifthNameSyncService(t, repo, templates, dags)

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: fifthNameSyncTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "  Aria  ",
		OwnerID:    ownerUserID,
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "Aria", created.Name)
	assert.Equal(t, "Aria", created.Data["identity"].(map[string]any)["name"])
}

func TestService_CreateWithValues_UsesIdentityName(t *testing.T) {
	tmpl, dags := fifthNameSyncDeps(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: fifthNameSyncTemplateID}).Return(tmpl, nil).Maybe()

	tests := []struct {
		name     string
		values   map[string]any
		wantName string
	}{
		{
			name:     "Happy path: a generated identity name becomes the sheet name",
			values:   map[string]any{"identity": map[string]any{"name": "  Brenna  "}},
			wantName: "Brenna",
		},
		{
			name:     "Edge: values without a name seed the identity name from the sheet name",
			wantName: "Brenna",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
			repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
				identity, ok := created.Data["identity"].(map[string]any)
				return ok && created.Name == tc.wantName && identity["name"] == tc.wantName
			})).Return(nil).Once()
			svc := newFifthNameSyncService(t, repo, templates, dags)

			created, err := svc.CreateWithValues(context.Background(), sheet.CreateWithValuesCommand{
				TemplateID: fifthNameSyncTemplateID,
				Kind:       string(plan.KindCharacter),
				SheetName:  "Brenna",
				OwnerID:    ownerUserID,
				Values:     tc.values,
			}, nil)

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, tc.wantName, created.Name)
			assert.Equal(t, tc.wantName, created.Data["identity"].(map[string]any)["name"])
		})
	}
}

func TestService_PartialUpdate_SyncsSheetNameFromIdentity(t *testing.T) {
	tmpl, dags := fifthNameSyncDeps(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: fifthNameSyncTemplateID}).Return(tmpl, nil).Once()
	created := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: fifthNameSyncTemplateID,
		OwnerID:    ownerUserID,
		Kind:       plan.KindCharacter,
		Name:       "Aria",
		Data: map[string]any{
			"identity": map[string]any{"name": "Aria"},
		},
	}
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	svc := newFifthNameSyncService(t, repo, templates, dags)
	seeded, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: fifthNameSyncTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "Aria",
		OwnerID:    ownerUserID,
	})
	require.NoError(t, err)
	created.ID = seeded.ID
	created.Data = seeded.Data

	repo.EXPECT().FetchByID(mock.Anything, created.ID).Return(created, nil).Once()
	repo.EXPECT().PartialUpdate(mock.Anything, created.ID, int64(0), mock.MatchedBy(func(update map[string]any) bool {
		identity, ok := update["data"].(map[string]any)["identity"].(map[string]any)
		return ok && identity["name"] == "Bryn" && update["name"] == "Bryn"
	})).Return(int64(1), nil).Once()

	_, err = svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: created.ID,
		OwnerID: ownerUserID,
		Data:    map[string]any{"identity": map[string]any{"name": "Bryn"}},
	})
	require.NoError(t, err)
}

func TestService_PartialUpdate_BlankIdentityKeepsSheetName(t *testing.T) {
	_, dags := fifthNameSyncDeps(t)
	repo := mocks.NewMockRepository(t)
	created := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: fifthNameSyncTemplateID,
		OwnerID:    ownerUserID,
		Kind:       plan.KindCharacter,
		Name:       "Bryn",
		Data: map[string]any{
			"identity": map[string]any{"name": "Bryn"},
		},
	}
	repo.EXPECT().FetchByID(mock.Anything, created.ID).Return(created, nil).Once()
	repo.EXPECT().PartialUpdate(mock.Anything, created.ID, int64(0), mock.MatchedBy(func(update map[string]any) bool {
		_, hasName := update["name"]
		return !hasName
	})).Return(int64(1), nil).Once()
	svc := newFifthNameSyncService(t, repo, templatemocks.NewMockService(t), dags)

	_, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: created.ID,
		OwnerID: ownerUserID,
		Data:    map[string]any{"identity": map[string]any{"name": "   "}},
	})
	require.NoError(t, err)
}

func TestService_Duplicate_CopiesIdentityNameWithSuffix(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	tmpl, ok := registry.FetchByID("fifth_stat_block_v1")
	require.True(t, ok)
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fifth_stat_block_v1"}).Return(tmpl, nil).Once()
	source := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: "fifth_stat_block_v1",
		OwnerID:    ownerUserID,
		Kind:       plan.KindNPC,
		Name:       "Aria",
		Data:       map[string]any{"identity": map[string]any{"name": "Aria"}},
	}
	repo.EXPECT().FetchByID(mock.Anything, source.ID).Return(source, nil).Once()
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
		identity, ok := created.Data["identity"].(map[string]any)
		return ok && created.Name == "Aria (copy)" && identity["name"] == "Aria (copy)"
	})).Return(nil).Once()
	svc := sheet.NewService(repo, templates, sheettest.DAGMap{}, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
	)

	copied, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: source.ID, OwnerID: ownerUserID})
	require.NoError(t, err)
	require.NotNil(t, copied)
	assert.Equal(t, "Aria (copy)", copied.Name)
	assert.Equal(t, "Aria (copy)", copied.Data["identity"].(map[string]any)["name"])
}

func TestService_Create_RejectsOverlongIdentityName(t *testing.T) {
	tmpl, dags := fifthNameSyncDeps(t)
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: fifthNameSyncTemplateID}).Return(tmpl, nil).Once()
	svc := newFifthNameSyncService(t, repo, templates, dags)

	created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: fifthNameSyncTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  strings.Repeat("A", 61),
		OwnerID:    ownerUserID,
	})

	require.Error(t, err)
	assert.Nil(t, created)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeSheetNameTooLong, appErr.Code)
	assert.Equal(t, 422, appErr.HttpStatus())
	assert.Equal(t, map[string]any{"max_length": 60}, appErr.Details)
}

func TestService_Duplicate_TruncatesNameToNameLeafLimit(t *testing.T) {
	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	tmpl, ok := registry.FetchByID("fifth_stat_block_v1")
	require.True(t, ok)
	sourceName := strings.Repeat("A", 60)
	source := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: "fifth_stat_block_v1",
		OwnerID:    ownerUserID,
		Kind:       plan.KindNPC,
		Name:       sourceName,
		Data:       map[string]any{"identity": map[string]any{"name": sourceName}},
	}
	wantName := strings.Repeat("A", 53) + " (copy)"
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "fifth_stat_block_v1"}).Return(tmpl, nil).Once()
	repo.EXPECT().FetchByID(mock.Anything, source.ID).Return(source, nil).Once()
	repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindNPC, 2).Return(nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(created *sheet.Sheet) bool {
		identity, ok := created.Data["identity"].(map[string]any)
		return ok && created.Name == wantName && identity["name"] == wantName
	})).Return(nil).Once()
	svc := sheet.NewService(repo, templates, sheettest.DAGMap{}, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), runInline(t)),
	)

	copied, err := svc.Duplicate(context.Background(), sheet.DuplicateSheetCommand{SheetID: source.ID, OwnerID: ownerUserID})
	require.NoError(t, err)
	require.NotNil(t, copied)
	assert.Equal(t, wantName, copied.Name)
	assert.Len(t, []rune(copied.Name), 60)
	assert.Equal(t, wantName, copied.Data["identity"].(map[string]any)["name"])
}

func TestService_PartialUpdate_DiceOnlyKeepsSheetName(t *testing.T) {
	tmpl, dags := fifthNameSyncDeps(t)
	repo := mocks.NewMockRepository(t)
	templates := templatemocks.NewMockService(t)
	templates.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: fifthNameSyncTemplateID}).Return(tmpl, nil).Once()
	stored := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: fifthNameSyncTemplateID,
		OwnerID:    ownerUserID,
		Kind:       plan.KindCharacter,
		Name:       "Old Name",
		Data:       map[string]any{"identity": map[string]any{"name": "New Name"}},
	}
	expression := "1d20"
	repo.EXPECT().FetchByID(mock.Anything, stored.ID).Return(stored, nil).Once()
	repo.EXPECT().PartialUpdate(mock.Anything, stored.ID, int64(0), mock.MatchedBy(func(update map[string]any) bool {
		_, hasName := update["name"]
		return !hasName
	})).Return(int64(1), nil).Once()
	svc := newFifthNameSyncService(t, repo, templates, dags)

	_, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: stored.ID,
		OwnerID: ownerUserID,
		Dice:    &sheet.Dice{Section: map[string]*string{"identity": &expression}},
	})
	require.NoError(t, err)
}
