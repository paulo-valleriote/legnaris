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
	ownerUserID      = "01900000-0000-7000-8000-0000000000aa"
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
		OwnerID:    ownerUserID,
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
				OwnerID: ownerUserID,
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

// TestService_PartialUpdate_NoOpDeltaPersistsAndReturnsEmptyCascade locks the
// split the shared recalculation helper introduces: a delta that leaves every
// value unchanged returns an empty cascade, but PATCH must still persist the
// merged data and advance updated_at. A wrong refactor that skipped persistence
// when the cascade is empty would pass the cascade assertion and fail here.
func TestService_PartialUpdate_NoOpDeltaPersistsAndReturnsEmptyCascade(t *testing.T) {
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

	dag, err := template.BuildDAG("fantasy_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	stored := &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: "fantasy_v1",
		OwnerID:    ownerUserID,
		Name:       "No-op Sheet",
		Data: map[string]any{
			"stats":     map[string]any{"dexterity": 14.0},
			"modifiers": map[string]any{"dexterityMod": 2.0},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		FetchByID(context.Background(), updateSheetID).
		Return(stored, nil).
		Once()
	mockRepo.EXPECT().
		PartialUpdate(context.Background(), updateSheetID, mock.MatchedBy(func(update map[string]any) bool {
			data, ok := update["data"].(map[string]any)
			if !ok {
				return false
			}
			stats, ok := data["stats"].(map[string]any)
			if !ok || stats["dexterity"] != 14.0 {
				return false
			}
			modifiers, ok := data["modifiers"].(map[string]any)
			if !ok || modifiers["dexterityMod"] != 2.0 {
				return false
			}
			_, hasTime := update["updated_at"].(time.Time)
			return hasTime
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{"fantasy_v1": dag})

	cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: updateSheetID,
		OwnerID: ownerUserID,
		Data:    map[string]any{"stats": map[string]any{"dexterity": 14.0}},
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, cascades)
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
			OwnerID:    ownerUserID,
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
		{
			name:   "Error: a non-numeric member is rejected",
			stored: storedWith(1, 0, 12),
			updates: map[string]any{
				"config": map[string]any{"acUsesDex": "1"},
			},
			expectStore: false,
			expectedErr: "exclusive option must be numeric",
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
				OwnerID: ownerUserID,
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

// TestService_PartialUpdate_NestingDepthLimit pins the DoS guard at the service
// boundary: a payload nested deeper than maputils.MaxNestingDepth is rejected
// with 400 before the repository is touched, so the recursive flatten never
// walks an unbounded structure.
func TestService_PartialUpdate_NestingDepthLimit(t *testing.T) {
	dag, err := template.BuildDAG("fantasy_v1", template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
	}, engine.NewFormulaEngine())
	require.NoError(t, err)

	deep := map[string]any{"leaf": 1.0}
	for i := 0; i < maputils.MaxNestingDepth+1; i++ {
		deep = map[string]any{"level": deep}
	}

	mockRepo := mocks.NewMockRepository(t)
	mockRepo.EXPECT().
		FetchByID(context.Background(), updateSheetID).
		Return(&sheet.Sheet{
			ID:         updateSheetID,
			TemplateID: "fantasy_v1",
			OwnerID:    ownerUserID,
			Data:       map[string]any{"stats": map[string]any{"dexterity": 10.0}},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}, nil).
		Once()

	svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{
		"fantasy_v1": dag,
	})

	_, err = svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: updateSheetID,
		OwnerID: ownerUserID,
		Data:    deep,
	})
	require.Error(t, err)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, 400, appErr.HttpStatus())
	assert.Contains(t, appErr.Message, "nested too deeply")
}

