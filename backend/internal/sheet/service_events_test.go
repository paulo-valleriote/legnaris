package sheet_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
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

const eventsCreateTemplateID = "events_test_v1"

// captureEvents accepts any number of Publish calls and records them. A test
// that expects nothing to be emitted asserts the recorded slice is empty.
func captureEvents(pub *eventsmocks.MockPublisher) *[]events.Event {
	got := &[]events.Event{}
	pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
		*got = append(*got, ev)
	}).Maybe()
	return got
}

func eventsOwnerActor() *string {
	actor := ownerUserID
	return &actor
}

func eventsCreateFields() template.FieldGroup {
	return template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
	}
}

func TestService_Create_EmitsSheetCreated(t *testing.T) {
	ctx := context.Background()
	fields := eventsCreateFields()
	dag, err := template.BuildDAG(eventsCreateTemplateID, fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	tests := []struct {
		name          string
		ownerHasSheet bool
		createErr     error
		wantEvents    int
	}{
		{name: "Happy path: a created sheet emits one sheet.created", wantEvents: 1},
		{name: "Error: a rejected repository insert emits nothing", createErr: errors.New("write failed"), wantEvents: 0},
		{name: "Edge: an owner who already has a sheet emits nothing", ownerHasSheet: true, wantEvents: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			tmplSvc := templatemocks.NewMockService(t)
			pub := eventsmocks.NewMockPublisher(t)
			got := captureEvents(pub)

			tmplSvc.EXPECT().FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: eventsCreateTemplateID}).
				Return(&template.Template{ID: eventsCreateTemplateID, Fields: fields, Kinds: []plan.SheetKind{plan.KindCharacter}}, nil).Once()
			switch {
			case tc.ownerHasSheet:
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(sheet.ErrSheetLimitReached).Once()
			default:
				repo.EXPECT().ReserveSlot(mock.Anything, ownerUserID, plan.KindCharacter, 1).Return(nil).Once()
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(tc.createErr).Once()
			}

			svc := sheet.NewService(repo, tmplSvc, sheettest.DAGMap{eventsCreateTemplateID: dag}, testLog, pub, limitOpts(t))
			created, err := svc.Create(ctx, sheet.CreateSheetCommand{TemplateID: eventsCreateTemplateID, Kind: string(plan.KindCharacter), OwnerID: ownerUserID})
			if tc.wantEvents == 0 {
				require.Error(t, err)
				assert.Empty(t, *got)
				return
			}
			require.NoError(t, err)

			require.Len(t, *got, tc.wantEvents)
			ev := (*got)[0]
			assert.Equal(t, "sheet.created", ev.Type)
			assert.Equal(t, eventsOwnerActor(), ev.ActorID)
			assert.Equal(t, "sheet", ev.AggregateType)
			assert.Equal(t, created.ID, ev.AggregateID)
			assert.Equal(t, created.Version, ev.AggregateVersion)
			assert.Equal(t, map[string]any{"template_id": eventsCreateTemplateID}, ev.Data)
		})
	}
}

func TestService_PartialUpdate_EmitsSheetUpdated(t *testing.T) {
	ctx := context.Background()
	dag := patchDAG(t)

	tests := []struct {
		name        string
		cmod        func(cmd *sheet.PartialUpdateSheetCommand)
		updateErr   error
		wantKeys    []string
		wantVersion int64
		wantEvents  int
	}{
		{
			name:        "Happy path: a data save emits changed keys and the new sheet version",
			cmod:        func(cmd *sheet.PartialUpdateSheetCommand) {},
			wantKeys:    []string{"stats"},
			wantVersion: 4,
			wantEvents:  1,
		},
		{
			name: "Happy path: a dice-only save emits changed_keys dice",
			cmod: func(cmd *sheet.PartialUpdateSheetCommand) {
				cmd.Data = nil
				cmd.Dice = &sheet.Dice{}
			},
			wantKeys:    []string{"dice"},
			wantVersion: 4,
			wantEvents:  1,
		},
		{
			name: "Edge: a combined data and dice save lists both keys in sorted order",
			cmod: func(cmd *sheet.PartialUpdateSheetCommand) {
				cmd.Dice = &sheet.Dice{}
			},
			wantKeys:    []string{"dice", "stats"},
			wantVersion: 4,
			wantEvents:  1,
		},
		{
			name:       "Error: a version conflict emits nothing",
			cmod:       func(cmd *sheet.PartialUpdateSheetCommand) {},
			updateErr:  sheet.ErrVersionConflict,
			wantEvents: 0,
		},
		{
			name:       "Error: a failed repository write emits nothing",
			cmod:       func(cmd *sheet.PartialUpdateSheetCommand) {},
			updateErr:  errors.New("write failed"),
			wantEvents: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			pub := eventsmocks.NewMockPublisher(t)
			got := captureEvents(pub)

			repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
			repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).
				Return(int64(4), tc.updateErr).Once()

			cmd := dexterityPatch(18)
			tc.cmod(&cmd)

			svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{patchTemplateID: dag}, testLog, pub)
			_, err := svc.PartialUpdate(ctx, cmd)
			if tc.wantEvents == 0 {
				require.Error(t, err)
				assert.Empty(t, *got)
				return
			}
			require.NoError(t, err)

			require.Len(t, *got, tc.wantEvents)
			ev := (*got)[0]
			assert.Equal(t, "sheet.updated", ev.Type)
			assert.Equal(t, eventsOwnerActor(), ev.ActorID)
			assert.Equal(t, "sheet", ev.AggregateType)
			assert.Equal(t, patchSheetID, ev.AggregateID)
			assert.Equal(t, tc.wantVersion, ev.AggregateVersion)
			assert.Equal(t, map[string]any{"changed_keys": tc.wantKeys, "sheet_version": tc.wantVersion}, ev.Data)
		})
	}
}

