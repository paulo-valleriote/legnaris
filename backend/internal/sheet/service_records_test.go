package sheet_test

import (
	"context"
	"strings"
	"testing"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func listSheet() *sheet.Sheet {
	return &sheet.Sheet{
		ID:         patchSheetID,
		TemplateID: sheettest.ListTemplateID,
		OwnerID:    ownerUserID,
		Data:       sheettest.ListSeedData(),
	}
}

func listSheetWithRecord(id string, position float64) *sheet.Sheet {
	data := sheettest.ListSeedData()
	data["inventory"].(map[string]any)["items"] = map[string]any{
		id: map[string]any{
			"name":         "Rope",
			"quantity":     1.0,
			"active":       0.0,
			"derivedTotal": 0.0,
			"position":     position,
		},
	}
	return &sheet.Sheet{
		ID:         patchSheetID,
		TemplateID: sheettest.ListTemplateID,
		OwnerID:    ownerUserID,
		Data:       data,
	}
}

func recordsPatch(data map[string]any) sheet.PartialUpdateSheetCommand {
	return sheet.PartialUpdateSheetCommand{
		SheetID:   patchSheetID,
		OwnerID:   ownerUserID,
		RequestID: patchRequestID,
		Data:      data,
	}
}

func newRecordsService(t *testing.T, repo sheet.Repository) sheet.Service {
	t.Helper()
	return sheet.NewService(
		repo,
		templatemocks.NewMockService(t),
		sheettest.DAGMap{sheettest.ListTemplateID: sheettest.ListDAG(t)},
		testLog,
		events.NoOp{},
	)
}

func itemsOf(data map[string]any) map[string]any {
	return data["inventory"].(map[string]any)["items"].(map[string]any)
}

func TestService_PartialUpdate_Records(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a new record is seeded from the shape and appended", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				if !ok {
					return false
				}
				record, ok := itemsOf(data)[sheettest.ListRecordID].(map[string]any)
				if !ok {
					return false
				}
				return record["name"] == "Rope" &&
					record["quantity"] == 1.0 &&
					record["active"] == 0.0 &&
					record["derivedTotal"] == 0.0 &&
					record["position"] == 0.0
			})).
			Return(int64(1), nil).
			Once()

		cascade, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID: map[string]any{"name": "Rope"},
				},
			},
		}))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, cascade)
	})

	t.Run("Happy path: removing a record leaves an empty list", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheetWithRecord(sheettest.ListRecordID, 0), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				return ok && len(itemsOf(data)) == 0
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{sheettest.ListRecordID: nil},
			},
		}))
		require.NoError(t, err)
	})

	t.Run("Edge: adding and removing in one patch is decided on the merged list", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().
			FetchByID(ctx, patchSheetID).
			Return(listSheetWithRecord(sheettest.ListRecordID, 0), nil).
			Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				if !ok {
					return false
				}
				items := itemsOf(data)
				_, removedStillThere := items[sheettest.ListRecordID]
				_, added := items[sheettest.ListRecordID2]
				return !removedStillThere && added && len(items) == 1
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID:  nil,
					sheettest.ListRecordID2: map[string]any{"name": "Torch"},
				},
			},
		}))
		require.NoError(t, err)
	})

	t.Run("Edge: writing to a record removed in the same patch leaves it removed", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				if !ok {
					return false
				}
				_, exists := itemsOf(data)[sheettest.ListRecordID]
				return !exists
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory.items." + sheettest.ListRecordID:           nil,
			"inventory.items." + sheettest.ListRecordID + ".name": "Rope",
		}))
		require.NoError(t, err)
	})

	t.Run("Edge: removing an absent record twice is a no-op", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Twice()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.Anything).
			Return(int64(1), nil).
			Twice()

		svc := newRecordsService(t, repo)
		remove := recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{sheettest.ListRecordID: nil},
			},
		})
		_, err := svc.PartialUpdate(ctx, remove)
		require.NoError(t, err)
		_, err = svc.PartialUpdate(ctx, remove)
		require.NoError(t, err)
	})

	t.Run("Happy path: a list cleared to empty accepts a new record", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				return ok && len(itemsOf(data)) == 1
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID2: map[string]any{"name": "Torch"},
				},
			},
		}))
		require.NoError(t, err)
	})

	t.Run("Happy path: reordering writes the new position", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheetWithRecord(sheettest.ListRecordID, 0), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				if !ok {
					return false
				}
				record := itemsOf(data)[sheettest.ListRecordID].(map[string]any)
				return record["position"] == 9.0
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID: map[string]any{"position": 9.0},
				},
			},
		}))
		require.NoError(t, err)
	})

	t.Run("Happy path: a max_length text and a flag inside a record save", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				if !ok {
					return false
				}
				record := itemsOf(data)[sheettest.ListRecordID].(map[string]any)
				return record["name"] == strings.Repeat("a", 20) && record["active"] == 1.0
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID: map[string]any{
						"name":   strings.Repeat("a", 20),
						"active": 1.0,
					},
				},
			},
		}))
		require.NoError(t, err)
	})
}