// TestService_Preview pins the stateless dry-run (DRY-01…DRY-12): it returns
// the same cascade as an equivalent PATCH, never writes, is idempotent, returns
// an empty map for an empty delta, and maps every reject path to the same typed
// error PATCH produces.
func TestService_Preview(t *testing.T) {
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity":    {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
			"constitution": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
		"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"config": {Children: template.FieldGroup{
			"acUsesDex": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 1, ExclusiveGroup: "armorClassAbility"}},
			"acUsesCon": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 0, ExclusiveGroup: "armorClassAbility"}},
		}},
		"derived": {Children: template.FieldGroup{
			"armorClass": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "baseArmorClass + modifiers_dexterityMod",
				DependsOn:  [][]string{{"baseArmorClass"}, {"modifiers", "dexterityMod"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("fantasy_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	dagCache := sheettest.DAGMap{"fantasy_v1": dag}

	storedSheet := func() *sheet.Sheet {
		return &sheet.Sheet{
			ID:         updateSheetID,
			TemplateID: "fantasy_v1",
			OwnerID:    ownerUserID,
			Name:       "Preview Sheet",
			Data: map[string]any{
				"stats":          map[string]any{"dexterity": 14.0, "constitution": 12.0},
				"modifiers":      map[string]any{"dexterityMod": 2.0},
				"baseArmorClass": 10.0,
				"config":         map[string]any{"acUsesDex": 1.0, "acUsesCon": 0.0},
				"derived":        map[string]any{"armorClass": 12.0},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	ctx := context.Background()

	t.Run("Happy path: valid delta returns the cascade and issues no write", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()

		svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID,
			OwnerID: ownerUserID,
			Data:    map[string]any{"stats": map[string]any{"dexterity": 18.0}},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"modifiers": map[string]any{"dexterityMod": 4.0},
			"derived":   map[string]any{"armorClass": 14.0},
		}, cascades)
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Happy path: preview equals an equivalent PATCH cascade", func(t *testing.T) {
		previewRepo := mocks.NewMockRepository(t)
		previewRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()
		previewSvc := sheet.NewService(previewRepo, templatemocks.NewMockService(t), dagCache)

		patchRepo := mocks.NewMockRepository(t)
		patchRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()
		patchRepo.EXPECT().PartialUpdate(ctx, updateSheetID, mock.Anything).Return(nil).Once()
		patchSvc := sheet.NewService(patchRepo, templatemocks.NewMockService(t), dagCache)

		delta := map[string]any{"stats": map[string]any{"dexterity": 16.0}}

		previewCascade, err := previewSvc.Preview(ctx, sheet.PartialUpdateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta})
		require.NoError(t, err)
		patchCascade, err := patchSvc.PartialUpdate(ctx, sheet.PartialUpdateSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta})
		require.NoError(t, err)

		assert.Equal(t, patchCascade, previewCascade)
	})

	t.Run("Happy path: repeating the preview returns the same cascade", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Twice()

		svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)
		cmd := sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID,
			OwnerID: ownerUserID,
			Data:    map[string]any{"stats": map[string]any{"dexterity": 18.0}},
		}

		first, err := svc.Preview(ctx, cmd)
		require.NoError(t, err)
		second, err := svc.Preview(ctx, cmd)
		require.NoError(t, err)

		assert.Equal(t, first, second)
	})

	t.Run("Edge: empty delta returns an empty cascade and issues no write", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()

		svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID,
			OwnerID: ownerUserID,
			Data:    map[string]any{},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, cascades)
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: missing or malformed sheet id returns 400 before the repository", func(t *testing.T) {
		tests := []struct {
			name    string
			id      string
			wantMsg string
		}{
			{name: "Edge: missing ID", id: "", wantMsg: sheet.MsgSheetIDNotProvided},
			{name: "Error: malformed ID", id: "not-a-uuid", wantMsg: sheet.MsgSheetIDInvalid},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called
				svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

				cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
					SheetID: tc.id,
					OwnerID: ownerUserID,
					Data:    map[string]any{"stats": map[string]any{"dexterity": 12.0}},
				})

				require.Error(t, err)
				assert.Nil(t, cascades)

				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
				assert.Equal(t, 400, appErr.HttpStatus())
				assert.Equal(t, tc.wantMsg, appErr.Message)
			})
		}
	})

	t.Run("Error: unknown sheet returns 404 NOT_FOUND", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(ctx, missingSheetID).Return(nil, sheet.ErrSheetNotFound).Once()

		svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
			SheetID: missingSheetID,
			OwnerID: ownerUserID,
			Data:    map[string]any{"stats": map[string]any{"dexterity": 12.0}},
		})

		require.Error(t, err)
		assert.Nil(t, cascades)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		assert.Equal(t, 404, appErr.HttpStatus())
		assert.Equal(t, "sheet not found", appErr.Message)
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: DAG rejections map to the same codes PATCH returns", func(t *testing.T) {
		tests := []struct {
			name     string
			updates  map[string]any
			wantCode apperr.Code
			wantMsg  string
		}{
			{
				name:     "Error: unknown field returns 400 UNKNOWN_FIELD",
				updates:  map[string]any{"stats": map[string]any{"unknown": 1.0}},
				wantCode: apperr.CodeUnknownField,
				wantMsg:  "field does not exist: stats.unknown",
			},
			{
				name:     "Error: derived field returns 400 INVALID_REQUEST",
				updates:  map[string]any{"modifiers": map[string]any{"dexterityMod": 5.0}},
				wantCode: apperr.CodeInvalidRequest,
				wantMsg:  "field is not editable: modifiers.dexterityMod",
			},
			{
				name:     "Error: exclusive group conflict returns 400 INVALID_REQUEST",
				updates:  map[string]any{"config": map[string]any{"acUsesDex": 1.0, "acUsesCon": 1.0}},
				wantCode: apperr.CodeInvalidRequest,
				wantMsg:  "conflicting exclusive options: armorClassAbility",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRepo := mocks.NewMockRepository(t)
				mockRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()

				svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

				cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
					SheetID: updateSheetID,
					OwnerID: ownerUserID,
					Data:    tc.updates,
				})

				require.Error(t, err)
				assert.Nil(t, cascades)

				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
				assert.Equal(t, 400, appErr.HttpStatus())
				assert.Equal(t, tc.wantMsg, appErr.Message)
				mockRepo.AssertNotCalled(t, "PartialUpdate")
			})
		}
	})

	t.Run("Error: over-deep nesting returns 400 INVALID_REQUEST", func(t *testing.T) {
		deep := map[string]any{"leaf": 1.0}
		for i := 0; i < maputils.MaxNestingDepth+1; i++ {
			deep = map[string]any{"level": deep}
		}

		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().FetchByID(ctx, updateSheetID).Return(storedSheet(), nil).Once()

		svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID,
			OwnerID: ownerUserID,
			Data:    deep,
		})

		require.Error(t, err)
		assert.Nil(t, cascades)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		assert.Equal(t, 400, appErr.HttpStatus())
		assert.Contains(t, appErr.Message, "nested too deeply")
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})
}