func TestService_SetLayout_EmitsSheetUpdated(t *testing.T) {
	ctx := context.Background()
	tmpl := layoutTemplate()

	tests := []struct {
		name       string
		layout     *sheet.Layout
		updateErr  error
		wantEvents int
	}{
		{name: "Happy path: a layout save emits changed_keys layout", layout: &sheet.Layout{SectionOrder: []string{"config", "stats"}}, wantEvents: 1},
		{name: "Edge: a reset to the default layout also emits", layout: nil, wantEvents: 1},
		{name: "Error: a failed layout write emits nothing", layout: nil, updateErr: errors.New("write failed"), wantEvents: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			templateSvc := templatemocks.NewMockService(t)
			pub := eventsmocks.NewMockPublisher(t)
			got := captureEvents(pub)

			stored := layoutStoredSheet(ownerUserID)
			stored.Version = 7
			repo.EXPECT().FetchByID(ctx, fetchSheetID).Return(stored, nil).Once()
			if tc.layout != nil {
				templateSvc.EXPECT().FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: stored.TemplateID}).
					Return(&tmpl, nil).Once()
			}
			repo.EXPECT().UpdateLayout(ctx, fetchSheetID, mock.Anything).Return(tc.updateErr).Once()

			svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog, pub)
			_, err := svc.SetLayout(ctx, sheet.SetLayoutCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Layout: tc.layout})
			if tc.wantEvents == 0 {
				require.Error(t, err)
				assert.Empty(t, *got)
				return
			}
			require.NoError(t, err)

			require.Len(t, *got, tc.wantEvents)
			ev := (*got)[0]
			assert.Equal(t, "sheet.updated", ev.Type)
			assert.Equal(t, fetchSheetID, ev.AggregateID)
			assert.Equal(t, int64(7), ev.AggregateVersion)
			assert.Equal(t, map[string]any{"changed_keys": []string{"layout"}, "sheet_version": int64(7)}, ev.Data)
		})
	}
}

func TestService_Delete_EmitsSheetDeleted(t *testing.T) {
	ctx := context.Background()
	owned := &sheet.Sheet{ID: updateSheetID, TemplateID: "fantasy_v1", OwnerID: ownerUserID, Version: 2}

	tests := []struct {
		name       string
		ownerID    string
		deleteErr  error
		wantEvents int
	}{
		{name: "Happy path: a delete emits one sheet.deleted with empty data", ownerID: ownerUserID, wantEvents: 1},
		{name: "Error: a foreign owner emits nothing", ownerID: "01900000-0000-7000-8000-0000000000bb", wantEvents: 0},
		{name: "Error: a failed delete emits nothing", ownerID: ownerUserID, deleteErr: errors.New("write failed"), wantEvents: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			pub := eventsmocks.NewMockPublisher(t)
			got := captureEvents(pub)

			repo.EXPECT().FetchByID(ctx, updateSheetID).Return(owned, nil).Once()
			if tc.wantEvents == 1 || tc.deleteErr != nil {
				repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(tc.deleteErr).Once()
			}
			if tc.wantEvents == 1 {
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.SheetKind("")).Return(nil).Once()
			}

			svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, pub, limitOpts(t))
			err := svc.Delete(ctx, sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: tc.ownerID})
			if tc.wantEvents == 0 {
				require.Error(t, err)
				assert.Empty(t, *got)
				return
			}
			require.NoError(t, err)

			require.Len(t, *got, tc.wantEvents)
			ev := (*got)[0]
			assert.Equal(t, "sheet.deleted", ev.Type)
			assert.Equal(t, eventsOwnerActor(), ev.ActorID)
			assert.Equal(t, updateSheetID, ev.AggregateID)
			assert.Equal(t, int64(2), ev.AggregateVersion)
			assert.Equal(t, map[string]any{}, ev.Data)
		})
	}
}

func TestService_Delete_MissingSheetEmitsNothing(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	pub := eventsmocks.NewMockPublisher(t)

	repo.EXPECT().FetchByID(ctx, missingSheetID).Return(nil, sheet.ErrSheetNotFound).Once()

	svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, pub)
	err := svc.Delete(ctx, sheet.DeleteSheetCommand{SheetID: missingSheetID, OwnerID: ownerUserID})

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	pub.AssertNotCalled(t, "Publish", mock.Anything)
}
