package sheet_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/internal/engine"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/mocks"
	"legnaris/internal/sheet/template"
	templatemocks "legnaris/internal/sheet/template/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_PartialUpdate(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockTemplateSvc := templatemocks.NewMockService(t)

	formulaEngine := engine.NewFormulaEngine()
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity":    {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"constitution": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"others": {Children: template.FieldGroup{
			"proficiency_bonus": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 2}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterity_mod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
			"constitution_mod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_constitution - 10) / 2)",
				DependsOn:  [][]string{{"stats", "constitution"}},
			}},
		}},
		"config": {Children: template.FieldGroup{
			"ac_uses_dex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 1}},
		}},
		"base_armor_class": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: template.FieldGroup{
			"armor_class": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "base_armor_class + (modifiers_dexterity_mod * config_ac_uses_dex)",
				DependsOn:  [][]string{{"base_armor_class"}, {"modifiers", "dexterity_mod"}, {"config", "ac_uses_dex"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, formulaEngine)
	require.NoError(t, err)

	dagCache := map[string]*template.DAG{
		"fantasy_v1": dag,
	}

	sheetID := "sheet-123"

	nestedStoredData := map[string]any{
		"stats": map[string]any{
			"dexterity":    14.0,
			"constitution": 12.0,
		},
		"others": map[string]any{
			"proficiency_bonus": 2.0,
		},
		"modifiers": map[string]any{
			"dexterity_mod":    2.0,
			"constitution_mod": 1.0,
		},
		"config": map[string]any{
			"ac_uses_dex": 1.0,
		},
		"base_armor_class": 10.0,
		"derived": map[string]any{
			"armor_class": 12.0,
		},
	}

	storedSheet := &sheet.Sheet{
		ID:         sheetID,
		TemplateID: "fantasy_v1",
		Name:       "Test Sheet",
		Data:       nestedStoredData,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	tests := []struct {
		name             string
		updates          map[string]any
		setupMocks       func(*mocks.MockRepository)
		expectedErr      string
		expectedCascades map[string]any
	}{
		{
			name: "Happy Path: Update editable field triggers cascade",
			updates: map[string]any{
				"stats": map[string]any{"dexterity": 18.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()

				m.EXPECT().
					PartialUpdate(context.Background(), sheetID, mock.MatchedBy(func(update map[string]any) bool {
						data, ok := update["data"].(map[string]any)
						if !ok {
							return false
						}
						stats, ok := data["stats"].(map[string]any)
						if !ok || stats["dexterity"] != 18.0 {
							return false
						}
						modifiers, ok := data["modifiers"].(map[string]any)
						if !ok || modifiers["dexterity_mod"] != 4.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armor_class"] != 14.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterity_mod": 4.0},
				"derived":   map[string]any{"armor_class": 14.0},
			},
		},
		{
			name: "Nested map with multiple groups (user example)",
			updates: map[string]any{
				"others": map[string]any{"proficiency_bonus": 4.0},
				"stats":  map[string]any{"constitution": 14.0, "dexterity": 12.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()

				m.EXPECT().
					PartialUpdate(context.Background(), sheetID, mock.MatchedBy(func(update map[string]any) bool {
						data, ok := update["data"].(map[string]any)
						if !ok {
							return false
						}
						stats, ok := data["stats"].(map[string]any)
						if !ok || stats["dexterity"] != 12.0 || stats["constitution"] != 14.0 {
							return false
						}
						others, ok := data["others"].(map[string]any)
						if !ok || others["proficiency_bonus"] != 4.0 {
							return false
						}
						modifiers, ok := data["modifiers"].(map[string]any)
						if !ok || modifiers["dexterity_mod"] != 1.0 || modifiers["constitution_mod"] != 2.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armor_class"] != 11.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterity_mod": 1.0, "constitution_mod": 2.0},
				"derived":   map[string]any{"armor_class": 11.0},
			},
		},
		{
			name: "Error: Field not in template",
			updates: map[string]any{
				"stats": map[string]any{"unknown": 10.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()
			},
			expectedErr: "field not exists",
		},
		{
			name: "Error: Field not editable",
			updates: map[string]any{
				"modifiers": map[string]any{"dexterity_mod": 5.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()
			},
			expectedErr: "field is not editable",
		},
		{
			name: "Error: Sheet not found",
			updates: map[string]any{
				"stats": map[string]any{"dexterity": 18.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(nil, errors.New("sheet not found")).
					Once()
			},
			expectedErr: "sheet not found",
		},
		{
			name: "Multiple field updates with cascade",
			updates: map[string]any{
				"stats": map[string]any{"dexterity": 16.0, "constitution": 14.0},
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()

				m.EXPECT().
					PartialUpdate(context.Background(), sheetID, mock.MatchedBy(func(update map[string]any) bool {
						data, ok := update["data"].(map[string]any)
						if !ok {
							return false
						}
						stats, ok := data["stats"].(map[string]any)
						if !ok || stats["dexterity"] != 16.0 || stats["constitution"] != 14.0 {
							return false
						}
						modifiers, ok := data["modifiers"].(map[string]any)
						if !ok || modifiers["dexterity_mod"] != 3.0 || modifiers["constitution_mod"] != 2.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armor_class"] != 13.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterity_mod": 3.0, "constitution_mod": 2.0},
				"derived":   map[string]any{"armor_class": 13.0},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo = mocks.NewMockRepository(t)
			mockTemplateSvc = templatemocks.NewMockService(t)
			svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

			tc.setupMocks(mockRepo)

			cmd := sheet.PartialUpdateSheetCommand{
				SheetID: sheetID,
				Data:    tc.updates,
			}

			cascades, err := svc.PartialUpdate(context.Background(), cmd)

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedCascades, cascades)
			}
		})
	}
}

func TestService_Create(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t)
	mockTemplateSvc := templatemocks.NewMockService(t)

	formulaEngine := engine.NewFormulaEngine()
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"str": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"mod": {Children: template.FieldGroup{
			"str": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_str - 10) / 2)",
				DependsOn:  [][]string{{"stats", "str"}},
			}},
		}},
		"base_armor_class": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: template.FieldGroup{
			"armor_class": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "base_armor_class + mod_str",
				DependsOn:  [][]string{{"base_armor_class"}, {"mod", "str"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	dagCache := map[string]*template.DAG{"test": dag}

	svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

	tmpl := &template.Template{
		ID:   "test",
		Name: "Test Template",
		Fields: template.FieldGroup{
			"stats": {Children: template.FieldGroup{
				"str": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			}},
			"mod": {Children: template.FieldGroup{
				"str": {Schema: &template.FieldSchema{
					Type:       "number",
					IsEditable: false,
					Formula:    "floor((stats_str - 10) / 2)",
					DependsOn:  [][]string{{"stats", "str"}},
				}},
			}},
			"base_armor_class": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"derived": {Children: template.FieldGroup{
				"armor_class": {Schema: &template.FieldSchema{
					Type:       "number",
					IsEditable: false,
					Formula:    "base_armor_class + mod_str",
					DependsOn:  [][]string{{"base_armor_class"}, {"mod", "str"}},
				}},
			}},
		},
	}

	mockTemplateSvc.EXPECT().
		FetchTemplate(context.Background(), &template.FetchTemplateByIDCommand{ID: "test"}).
		Return(tmpl, nil).
		Once()

	mockRepo.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(s *sheet.Sheet) bool {
			stats, ok := s.Data["stats"].(map[string]any)
			if !ok || stats["str"] != 10.0 {
				return false
			}
			mod, ok := s.Data["mod"].(map[string]any)
			if !ok || mod["str"] != 0.0 {
				return false
			}
			derived, ok := s.Data["derived"].(map[string]any)
			if !ok || derived["armor_class"] != 10.0 {
				return false
			}
			return s.TemplateID == "test" && s.Name == "New Character"
		})).
		Return(nil).
		Once()

	cmd := sheet.CreateSheetCommand{
		TemplateID: "test",
		SheetName:  "New Character",
	}

	s, err := svc.Create(context.Background(), cmd)
	require.NoError(t, err)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "test", s.TemplateID)
}
