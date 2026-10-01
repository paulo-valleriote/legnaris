package sheet_test

import (
	"context"
	"testing"
	"time"

	"legnaris/internal/engine"
	"legnaris/internal/sheet"
	"legnaris/internal/sheet/mocks"
	"legnaris/internal/sheet/sheettest"
	"legnaris/internal/sheet/template"
	templatemocks "legnaris/internal/sheet/template/mocks"
	"legnaris/shared/apperr"
	"legnaris/shared/idgen"
	"legnaris/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Canonical UUID v7 fixtures: sheet IDs now pass idgen.Validate at the
// service boundary (LGN-86), so any test that must reach the repository needs
// an ID in the exact shape New produces.
const (
	fetchSheetID     = "01900000-0000-7000-8000-000000000001"
	missingSheetID   = "01900000-0000-7000-8000-000000000404"
	updateSheetID    = "01900000-0000-7000-8000-000000000123"
	corruptedSheetID = "01900000-0000-7000-8000-00000000c0de"
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
			"proficiencyBonus": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 2}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
			"constitutionMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_constitution - 10) / 2)",
				DependsOn:  [][]string{{"stats", "constitution"}},
			}},
		}},
		"config": {Children: template.FieldGroup{
			"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 1}},
		}},
		"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: template.FieldGroup{
			"armorClass": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "baseArmorClass + (modifiers_dexterityMod * config_acUsesDex)",
				DependsOn:  [][]string{{"baseArmorClass"}, {"modifiers", "dexterityMod"}, {"config", "acUsesDex"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, formulaEngine)
	require.NoError(t, err)

	dagCache := sheettest.DAGMap{
		"fantasy_v1": dag,
	}

	sheetID := updateSheetID

	nestedStoredData := map[string]any{
		"stats": map[string]any{
			"dexterity":    14.0,
			"constitution": 12.0,
		},
		"others": map[string]any{
			"proficiencyBonus": 2.0,
		},
		"modifiers": map[string]any{
			"dexterityMod":    2.0,
			"constitutionMod": 1.0,
		},
		"config": map[string]any{
			"acUsesDex": 1.0,
		},
		"baseArmorClass": 10.0,
		"derived": map[string]any{
			"armorClass": 12.0,
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
						if !ok || modifiers["dexterityMod"] != 4.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armorClass"] != 14.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterityMod": 4.0},
				"derived":   map[string]any{"armorClass": 14.0},
			},
		},
		{
			name: "Nested map with multiple groups (user example)",
			updates: map[string]any{
				"others": map[string]any{"proficiencyBonus": 4.0},
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
						if !ok || others["proficiencyBonus"] != 4.0 {
							return false
						}
						modifiers, ok := data["modifiers"].(map[string]any)
						if !ok || modifiers["dexterityMod"] != 1.0 || modifiers["constitutionMod"] != 2.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armorClass"] != 11.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterityMod": 1.0, "constitutionMod": 2.0},
				"derived":   map[string]any{"armorClass": 11.0},
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
			expectedErr: "field does not exist",
		},
		{
			// The DAG accepts a group prefix, an update payload must not:
			// applying it would replace the whole group with the scalar and
			// drop its siblings.
			name: "Error: group key is not a writable field",
			updates: map[string]any{
				"stats": 12.0,
			},
			setupMocks: func(m *mocks.MockRepository) {
				m.EXPECT().
					FetchByID(context.Background(), sheetID).
					Return(storedSheet, nil).
					Once()
			},
			expectedErr: "field does not exist",
		},
		{
			name: "Error: Field not editable",
			updates: map[string]any{
				"modifiers": map[string]any{"dexterityMod": 5.0},
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
					Return(nil, sheet.ErrSheetNotFound).
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
						if !ok || modifiers["dexterityMod"] != 3.0 || modifiers["constitutionMod"] != 2.0 {
							return false
						}
						derived, ok := data["derived"].(map[string]any)
						if !ok || derived["armorClass"] != 13.0 {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			},
			expectedCascades: map[string]any{
				"modifiers": map[string]any{"dexterityMod": 3.0, "constitutionMod": 2.0},
				"derived":   map[string]any{"armorClass": 13.0},
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

// TestService_PartialUpdate_ExclusiveGroups covers the PATCH contract for
// mutually exclusive options (EXC-04, EXC-05, EXC-07): turning one member on
// zeroes its sibling and recomputes the dependent derived value; turning a
// member off leaves the sibling alone; two members on is a 400.
func TestService_PartialUpdate_ExclusiveGroups(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity":    {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"constitution": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type: "number", IsEditable: false,
				Formula:   "floor((stats_dexterity - 10) / 2)",
				DependsOn: [][]string{{"stats", "dexterity"}},
			}},
			"constitutionMod": {Schema: &template.FieldSchema{
				Type: "number", IsEditable: false,
				Formula:   "floor((stats_constitution - 10) / 2)",
				DependsOn: [][]string{{"stats", "constitution"}},
			}},
		}},
		"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"config": {Children: template.FieldGroup{
			"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 1, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 0, ExclusiveGroup: "armorClassAbility"}},
		}},
		"derived": {Children: template.FieldGroup{
			"armorClass": {Schema: &template.FieldSchema{
				Type: "number", IsEditable: false,
				Formula: "baseArmorClass + (modifiers_dexterityMod * config_acUsesDex) + (modifiers_constitutionMod * config_acUsesCon)",
				DependsOn: [][]string{
					{"baseArmorClass"},
					{"modifiers", "dexterityMod"},
					{"config", "acUsesDex"},
					{"modifiers", "constitutionMod"},
					{"config", "acUsesCon"},
				},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, formulaEngine)
	require.NoError(t, err)

	sheetID := updateSheetID

	// storedWith builds a sheet with dexMod 2 / conMod 1 and the given flag
	// pair; armorClass must match the pair (10 + 2*dex + 1*con).
	storedWith := func(acUsesDex, acUsesCon, armorClass float64) *sheet.Sheet {
		return &sheet.Sheet{
			ID:         sheetID,
			TemplateID: "fantasy_v1",
			Name:       "Exclusive Sheet",
			Data: map[string]any{
				"stats":          map[string]any{"dexterity": 14.0, "constitution": 12.0},
				"modifiers":      map[string]any{"dexterityMod": 2.0, "constitutionMod": 1.0},
				"baseArmorClass": 10.0,
				"config":         map[string]any{"acUsesDex": acUsesDex, "acUsesCon": acUsesCon},
				"derived":        map[string]any{"armorClass": armorClass},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	tests := []struct {
		name             string
		stored           *sheet.Sheet
		updates          map[string]any
		expectStore      bool
		wantStoredDex    float64
		wantStoredCon    float64
		expectedErr      string
		expectedCascades map[string]any
	}{
		{
			// Spec Independent Test: stored acUsesCon=1, PATCH acUsesDex=1.
			name:   "Happy path: turning dexterity on zeroes the constitution sibling and recomputes armor class",
			stored: storedWith(0, 1, 11),
			updates: map[string]any{
				"config": map[string]any{"acUsesDex": 1.0},
			},
			expectStore:   true,
			wantStoredDex: 1.0,
			wantStoredCon: 0.0,
			expectedCascades: map[string]any{
				"derived": map[string]any{"armorClass": 12.0},
			},
		},
		{
			name:   "Edge: zeroing a member leaves its sibling unchanged",
			stored: storedWith(0, 1, 11),
			updates: map[string]any{
				"config": map[string]any{"acUsesCon": 0.0},
			},
			expectStore:   true,
			wantStoredDex: 0.0,
			wantStoredCon: 0.0,
			expectedCascades: map[string]any{
				"derived": map[string]any{"armorClass": 10.0},
			},
		},
		{
			name:   "Error: turning both members on returns a conflict",
			stored: storedWith(1, 0, 12),
			updates: map[string]any{
				"config": map[string]any{"acUsesDex": 1.0, "acUsesCon": 1.0},
			},
			expectStore: false,
			expectedErr: "conflicting exclusive options",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			mockRepo.EXPECT().
				FetchByID(context.Background(), sheetID).
				Return(tc.stored, nil).
				Once()

			if tc.expectStore {
				mockRepo.EXPECT().
					PartialUpdate(context.Background(), sheetID, mock.MatchedBy(func(update map[string]any) bool {
						data, ok := update["data"].(map[string]any)
						if !ok {
							return false
						}
						config, ok := data["config"].(map[string]any)
						if !ok {
							return false
						}
						if config["acUsesDex"] != tc.wantStoredDex || config["acUsesCon"] != tc.wantStoredCon {
							return false
						}
						_, hasTime := update["updated_at"].(time.Time)
						return hasTime
					})).
					Return(nil).
					Once()
			}

			svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{
				"fantasy_v1": dag,
			})

			cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
				SheetID: sheetID,
				Data:    tc.updates,
			})

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)

				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
				assert.Equal(t, 400, appErr.HttpStatus())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedCascades, cascades)
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
		"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: template.FieldGroup{
			"armorClass": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "baseArmorClass + mod_str",
				DependsOn:  [][]string{{"baseArmorClass"}, {"mod", "str"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	dagCache := sheettest.DAGMap{"test": dag}

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
			"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"derived": {Children: template.FieldGroup{
				"armorClass": {Schema: &template.FieldSchema{
					Type:       "number",
					IsEditable: false,
					Formula:    "baseArmorClass + mod_str",
					DependsOn:  [][]string{{"baseArmorClass"}, {"mod", "str"}},
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
			if !ok || derived["armorClass"] != 10.0 {
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
	// Round-trip: what Create mints must satisfy the validation every entry
	// point applies before the repository (LGN-86).
	require.NoError(t, idgen.Validate(s.ID))
	assert.Equal(t, "test", s.TemplateID)
}

// TestService_Create_SeedsTypedValues pins the type-aware seeding contract:
// a text leaf must reach the repository as an empty string (never the
// float64 default, which would store a number where the client expects
// text), number leaves keep their defaults, and derived leaves arrive
// computed by the Recalculate call Create runs over the seed.
func TestService_Create_SeedsTypedValues(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := template.FieldGroup{
		"identity": {Children: template.FieldGroup{
			"characterName": {Schema: &template.FieldSchema{Type: "text", Label: "Character name", IsEditable: true}},
		}},
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("test", fields, formulaEngine)
	require.NoError(t, err)

	tmpl := &template.Template{ID: "test", Name: "Test Template", Fields: fields}

	mockTemplateSvc := templatemocks.NewMockService(t)
	mockTemplateSvc.EXPECT().
		FetchTemplate(context.Background(), &template.FetchTemplateByIDCommand{ID: "test"}).
		Return(tmpl, nil).
		Once()

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(s *sheet.Sheet) bool {
			identity, ok := s.Data["identity"].(map[string]any)
			if !ok || identity["characterName"] != "" {
				return false
			}
			stats, ok := s.Data["stats"].(map[string]any)
			if !ok || stats["dexterity"] != 10.0 {
				return false
			}
			modifiers, ok := s.Data["modifiers"].(map[string]any)
			if !ok || modifiers["dexterityMod"] != 0.0 {
				return false
			}
			return true
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(mockRepo, mockTemplateSvc, sheettest.DAGMap{"test": dag})

	s, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: "test",
		SheetName:  "Seed Check",
	})
	require.NoError(t, err)
	assert.Equal(t, "test", s.TemplateID)
}

// TestService_Create_TemplateIDErrors: Create consults FetchTemplate first so
// the template-ID checks own both entry points (LGN-86 follow-up) — an empty
// or malformed template_id must surface as 400 INVALID_REQUEST through the
// service boundary instead of the 404 "template not found" the DAG lookup used
// to return, and the repository must never be reached.
func TestService_Create_TemplateIDErrors(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		sentinel error
		wantMsg  string
	}{
		{
			name:     "Edge: empty template ID",
			id:       "",
			sentinel: template.ErrTemplateIDNotProvided,
			wantMsg:  "template id not provided",
		},
		{
			name:     "Error: malformed template ID slug",
			id:       "Fantasy-V1",
			sentinel: template.ErrTemplateIDInvalid,
			wantMsg:  "template id is not a valid slug",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

			mockTemplateSvc := templatemocks.NewMockService(t)
			mockTemplateSvc.EXPECT().
				FetchTemplate(context.Background(), &template.FetchTemplateByIDCommand{ID: tc.id}).
				Return(nil, tc.sentinel).
				Once()

			svc := sheet.NewService(mockRepo, mockTemplateSvc, sheettest.DAGMap{})

			s, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
				TemplateID: tc.id,
				SheetName:  "New Character",
			})

			require.Error(t, err)
			assert.Nil(t, s)

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, tc.wantMsg, appErr.Message)
		})
	}
}

// TestService_PartialUpdate_InvalidStoredData locks the consolidated
// maputils behavior at the service boundary: when a stored sheet holds a
// scalar where a nested map is expected, the typed maputils.ErrNotMap must
// surface as a 400 apperr instead of the 500 INTERNAL_ERROR fallback.
func TestService_PartialUpdate_InvalidStoredData(t *testing.T) {
	formulaEngine := engine.NewFormulaEngine()
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, formulaEngine)
	require.NoError(t, err)

	sheetID := corruptedSheetID
	corrupted := &sheet.Sheet{
		ID:         sheetID,
		TemplateID: "fantasy_v1",
		Name:       "Corrupted Sheet",
		Data:       map[string]any{"stats": "not a map"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		FetchByID(context.Background(), sheetID).
		Return(corrupted, nil).
		Once()

	svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{
		"fantasy_v1": dag,
	})

	_, err = svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: sheetID,
		Data:    map[string]any{"stats": map[string]any{"dexterity": 18.0}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, maputils.ErrNotMap)
	assert.Contains(t, err.Error(), "invalid sheet data")

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, 400, appErr.HttpStatus())
}

// TestService_FetchByID pins the ID checks to the service (LGN-66, LGN-86):
// controllers forward the path param, so an empty ID must be rejected here —
// and a non-UUID must die with 400 before the repository — with the same
// error every caller gets. A valid but unknown UUID keeps returning 404.
func TestService_FetchByID(t *testing.T) {
	storedSheet := &sheet.Sheet{
		ID:         fetchSheetID,
		TemplateID: "fantasy_v1",
		Name:       "Fetched",
		Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	tests := []struct {
		name       string
		id         string
		repoSheet  *sheet.Sheet
		repoErr    error
		callRepo   bool
		wantCode   apperr.Code
		wantStatus int
		wantMsg    string
	}{
		{
			name:      "Happy path: returns the stored sheet",
			id:        fetchSheetID,
			repoSheet: storedSheet,
			callRepo:  true,
		},
		{
			name:       "Edge: empty ID is rejected before the repository",
			id:         "",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id not provided",
		},
		{
			name:       "Error: non-hex ID is rejected before the repository",
			id:         "01900000-0000-7000-800g-000000000001",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: wrong-length ID is rejected before the repository",
			id:         "01900000-0000-7000-8000-00000000000",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: UUID v4 (pre-LGN-86 format) is rejected before the repository",
			id:         "9f2c8a1b-3d4e-4f60-8a1b-2c3d4e5f6071",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: non-RFC 4122 variant is rejected before the repository",
			id:         "01900000-0000-7000-c000-000000000001",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: uppercase ID is rejected before the repository",
			id:         "01900000-0000-7000-8ABC-DEF012345678",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: urn:uuid encoding is rejected before the repository",
			id:         "urn:uuid:01900000-0000-7000-8000-000000000001",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: garbage string is rejected before the repository",
			id:         "not-a-uuid",
			callRepo:   false,
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
			wantMsg:    "sheet id must be a canonical UUID v7",
		},
		{
			name:       "Error: missing sheet maps ErrSheetNotFound to 404",
			id:         missingSheetID,
			repoErr:    sheet.ErrSheetNotFound,
			callRepo:   true,
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
			wantMsg:    "sheet not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t)
			if tc.callRepo {
				mockRepo.EXPECT().
					FetchByID(context.Background(), tc.id).
					Return(tc.repoSheet, tc.repoErr).
					Once()
			}

			svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{})

			got, err := svc.FetchByID(context.Background(), tc.id)

			if tc.wantCode == "" {
				require.NoError(t, err)
				assert.Equal(t, storedSheet, got)
				return
			}

			require.Error(t, err)
			assert.Nil(t, got)

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
			assert.Equal(t, tc.wantMsg, appErr.Message)
		})
	}
}