func TestService_PartialUpdate_RecordErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		data       map[string]any
		wantCode   apperr.Code
		wantStatus int
	}{
		{
			name:       "Error: a malformed record id",
			data:       map[string]any{"inventory": map[string]any{"items": map[string]any{"rNOTHEX": map[string]any{"name": "Rope"}}}},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "Error: an unknown record field",
			data:       map[string]any{"inventory": map[string]any{"items": map[string]any{sheettest.ListRecordID: map[string]any{"weight": 2.0}}}},
			wantCode:   apperr.CodeUnknownField,
			wantStatus: 400,
		},
		{
			name:       "Error: a derived record field",
			data:       map[string]any{"inventory": map[string]any{"items": map[string]any{sheettest.ListRecordID: map[string]any{"derivedTotal": 2.0}}}},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "Error: text longer than max_length",
			data:       map[string]any{"inventory": map[string]any{"items": map[string]any{sheettest.ListRecordID: map[string]any{"name": strings.Repeat("a", 21)}}}},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
		{
			name:       "Error: a flag value other than zero or one",
			data:       map[string]any{"inventory": map[string]any{"items": map[string]any{sheettest.ListRecordID: map[string]any{"active": 2.0}}}},
			wantCode:   apperr.CodeInvalidRequest,
			wantStatus: 400,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()

			_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(tc.data))
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
			repo.AssertNotCalled(t, "PartialUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestService_PartialUpdate_RecordCapacity(t *testing.T) {
	ctx := context.Background()
	newRecord := func(id string) map[string]any {
		return map[string]any{"name": "Item"}
	}

	t.Run("Happy path: at capacity is allowed", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()
		repo.EXPECT().
			PartialUpdate(ctx, patchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
				data, ok := updates["data"].(map[string]any)
				return ok && len(itemsOf(data)) == 2
			})).
			Return(int64(1), nil).
			Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID:  newRecord(sheettest.ListRecordID),
					sheettest.ListRecordID2: newRecord(sheettest.ListRecordID2),
				},
			},
		}))
		require.NoError(t, err)
	})

	t.Run("Error: over capacity is refused before the write", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()

		_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID:  newRecord(sheettest.ListRecordID),
					sheettest.ListRecordID2: newRecord(sheettest.ListRecordID2),
					sheettest.ListRecordID3: newRecord(sheettest.ListRecordID3),
				},
			},
		}))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		repo.AssertNotCalled(t, "PartialUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestService_PartialUpdate_RecordsVersionConflict(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheetWithRecord(sheettest.ListRecordID, 0), nil).Once()
	repo.EXPECT().
		PartialUpdate(ctx, patchSheetID, int64(0), mock.Anything).
		Return(int64(0), sheeterr.ErrVersionConflict).
		Once()

	_, err := newRecordsService(t, repo).PartialUpdate(ctx, recordsPatch(map[string]any{
		"inventory": map[string]any{
			"items": map[string]any{
				sheettest.ListRecordID: map[string]any{"position": 3.0},
			},
		},
	}))
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeConflict, appErr.Code)
	assert.Equal(t, 409, appErr.HttpStatus())
}