// TestService_PreviewTemplate pins the template dry-run (DRY-13…DRY-17): a
// valid delta returns the cascade computed over the template's seeded fields,
// the seed matches what Create stores, missing/malformed ids are 400, an
// unknown slug is 404, and unknown/derived keys map like the sheet preview.
func TestService_PreviewTemplate(t *testing.T) {
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"mod": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "floor((stats_dexterity - 10) / 2)",
				DependsOn:  [][]string{{"stats", "dexterity"}},
			}},
		}},
		"baseArmorClass": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		"derived": {Children: template.FieldGroup{
			"armorClass": {Schema: &template.FieldSchema{
				Type:       "number",
				IsEditable: false,
				Formula:    "baseArmorClass + mod_dexterityMod",
				DependsOn:  [][]string{{"baseArmorClass"}, {"mod", "dexterityMod"}},
			}},
		}},
	}

	dag, err := template.BuildDAG("test", fields, engine.NewFormulaEngine())
	require.NoError(t, err)

	dagCache := sheettest.DAGMap{"test": dag}
	tmpl := &template.Template{ID: "test", Name: "Test Template", Fields: fields}
	ctx := context.Background()

	t.Run("Happy path: valid delta returns the cascade over the seeded fields", func(t *testing.T) {
		mockTemplateSvc := templatemocks.NewMockService(t)
		mockTemplateSvc.EXPECT().
			FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: "test"}).
			Return(tmpl, nil).
			Once()
		mockRepo := mocks.NewMockRepository(t) // no expectations: the template preview never touches the repository

		svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

		cascades, err := svc.PreviewTemplate(ctx, sheet.PreviewTemplateCommand{
			TemplateID: "test",
			Data:       map[string]any{"stats": map[string]any{"dexterity": 18.0}},
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"mod":     map[string]any{"dexterityMod": 4.0},
			"derived": map[string]any{"armorClass": 14.0},
		}, cascades)
		mockRepo.AssertNotCalled(t, "FetchByID")
		mockRepo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Happy path: the seed equals the data Create builds for the template", func(t *testing.T) {
		mockTemplateSvc := templatemocks.NewMockService(t)
		mockTemplateSvc.EXPECT().
			FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: "test"}).
			Return(tmpl, nil).
			Twice()

		var createdData map[string]any
		mockRepo := mocks.NewMockRepository(t)
		mockRepo.EXPECT().
			FetchByOwnerID(ctx, ownerUserID).
			Return(nil, sheet.ErrSheetNotFound).
			Once()
		mockRepo.EXPECT().
			Create(ctx, mock.MatchedBy(func(s *sheet.Sheet) bool {
				createdData = s.Data
				return true
			})).
			Return(nil).
			Once()

		svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

		created, err := svc.Create(ctx, sheet.CreateSheetCommand{TemplateID: "test", SheetName: "Seed", OwnerID: ownerUserID})
		require.NoError(t, err)
		require.NotNil(t, createdData)

		delta := map[string]any{"stats": map[string]any{"dexterity": 18.0}}

		previewTemplateCascade, err := svc.PreviewTemplate(ctx, sheet.PreviewTemplateCommand{TemplateID: "test", Data: delta})
		require.NoError(t, err)

		mockRepo.EXPECT().
			FetchByID(ctx, created.ID).
			Return(&sheet.Sheet{ID: created.ID, TemplateID: "test", OwnerID: ownerUserID, Data: createdData}, nil).
			Once()
		previewCascade, err := svc.Preview(ctx, sheet.PartialUpdateSheetCommand{SheetID: created.ID, OwnerID: ownerUserID, Data: delta})
		require.NoError(t, err)

		assert.Equal(t, previewCascade, previewTemplateCascade)
	})

	t.Run("Error: template ID errors map to 400/404", func(t *testing.T) {
		tests := []struct {
			name       string
			id         string
			fetchErr   error
			wantCode   apperr.Code
			wantStatus int
			wantMsg    string
		}{
			{
				name:       "Edge: missing template ID",
				id:         "",
				fetchErr:   template.ErrTemplateIDNotProvided,
				wantCode:   apperr.CodeInvalidRequest,
				wantStatus: 400,
				wantMsg:    "template id not provided",
			},
			{
				name:       "Error: malformed template ID",
				id:         "Fantasy-V1",
				fetchErr:   template.ErrTemplateIDInvalid,
				wantCode:   apperr.CodeInvalidRequest,
				wantStatus: 400,
				wantMsg:    "template id is not a valid slug",
			},
			{
				name:       "Error: unknown template slug",
				id:         "unknown_template",
				fetchErr:   apperr.NewError(apperr.CodeNotFound, "template not found", nil),
				wantCode:   apperr.CodeNotFound,
				wantStatus: 404,
				wantMsg:    "template not found",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockTemplateSvc := templatemocks.NewMockService(t)
				mockTemplateSvc.EXPECT().
					FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: tc.id}).
					Return(nil, tc.fetchErr).
					Once()
				mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

				svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

				cascades, err := svc.PreviewTemplate(ctx, sheet.PreviewTemplateCommand{
					TemplateID: tc.id,
					Data:       map[string]any{"stats": map[string]any{"dexterity": 12.0}},
				})

				require.Error(t, err)
				assert.Nil(t, cascades)

				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
				assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
				assert.Equal(t, tc.wantMsg, appErr.Message)
			})
		}
	})

	t.Run("Error: DAG rejections map like the sheet preview", func(t *testing.T) {
		tests := []struct {
			name     string
			data     map[string]any
			wantCode apperr.Code
			wantMsg  string
		}{
			{
				name:     "Error: unknown field returns 400 UNKNOWN_FIELD",
				data:     map[string]any{"stats": map[string]any{"unknown": 1.0}},
				wantCode: apperr.CodeUnknownField,
				wantMsg:  "field does not exist: stats.unknown",
			},
			{
				name:     "Error: derived field returns 400 INVALID_REQUEST",
				data:     map[string]any{"mod": map[string]any{"dexterityMod": 5.0}},
				wantCode: apperr.CodeInvalidRequest,
				wantMsg:  "field is not editable: mod.dexterityMod",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockTemplateSvc := templatemocks.NewMockService(t)
				mockTemplateSvc.EXPECT().
					FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: "test"}).
					Return(tmpl, nil).
					Once()
				mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

				svc := sheet.NewService(mockRepo, mockTemplateSvc, dagCache)

				cascades, err := svc.PreviewTemplate(ctx, sheet.PreviewTemplateCommand{TemplateID: "test", Data: tc.data})

				require.Error(t, err)
				assert.Nil(t, cascades)

				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantCode, appErr.Code)
				assert.Equal(t, 400, appErr.HttpStatus())
				assert.Equal(t, tc.wantMsg, appErr.Message)
			})
		}
	})
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
		FetchByOwnerID(context.Background(), ownerUserID).
		Return(nil, sheet.ErrSheetNotFound).
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
			return s.TemplateID == "test" && s.Name == "New Character" && s.OwnerID == ownerUserID
		})).
		Return(nil).
		Once()

	cmd := sheet.CreateSheetCommand{
		TemplateID: "test",
		SheetName:  "New Character",
		OwnerID:    ownerUserID,
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
		FetchByOwnerID(context.Background(), ownerUserID).
		Return(nil, sheet.ErrSheetNotFound).
		Once()
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
			return s.OwnerID == ownerUserID
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(mockRepo, mockTemplateSvc, sheettest.DAGMap{"test": dag})

	s, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
		TemplateID: "test",
		SheetName:  "Seed Check",
		OwnerID:    ownerUserID,
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
				OwnerID:    ownerUserID,
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
		OwnerID:    ownerUserID,
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
		OwnerID: ownerUserID,
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

// TestService_FetchByID pins the ID and ownership checks to the service
// (LGN-66, LGN-86, OWN-10, OWN-11, OWN-24): controllers forward the path param
// and the authenticated owner, so an empty ID is 400, a non-UUID is 400 before
// the repository, a missing sheet is 404, and a sheet owned by another user or
// by nobody (legacy row) is the same 404.
func TestService_FetchByID(t *testing.T) {
	storedSheet := &sheet.Sheet{
		ID:         fetchSheetID,
		TemplateID: "fantasy_v1",
		OwnerID:    ownerUserID,
		Name:       "Fetched",
		Data:       map[string]any{"stats": map[string]any{"dexterity": 14.0}},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	tests := []struct {
		name       string
		id         string
		ownerID    string // empty means the default ownerUserID
		repoSheet  *sheet.Sheet
		repoErr    error
		callRepo   bool
		noOwner    bool
		wantCode   apperr.Code
		wantStatus int
		wantMsg    string
	}{
		{
			name:      "Happy path: returns the stored sheet to its owner",
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
		{
			name:       "Error: another user's sheet is the same 404 as a missing sheet",
			id:         fetchSheetID,
			repoSheet:  &sheet.Sheet{ID: fetchSheetID, TemplateID: "fantasy_v1", OwnerID: "01900000-0000-7000-8000-0000000000bb"},
			callRepo:   true,
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
			wantMsg:    "sheet not found",
		},
		{
			name:       "Error: a legacy sheet with no owner is the same 404",
			id:         fetchSheetID,
			repoSheet:  &sheet.Sheet{ID: fetchSheetID, TemplateID: "fantasy_v1", OwnerID: ""},
			callRepo:   true,
			wantCode:   apperr.CodeNotFound,
			wantStatus: 404,
			wantMsg:    "sheet not found",
		},
		{
			name:       "Error: missing owner is 401 before the repository",
			id:         fetchSheetID,
			callRepo:   false,
			noOwner:    true,
			wantCode:   apperr.CodeUnauthorized,
			wantStatus: 401,
			wantMsg:    sheet.MsgOwnerNotProvided,
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

			owner := tc.ownerID
			if !tc.noOwner && owner == "" {
				owner = ownerUserID
			}

			got, err := svc.FetchByID(context.Background(), tc.id, owner)

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

// TestService_FetchMySheet pins the owner-scoped read (OWN-07, OWN-08): the
// owner's sheet is returned, and an owner with no sheet is a 404 NOT_FOUND.
func TestService_FetchMySheet(t *testing.T) {
	mine := &sheet.Sheet{ID: fetchSheetID, TemplateID: "fantasy_v1", OwnerID: ownerUserID}

	t.Run("Happy path: returns the owner's sheet", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByOwnerID(context.Background(), ownerUserID).Return(mine, nil).Once()

		svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{})

		got, err := svc.FetchMySheet(context.Background(), ownerUserID)
		require.NoError(t, err)
		assert.Equal(t, mine, got)
	})

	t.Run("Error: an owner with no sheet is a 404 NOT_FOUND", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByOwnerID(context.Background(), ownerUserID).Return(nil, sheet.ErrSheetNotFound).Once()

		svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{})

		got, err := svc.FetchMySheet(context.Background(), ownerUserID)
		require.Error(t, err)
		assert.Nil(t, got)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		assert.Equal(t, 404, appErr.HttpStatus())
		assert.Equal(t, "sheet not found", appErr.Message)
	})

	t.Run("Error: a missing owner is 401 before the repository", func(t *testing.T) {
		repo := mocks.NewMockRepository(t) // no expectations: must never be called
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{})

		got, err := svc.FetchMySheet(context.Background(), "")
		require.Error(t, err)
		assert.Nil(t, got)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeUnauthorized, appErr.Code)
		assert.Equal(t, 401, appErr.HttpStatus())
	})
}

