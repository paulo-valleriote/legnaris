package sheet_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
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

const (
	recordRowTemplateID = "record_row_v1"
	recordRowID         = "r0123456789abcdef0123456789abcdef"
)

func recordRowTemplate() template.Template {
	return template.Template{
		ID:   recordRowTemplateID,
		Name: "Record Rows",
		Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{
				"strength": {Schema: &template.FieldSchema{Type: "number", IsEditable: true}},
			}},
			"inventory": {Children: template.FieldGroup{
				"items": {List: &template.ListSchema{
					Label:    "Items",
					Capacity: 2,
					Dice:     "1d20 + @.quantity",
					Shape: template.FieldGroup{
						"name":     {Schema: &template.FieldSchema{Type: "text", IsEditable: true}},
						"quantity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true}},
					},
				}},
			}},
		},
		SectionOrder: []string{"stats", "inventory"},
	}
}

func recordRowDAGs(t *testing.T, tmpl template.Template) sheettest.DAGMap {
	t.Helper()
	dag, err := template.BuildDAG(tmpl.ID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheettest.DAGMap{tmpl.ID: dag}
}

func recordRowStoredSheet(ownerID string) *sheet.Sheet {
	return &sheet.Sheet{
		ID:         fetchSheetID,
		TemplateID: recordRowTemplateID,
		OwnerID:    ownerID,
		Name:       "Record row sheet",
		Data: map[string]any{
			"stats":     map[string]any{"strength": 10.0},
			"inventory": map[string]any{"items": map[string]any{}},
		},
	}
}

func TestService_SetLayout_RecordRowPin(t *testing.T) {
	tmpl := recordRowTemplate()
	recordRow := "inventory.items." + recordRowID

	tests := []struct {
		name    string
		pinned  string
		wantErr bool
	}{
		{
			name:   "Edge: a pin left by a removed Record still saves",
			pinned: recordRow,
		},
		{
			name:    "Error: a malformed Record id is refused",
			pinned:  "inventory.items.rbad",
			wantErr: true,
		},
		{
			name:    "Error: an unknown list is refused",
			pinned:  "unknown.items." + recordRowID,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			templateSvc := templatemocks.NewMockService(t)
			svc := sheet.NewService(repo, templateSvc, recordRowDAGs(t, tmpl), testLog, events.NoOp{})
			layout := &sheet.Layout{Pinned: []string{tc.pinned}}

			repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(recordRowStoredSheet(ownerUserID), nil).Once()
			templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
			if !tc.wantErr {
				repo.EXPECT().UpdateLayout(mock.Anything, fetchSheetID, layout).Return(nil).Once()
			}

			got, err := svc.SetLayout(context.Background(), sheet.SetLayoutCommand{
				SheetID: fetchSheetID,
				OwnerID: ownerUserID,
				Layout:  layout,
			})

			if tc.wantErr {
				require.Error(t, err)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, layout, got)
		})
	}
}

func TestService_PartialUpdate_RecordRowDice(t *testing.T) {
	tmpl := recordRowTemplate()
	recordRow := "inventory.items." + recordRowID

	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "Happy path: a Record Row override fits the list shape",
			expression: "1d20 + @.quantity",
		},
		{
			name:       "Edge: a Record Row left by a removed Record still saves",
			expression: "1d6",
		},
		{
			name:       "Error: an unknown shape column is refused",
			expression: "1d20 + @.missing",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			templateSvc := templatemocks.NewMockService(t)
			svc := sheet.NewService(repo, templateSvc, recordRowDAGs(t, tmpl), testLog, events.NoOp{})
			dice := &sheet.Dice{Row: map[string]*string{recordRow: text(tc.expression)}}

			repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(recordRowStoredSheet(ownerUserID), nil).Once()
			templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
			if !tc.wantErr {
				repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, int64(0), diceOnlyUpdate(dice)).
					Return(int64(1), nil).Once()
			}

			got, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
				SheetID: fetchSheetID,
				OwnerID: ownerUserID,
				Dice:    dice,
			})

			if tc.wantErr {
				require.Error(t, err)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, map[string]any{}, got)
		})
	}
}
