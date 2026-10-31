package sheet_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const layoutTemplateID = "layout_v1"

func layoutTemplate() template.Template {
	return template.Template{
		ID:   layoutTemplateID,
		Name: "Layout",
		Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{
				"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
				"strength":  {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			}},
			"modifiers": {Children: template.FieldGroup{
				"dexterityMod": {Schema: &template.FieldSchema{
					Type:      "number",
					Formula:   "floor((stats_dexterity - 10) / 2)",
					DependsOn: [][]string{{"stats", "dexterity"}},
				}},
			}},
			"skills": {Children: template.FieldGroup{
				"acrobatics": {
					Children: template.FieldGroup{
						"bonus": {Schema: &template.FieldSchema{Type: "number", IsEditable: true}},
						"total": {Schema: &template.FieldSchema{
							Type:      "number",
							Formula:   "skills_acrobatics_bonus * 2",
							DependsOn: [][]string{{"skills", "acrobatics", "bonus"}},
						}},
					},
					Columns: []string{"bonus", "total"},
				},
			}},
			"config": {Children: template.FieldGroup{
				"acUsesCon": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility"}},
				"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, ExclusiveGroup: "armorClassAbility", Default: 1}},
			}},
			"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		},
		SectionOrder: []string{"stats", "modifiers", "skills", "config", "baseArmorClass"},
	}
}

func layoutStoredSheet(ownerID string) *sheet.Sheet {
	return &sheet.Sheet{
		ID:         fetchSheetID,
		TemplateID: layoutTemplateID,
		OwnerID:    ownerID,
		Name:       "Layout sheet",
		Data: map[string]any{
			"stats":          map[string]any{"dexterity": 14.0, "strength": 10.0},
			"modifiers":      map[string]any{"dexterityMod": 2.0},
			"skills":         map[string]any{"acrobatics": map[string]any{"bonus": 3.0, "total": 6.0}},
			"config":         map[string]any{"acUsesCon": 0.0, "acUsesDex": 1.0},
			"baseArmorClass": 10.0,
		},
	}
}

func layoutErrCode(t *testing.T, err error) (apperr.Code, string) {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code, appErr.Message
}