// TestService_PartialUpdate_MissingSheetID: PartialUpdate used to skip the ID
// check its FetchByID does, so an empty ID fell through to the repository and
// surfaced as 404 "sheet not found" instead of 400 (LGN-66).
func TestService_PartialUpdate_MissingSheetID(t *testing.T) {
	mockRepo := mocks.NewMockRepository(t) // no expectations: must never be called

	svc := sheet.NewService(mockRepo, templatemocks.NewMockService(t), sheettest.DAGMap{})

	cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
		SheetID: "",
		OwnerID: ownerUserID,
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
				OwnerID: ownerUserID,
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

// TestService_Create_Ownership pins the one-sheet-per-user rule at the service
// boundary (OWN-03, OWN-04, OWN-05, OWN-06, OWN-19): the owner is bound on the
// persisted sheet, a pre-existing owner is a 409 SHEET_LIMIT_REACHED, a
// duplicate-key race is the same 409 (never a 500), and a missing owner is a
// 401 with no repository access.
func TestService_Create_Ownership(t *testing.T) {
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
	}
	dag, err := template.BuildDAG("test", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	dagCache := sheettest.DAGMap{"test": dag}

	newTemplateSvc := func(t *testing.T) *templatemocks.MockService {
		t.Helper()
		svc := templatemocks.NewMockService(t)
		svc.EXPECT().
			FetchTemplate(mock.Anything, &template.FetchTemplateByIDCommand{ID: "test"}).
			Return(&template.Template{ID: "test", Name: "Test Template", Fields: fields}, nil).
			Once()
		return svc
	}

	t.Run("Happy path: the created sheet carries the authenticated owner", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByOwnerID(context.Background(), ownerUserID).Return(nil, sheet.ErrSheetNotFound).Once()
		repo.EXPECT().Create(context.Background(), mock.MatchedBy(func(s *sheet.Sheet) bool {
			return s.OwnerID == ownerUserID
		})).Return(nil).Once()

		svc := sheet.NewService(repo, newTemplateSvc(t), dagCache)

		created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
			TemplateID: "test", SheetName: "Mine", OwnerID: ownerUserID,
		})
		require.NoError(t, err)
		assert.Equal(t, ownerUserID, created.OwnerID)
	})

	t.Run("Error: an owner that already has a sheet is a 409 with no write", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByOwnerID(context.Background(), ownerUserID).
			Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID}, nil).Once()

		svc := sheet.NewService(repo, newTemplateSvc(t), dagCache)

		created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
			TemplateID: "test", SheetName: "Second", OwnerID: ownerUserID,
		})
		require.Error(t, err)
		assert.Nil(t, created)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeSheetLimitReached, appErr.Code)
		assert.Equal(t, 409, appErr.HttpStatus())
		assert.Equal(t, sheet.MsgSheetLimitReached, appErr.Message)
		repo.AssertNotCalled(t, "Create")
	})

	t.Run("Error: a duplicate-key race is a 409, never a 500", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByOwnerID(context.Background(), ownerUserID).Return(nil, sheet.ErrSheetNotFound).Once()
		repo.EXPECT().Create(context.Background(), mock.Anything).Return(sheet.ErrSheetOwnerExists).Once()

		svc := sheet.NewService(repo, newTemplateSvc(t), dagCache)

		created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
			TemplateID: "test", SheetName: "Race", OwnerID: ownerUserID,
		})
		require.Error(t, err)
		assert.Nil(t, created)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeSheetLimitReached, appErr.Code)
		assert.Equal(t, 409, appErr.HttpStatus())
	})

	t.Run("Error: a missing owner is a 401 before any repository access", func(t *testing.T) {
		repo := mocks.NewMockRepository(t) // no expectations: must never be called
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		created, err := svc.Create(context.Background(), sheet.CreateSheetCommand{
			TemplateID: "test", SheetName: "Ownerless",
		})
		require.Error(t, err)
		assert.Nil(t, created)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeUnauthorized, appErr.Code)
		assert.Equal(t, 401, appErr.HttpStatus())
		assert.Equal(t, sheet.MsgOwnerNotProvided, appErr.Message)
	})
}