func TestService_Preview_AcceptsRecordsWithoutWriting(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(ctx, patchSheetID).Return(listSheet(), nil).Once()

	cascade, err := newRecordsService(t, repo).Preview(ctx, recordsPatch(map[string]any{
		"inventory": map[string]any{
			"items": map[string]any{
				sheettest.ListRecordID: map[string]any{"name": "Rope"},
			},
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, cascade)
	repo.AssertNotCalled(t, "PartialUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestService_Create_SeedsListsEmpty(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	tmpl := templatemocks.NewMockService(t)
	tmpl.EXPECT().
		FetchTemplate(ctx, &template.FetchTemplateByIDCommand{ID: sheettest.ListTemplateID}).
		Return(&template.Template{ID: sheettest.ListTemplateID, Name: "Records", Fields: sheettest.ListFields, Kinds: []plan.SheetKind{plan.KindCharacter}}, nil).
		Once()
	repo.EXPECT().ReserveSlot(ctx, ownerUserID, plan.KindCharacter, mock.Anything).Return(nil).Once()
	repo.EXPECT().
		Create(ctx, mock.MatchedBy(func(stored *sheet.Sheet) bool {
			items, ok := stored.Data["inventory"].(map[string]any)["items"].(map[string]any)
			return ok && len(items) == 0
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(repo, tmpl, sheettest.DAGMap{sheettest.ListTemplateID: sheettest.ListDAG(t)}, testLog, events.NoOp{}, limitOpts(t))
	_, err := svc.Create(ctx, sheet.CreateSheetCommand{
		TemplateID: sheettest.ListTemplateID,
		Kind:       string(plan.KindCharacter),
		SheetName:  "Records",
		OwnerID:    ownerUserID,
	})
	require.NoError(t, err)
}

func TestService_Duplicate_DeepCopiesRecords(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	stored := listSheetWithRecord(sheettest.ListRecordID, 0)
	stored.Kind = plan.KindNPC
	var created *sheet.Sheet
	repo.EXPECT().FetchByID(ctx, patchSheetID).Return(stored, nil).Once()
	repo.EXPECT().ReserveSlot(ctx, ownerUserID, plan.KindNPC, mock.Anything).Return(nil).Once()
	repo.EXPECT().
		Create(ctx, mock.MatchedBy(func(s *sheet.Sheet) bool {
			created = s
			return true
		})).
		Return(nil).
		Once()

	svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, events.NoOp{}, limitOpts(t))
	copied, err := svc.Duplicate(ctx, sheet.DuplicateSheetCommand{SheetID: patchSheetID, OwnerID: ownerUserID})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, stored.Data, created.Data)

	itemsOf(copied.Data)[sheettest.ListRecordID].(map[string]any)["name"] = "Tampered"
	assert.Equal(t, "Rope", itemsOf(stored.Data)[sheettest.ListRecordID].(map[string]any)["name"])
}

func TestService_PartialUpdate_RecordPathOnTemplateWithoutListIsUnknown(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(0, "Aria", 14), nil).Once()

	_, err := newPatchService(t, repo, testLog, patchDAG(t)).PartialUpdate(ctx, recordsPatch(map[string]any{
		"items": map[string]any{
			sheettest.ListRecordID: map[string]any{"name": "Rope"},
		},
	}))
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnknownField, appErr.Code)
}

func TestService_Records_MongoRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepositoryTestWithDB(t)

	const sheetID = "01900000-0000-7000-8000-0000000000f1"
	created := &sheet.Sheet{
		ID:         sheetID,
		TemplateID: sheettest.ListTemplateID,
		OwnerID:    ownerUserID,
		Kind:       plan.KindCharacter,
		Name:       "Inventory",
		Data:       sheettest.ListSeedData(),
	}
	require.NoError(t, repo.Create(ctx, created))

	svc := sheet.NewService(
		repo,
		templatemocks.NewMockService(t),
		sheettest.DAGMap{sheettest.ListTemplateID: sheettest.ListDAG(t)},
		testLog,
		events.NoOp{},
	)

	t.Run("Happy path: create, reorder, remove and read back", func(t *testing.T) {
		_, err := svc.PartialUpdate(ctx, recordsPatchAt(sheetID, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID:  map[string]any{"name": "Rope", "position": 1.0},
					sheettest.ListRecordID2: map[string]any{"name": "Torch", "position": 2.0},
				},
			},
		}))
		require.NoError(t, err)

		_, err = svc.PartialUpdate(ctx, recordsPatchAt(sheetID, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID: map[string]any{"position": 3.0},
				},
			},
		}))
		require.NoError(t, err)

		_, err = svc.PartialUpdate(ctx, recordsPatchAt(sheetID, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{sheettest.ListRecordID2: nil},
			},
		}))
		require.NoError(t, err)

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		items := itemsOf(stored.Data)
		require.Len(t, items, 1)
		record, ok := items[sheettest.ListRecordID].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Rope", record["name"])
		assert.InDelta(t, 1.0, record["quantity"], 0)
		assert.InDelta(t, 3.0, record["position"], 0)
	})

	t.Run("Happy path: the last removal leaves an empty list that takes a new record", func(t *testing.T) {
		_, err := svc.PartialUpdate(ctx, recordsPatchAt(sheetID, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{sheettest.ListRecordID: nil},
			},
		}))
		require.NoError(t, err)

		stored, err := repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		assert.Empty(t, itemsOf(stored.Data))

		_, err = svc.PartialUpdate(ctx, recordsPatchAt(sheetID, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID3: map[string]any{"name": "Rations"},
				},
			},
		}))
		require.NoError(t, err)

		stored, err = repo.FetchByID(ctx, sheetID)
		require.NoError(t, err)
		require.Len(t, itemsOf(stored.Data), 1)
	})
}

func recordsPatchAt(sheetID string, data map[string]any) sheet.PartialUpdateSheetCommand {
	return sheet.PartialUpdateSheetCommand{
		SheetID:   sheetID,
		OwnerID:   ownerUserID,
		RequestID: patchRequestID,
		Data:      data,
	}
}