func TestService_SetLayout(t *testing.T) {
	tmpl := layoutTemplate()
	fullLayout := &sheet.Layout{
		SectionOrder: []string{"config", "stats"},
		Pinned:       []string{"skills.acrobatics", "stats.strength", "config#armorClassAbility"},
	}
	templateOrder := &sheet.Layout{
		SectionOrder: []string{"stats", "modifiers", "skills", "config", "baseArmorClass"},
	}

	tests := []struct {
		name         string
		ownerID      string
		sheetID      string
		layout       *sheet.Layout
		stored       *sheet.Sheet
		fetchErr     error
		templateErr  error
		updateErr    error
		wantStored   *sheet.Layout
		wantUpdate   bool
		wantFetch    bool
		wantTemplate bool
		wantCode     apperr.Code
		wantMessage  string
		wantStatus   int
	}{
		{
			name:        "Error: missing owner is unauthorized and touches nothing",
			sheetID:     fetchSheetID,
			layout:      fullLayout,
			wantCode:    apperr.CodeUnauthorized,
			wantMessage: sheet.MsgOwnerNotProvided,
			wantStatus:  401,
		},
		{
			name:        "Error: missing sheet id",
			ownerID:     ownerUserID,
			layout:      fullLayout,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgSheetIDNotProvided,
			wantStatus:  400,
		},
		{
			name:        "Error: malformed sheet id",
			ownerID:     ownerUserID,
			sheetID:     "not-a-uuid",
			layout:      fullLayout,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgSheetIDInvalid,
			wantStatus:  400,
		},
		{
			name:        "Error: sheet does not exist",
			ownerID:     ownerUserID,
			sheetID:     missingSheetID,
			layout:      fullLayout,
			fetchErr:    sheet.ErrSheetNotFound,
			wantFetch:   true,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "sheet not found",
			wantStatus:  404,
		},
		{
			name:        "Error: foreign sheet is the same not found as a missing one",
			ownerID:     ownerUserID,
			sheetID:     fetchSheetID,
			layout:      fullLayout,
			stored:      layoutStoredSheet("someone-else"),
			wantFetch:   true,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "sheet not found",
			wantStatus:  404,
		},
		{
			name:        "Error: legacy sheet without owner is not found",
			ownerID:     ownerUserID,
			sheetID:     fetchSheetID,
			layout:      fullLayout,
			stored:      layoutStoredSheet(""),
			wantFetch:   true,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "sheet not found",
			wantStatus:  404,
		},
		{
			name:        "Error: repository failure on fetch is internal",
			ownerID:     ownerUserID,
			sheetID:     fetchSheetID,
			layout:      fullLayout,
			fetchErr:    errors.New("boom"),
			wantFetch:   true,
			wantCode:    apperr.CodeInternal,
			wantMessage: "failed to fetch sheet",
			wantStatus:  500,
		},
		{
			name:         "Error: template of the sheet is gone",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       fullLayout,
			stored:       layoutStoredSheet(ownerUserID),
			templateErr:  apperr.NewError(apperr.CodeNotFound, "template not found", nil),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeNotFound,
			wantMessage:  "template not found",
			wantStatus:   404,
		},
		{
			name:         "Error: unknown section in section_order",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{SectionOrder: []string{"stats", "ghost"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid layout: unknown layout section 'ghost'",
			wantStatus:   400,
		},
		{
			name:         "Error: duplicate section in section_order",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{SectionOrder: []string{"stats", "stats"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid layout: duplicate key 'stats' in section_order",
			wantStatus:   400,
		},
		{
			name:         "Error: pinned key that is not a row of any section",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{Pinned: []string{"skills.acrobatics.bonus"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid layout: unknown pinned row 'skills.acrobatics.bonus'",
			wantStatus:   400,
		},
		{
			name:         "Error: pinned key from a column path",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{Pinned: []string{"config.acUsesDex"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid layout: unknown pinned row 'config.acUsesDex'",
			wantStatus:   400,
		},
		{
			name:         "Error: duplicate pinned key",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{Pinned: []string{"stats.strength", "stats.strength"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid layout: duplicate key 'stats.strength' in pinned",
			wantStatus:   400,
		},
		{
			name:         "Error: sheet deleted between fetch and write is not found",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       fullLayout,
			stored:       layoutStoredSheet(ownerUserID),
			updateErr:    sheet.ErrSheetNotFound,
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   fullLayout,
			wantCode:     apperr.CodeNotFound,
			wantMessage:  "sheet not found",
			wantStatus:   404,
		},
		{
			name:         "Error: repository failure on write is internal",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       fullLayout,
			stored:       layoutStoredSheet(ownerUserID),
			updateErr:    errors.New("boom"),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   fullLayout,
			wantCode:     apperr.CodeInternal,
			wantMessage:  "failed to update sheet",
			wantStatus:   500,
		},
		{
			name:       "Edge: nil layout resets without consulting the template",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			layout:     nil,
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
		},
		{
			name:       "Edge: empty layout object resets",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			layout:     &sheet.Layout{},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
		},
		{
			name:       "Edge: layout of empty lists resets",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			layout:     &sheet.Layout{SectionOrder: []string{}, Pinned: []string{}},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
		},
		{
			name:         "Edge: order identical to the template is stored as given",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       templateOrder,
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   templateOrder,
		},
		{
			name:         "Edge: partial layout with only pins is valid",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       &sheet.Layout{Pinned: []string{"config#armorClassAbility"}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   &sheet.Layout{Pinned: []string{"config#armorClassAbility"}},
		},
		{
			name:         "Happy path: full layout is validated and stored",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			layout:       fullLayout,
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   fullLayout,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			templateSvc := templatemocks.NewMockService(t)
			svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{})

			if tc.wantFetch {
				call := repo.EXPECT().FetchByID(mock.Anything, tc.sheetID)
				if tc.stored != nil {
					call.Return(tc.stored, nil).Once()
				} else {
					call.Return(nil, tc.fetchErr).Once()
				}
			}
			if tc.wantTemplate {
				call := templateSvc.EXPECT().FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: layoutTemplateID})
				if tc.templateErr != nil {
					call.Return(nil, tc.templateErr).Once()
				} else {
					call.Return(&tmpl, nil).Once()
				}
			}
			if tc.wantUpdate {
				repo.EXPECT().UpdateLayout(mock.Anything, tc.sheetID, tc.wantStored).Return(tc.updateErr).Once()
			}

			got, err := svc.SetLayout(context.Background(), sheet.SetLayoutCommand{
				SheetID: tc.sheetID,
				OwnerID: tc.ownerID,
				Layout:  tc.layout,
			})

			if tc.wantCode != "" {
				require.Error(t, err)
				code, message := layoutErrCode(t, err)
				assert.Equal(t, tc.wantCode, code)
				assert.Equal(t, tc.wantMessage, message)
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantStored, got)
		})
	}
}

func TestService_SetLayout_ReturnedLayoutDoesNotAliasTheCommand(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{})

	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
	repo.EXPECT().UpdateLayout(mock.Anything, fetchSheetID, mock.Anything).Return(nil).Once()

	cmdLayout := &sheet.Layout{Pinned: []string{"stats.strength"}}
	got, err := svc.SetLayout(context.Background(), sheet.SetLayoutCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Layout: cmdLayout})
	require.NoError(t, err)

	cmdLayout.Pinned[0] = "mutated"
	assert.Equal(t, []string{"stats.strength"}, got.Pinned)
}

func TestService_SetLayout_IsIdempotent(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{})

	layout := &sheet.Layout{SectionOrder: []string{"config"}, Pinned: []string{"stats.strength"}}
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Twice()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Twice()
	repo.EXPECT().UpdateLayout(mock.Anything, fetchSheetID, layout).Return(nil).Twice()

	cmd := sheet.SetLayoutCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Layout: layout}
	first, err := svc.SetLayout(context.Background(), cmd)
	require.NoError(t, err)
	second, err := svc.SetLayout(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestService_SetLayout_LeavesRecalculationUntouched(t *testing.T) {
	tmpl := layoutTemplate()
	dag, err := template.BuildDAG(layoutTemplateID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	stored := layoutStoredSheet(ownerUserID)
	flatBefore, err := maputils.FlattenPathKeys(stored.Data)
	require.NoError(t, err)
	updates := map[string]any{"stats.dexterity": 18.0, "skills.acrobatics.bonus": 5.0}

	_, cascadeBefore, err := dag.NormalizeAndRecalculate(stored.Data, updates)
	require.NoError(t, err)

	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{layoutTemplateID: dag})

	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(stored, nil).Once()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
	repo.EXPECT().UpdateLayout(mock.Anything, fetchSheetID, mock.Anything).Return(nil).Once()

	_, err = svc.SetLayout(context.Background(), sheet.SetLayoutCommand{
		SheetID: fetchSheetID,
		OwnerID: ownerUserID,
		Layout:  &sheet.Layout{SectionOrder: []string{"config", "stats"}, Pinned: []string{"stats.dexterity"}},
	})
	require.NoError(t, err)

	flatAfter, err := maputils.FlattenPathKeys(stored.Data)
	require.NoError(t, err)
	assert.Equal(t, flatBefore, flatAfter, "a layout write must not change the stored values")

	_, cascadeAfter, err := dag.NormalizeAndRecalculate(stored.Data, updates)
	require.NoError(t, err)
	assert.Equal(t, cascadeBefore, cascadeAfter, "the DAG result must not depend on a layout write")
	assert.Equal(t, map[string]any{"modifiers.dexterityMod": 4.0, "skills.acrobatics.total": 10.0}, cascadeAfter)
}

func TestService_PartialUpdate_DoesNotWriteOrReadLayout(t *testing.T) {
	tmpl := layoutTemplate()
	dag, err := template.BuildDAG(layoutTemplateID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	stored := layoutStoredSheet(ownerUserID)
	stored.Layout = &sheet.Layout{Pinned: []string{"stats.strength"}}

	repo := mocks.NewMockRepository(t)
	svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{layoutTemplateID: dag})

	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(stored, nil).Once()
	repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, mock.MatchedBy(func(update map[string]any) bool {
		_, hasLayout := update["layout"]
		_, hasData := update["data"]
		return !hasLayout && hasData && len(update) == 2
	})).Return(nil).Once()

	_, err = svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: fetchSheetID,
		OwnerID: ownerUserID,
		Data:    map[string]any{"stats": map[string]any{"dexterity": 18.0}},
	})
	require.NoError(t, err)
	assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.strength"}}, stored.Layout)
}