// TestService_OwnershipGuardOnRecalculation pins the ownership check inside
// recalculate, which PATCH and both previews share (OWN-10, OWN-11, OWN-12,
// OWN-14, OWN-21, OWN-24): a foreign or legacy (no owner) sheet is the same 404
// as a missing sheet and triggers no write or recomputation, a missing owner is
// a 401 before the repository, and the owner's own delta still recalculates.
func TestService_OwnershipGuardOnRecalculation(t *testing.T) {
	fields := template.FieldGroup{
		"stats": {Children: template.FieldGroup{
			"dexterity": {Schema: &template.FieldSchema{Type: "number", IsEditable: true, Default: 10}},
		}},
		"modifiers": {Children: template.FieldGroup{
			"dexterityMod": {Schema: &template.FieldSchema{
				Type: "number", IsEditable: false,
				Formula:   "floor((stats_dexterity - 10) / 2)",
				DependsOn: [][]string{{"stats", "dexterity"}},
			}},
		}},
	}
	dag, err := template.BuildDAG("fantasy_v1", fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	dagCache := sheettest.DAGMap{"fantasy_v1": dag}

	data := map[string]any{"stats": map[string]any{"dexterity": 14.0}}
	foreign := &sheet.Sheet{ID: updateSheetID, TemplateID: "fantasy_v1", OwnerID: "01900000-0000-7000-8000-0000000000bb", Data: data}
	legacy := &sheet.Sheet{ID: updateSheetID, TemplateID: "fantasy_v1", Data: data}
	delta := map[string]any{"stats": map[string]any{"dexterity": 16.0}}

	assertNotFound := func(t *testing.T, err error) {
		t.Helper()
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		assert.Equal(t, 404, appErr.HttpStatus())
	}

	t.Run("Error: PATCH on a foreign sheet is 404 and writes nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(context.Background(), updateSheetID).Return(foreign, nil).Once()
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta,
		})
		require.Error(t, err)
		assert.Nil(t, cascades)
		assertNotFound(t, err)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: preview on a foreign sheet is 404 and writes nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(context.Background(), updateSheetID).Return(foreign, nil).Once()
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.Preview(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta,
		})
		require.Error(t, err)
		assert.Nil(t, cascades)
		assertNotFound(t, err)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: a legacy sheet with no owner is 404 for any reader", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(context.Background(), updateSheetID).Return(legacy, nil).Once()
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta,
		})
		require.Error(t, err)
		assert.Nil(t, cascades)
		assertNotFound(t, err)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: a missing owner is 401 before the repository on PATCH and preview", func(t *testing.T) {
		repo := mocks.NewMockRepository(t) // no expectations: must never be called
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		for _, err := range []error{
			func() error {
				_, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{SheetID: updateSheetID, Data: delta})
				return err
			}(),
			func() error {
				_, err := svc.Preview(context.Background(), sheet.PartialUpdateSheetCommand{SheetID: updateSheetID, Data: delta})
				return err
			}(),
		} {
			require.Error(t, err)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeUnauthorized, appErr.Code)
			assert.Equal(t, 401, appErr.HttpStatus())
		}
	})

	t.Run("Happy path: the owner's own PATCH still recalculates and persists", func(t *testing.T) {
		owned := &sheet.Sheet{
			ID:         updateSheetID,
			TemplateID: "fantasy_v1",
			OwnerID:    ownerUserID,
			Data: map[string]any{
				"stats":     map[string]any{"dexterity": 14.0},
				"modifiers": map[string]any{"dexterityMod": 2.0},
			},
		}

		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(context.Background(), updateSheetID).Return(owned, nil).Once()
		repo.EXPECT().PartialUpdate(context.Background(), updateSheetID, mock.Anything).Return(nil).Once()
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), dagCache)

		cascades, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: updateSheetID, OwnerID: ownerUserID, Data: delta,
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"modifiers": map[string]any{"dexterityMod": 3.0}}, cascades)
	})
}