// TestService_PartialUpdate_MissingSheetID: PartialUpdate used to skip the ID
// check its FetchByID does, so an empty ID fell through to the repository and
// surfaced as 404 "sheet not found" instead of 400 (LGN-66).
func TestService_PartialUpdate_MissingSheetID(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

	svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{})

	cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: "",
		Data:    map[string]any{"stats": map[string]any{"dexterity": 12.0}},
	})

	require.Error(t, err)
	assert.Nil(t, cascades)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, 400, appErr.HttpStatus())
	assert.Equal(t, "sheet id not provided", appErr.Message)
}

// TestService_PartialUpdate_MalformedSheetID: a non-UUID sheet ID must fail
// with 400 INVALID_REQUEST before the repository on partial update too
// (LGN-86) — FetchByID is not the only entry point taking a sheet ID.
func TestService_PartialUpdate_MalformedSheetID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "Edge: uppercase non-canonical ID", id: "01900000-0000-7000-8ABC-DEF012345678"},
		{name: "Error: UUID v4 (pre-LGN-86 format)", id: "9f2c8a1b-3d4e-4f60-8a1b-2c3d4e5f6071"},
		{name: "Error: braced encoding", id: "{01900000-0000-7000-8000-000000000001}"},
		{name: "Error: garbage string", id: "not-a-uuid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

			svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{})

			cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
				SheetID: tc.id,
				Data:    map[string]any{"stats": map[string]any{"dexterity": 12.0}},
			})

			require.Error(t, err)
			assert.Nil(t, cascades)

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
			assert.Equal(t, 400, appErr.HttpStatus())
			assert.Equal(t, "sheet id must be a canonical UUID v7", appErr.Message)
		})
	}
}
