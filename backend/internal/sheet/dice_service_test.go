package sheet_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"legnaris/backend/internal/engine"
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

// diceOnlyUpdate matches the repository write of a save that carried only a
// Dice block: the block as want, an updated_at, and no data key.
func diceOnlyUpdate(want *sheet.Dice) any {
	return mock.MatchedBy(func(updates map[string]any) bool {
		stored, ok := updates["dice"].(*sheet.Dice)
		_, hasData := updates["data"]
		_, hasUpdatedAt := updates["updated_at"]
		return ok && !hasData && hasUpdatedAt && len(updates) == 2 && assert.ObjectsAreEqual(want, stored)
	})
}

func diceSave(dice *sheet.Dice) sheet.PartialUpdateSheetCommand {
	return sheet.PartialUpdateSheetCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: dice}
}

func TestService_PartialUpdate_Dice(t *testing.T) {
	tmpl := layoutTemplate()
	full := &sheet.Dice{
		Section: map[string]*string{"stats": text("1d20 + @"), "config": nil},
		Row:     map[string]*string{"skills.acrobatics": text("1d20 + @.total"), "stats.strength": nil},
	}

	tests := []struct {
		name         string
		ownerID      string
		sheetID      string
		dice         *sheet.Dice
		stored       *sheet.Sheet
		fetchErr     error
		templateErr  error
		updateErr    error
		wantFetch    bool
		wantTemplate bool
		wantUpdate   bool
		wantStored   *sheet.Dice
		wantCode     apperr.Code
		wantMessage  string
		wantStatus   int
	}{
		{
			name:        "Error: missing owner is unauthorized and touches nothing",
			sheetID:     fetchSheetID,
			dice:        full,
			wantCode:    apperr.CodeUnauthorized,
			wantMessage: sheet.MsgOwnerNotProvided,
			wantStatus:  401,
		},
		{
			name:        "Error: missing sheet id",
			ownerID:     ownerUserID,
			dice:        full,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgSheetIDNotProvided,
			wantStatus:  400,
		},
		{
			name:        "Error: malformed sheet id",
			ownerID:     ownerUserID,
			sheetID:     "not-a-uuid",
			dice:        full,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgSheetIDInvalid,
			wantStatus:  400,
		},
		{
			name:        "Error: sheet does not exist",
			ownerID:     ownerUserID,
			sheetID:     missingSheetID,
			dice:        full,
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
			dice:        full,
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
			dice:        full,
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
			dice:        full,
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
			dice:         full,
			stored:       layoutStoredSheet(ownerUserID),
			templateErr:  apperr.NewError(apperr.CodeNotFound, "template not found", nil),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeNotFound,
			wantMessage:  "template not found",
			wantStatus:   404,
		},
		{
			name:         "Error: unknown section",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Section: map[string]*string{"ghost": text("1d20")}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid dice: dice section 'ghost' is not a section of the template",
			wantStatus:   400,
		},
		{
			name:         "Error: unknown row, even when the value is null",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Row: map[string]*string{"stats.ghost": nil}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid dice: dice row 'stats.ghost' is not a row of any section",
			wantStatus:   400,
		},
		{
			name:         "Error: invalid expression",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Row: map[string]*string{"stats.strength": text("1d20 +")}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantMessage:  "invalid dice: invalid dice expression on row 'stats.strength': a term is missing next to a + or -",
			wantStatus:   400,
		},
		{
			name:         "Error: row override that does not fit its row",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Row: map[string]*string{"skills.acrobatics": text("1d20 + @")}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantStatus:   400,
			wantMessage:  "invalid dice: invalid dice expression on row 'skills.acrobatics': \"@\" points at \"skills.acrobatics\", which does not exist",
		},
		{
			name:         "Error: section override that fits no row",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Section: map[string]*string{"stats": text("1d20 + @.total")}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantCode:     apperr.CodeInvalidRequest,
			wantStatus:   400,
			wantMessage:  "invalid dice: invalid dice expression on section 'stats': it fits none of the Rows in the Section: \"@.total\" points at \"stats.dexterity.total\", which does not exist",
		},
		{
			name:         "Error: sheet deleted between fetch and write is not found",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         full,
			stored:       layoutStoredSheet(ownerUserID),
			updateErr:    sheet.ErrSheetNotFound,
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   full,
			wantCode:     apperr.CodeNotFound,
			wantMessage:  "sheet not found",
			wantStatus:   404,
		},
		{
			name:         "Error: a lost compare-and-swap is 409 and the block is not retried",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         full,
			stored:       layoutStoredSheet(ownerUserID),
			updateErr:    sheet.ErrVersionConflict,
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   full,
			wantCode:     apperr.CodeConflict,
			wantMessage:  sheet.MsgSheetConflict,
			wantStatus:   409,
		},
		{
			name:         "Error: repository failure on write is internal",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         full,
			stored:       layoutStoredSheet(ownerUserID),
			updateErr:    errors.New("boom"),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   full,
			wantCode:     apperr.CodeInternal,
			wantMessage:  "failed to update sheet",
			wantStatus:   500,
		},
		{
			name:        "Error: neither data nor dice is 400 before the repository",
			ownerID:     ownerUserID,
			sheetID:     fetchSheetID,
			dice:        nil,
			wantCode:    apperr.CodeInvalidRequest,
			wantMessage: sheet.MsgNothingToUpdate,
			wantStatus:  400,
		},
		{
			name:       "Edge: empty block object clears without consulting the template",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			dice:       &sheet.Dice{},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
		},
		{
			name:       "Edge: block of empty maps clears",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			dice:       &sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
		},
		{
			name:         "Edge: only null entries is a real block, stored as given",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Section: map[string]*string{"stats": nil}},
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   &sheet.Dice{Section: map[string]*string{"stats": nil}},
		},
		{
			name:         "Edge: a sheet with an existing block is fully replaced",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         &sheet.Dice{Row: map[string]*string{"baseArmorClass": text("1d20 + @")}},
			stored:       func() *sheet.Sheet { s := layoutStoredSheet(ownerUserID); s.Dice = full.Clone(); return s }(),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   &sheet.Dice{Row: map[string]*string{"baseArmorClass": text("1d20 + @")}},
		},
		{
			name:         "Happy path: full block is validated and stored",
			ownerID:      ownerUserID,
			sheetID:      fetchSheetID,
			dice:         full,
			stored:       layoutStoredSheet(ownerUserID),
			wantFetch:    true,
			wantTemplate: true,
			wantUpdate:   true,
			wantStored:   full,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			templateSvc := templatemocks.NewMockService(t)
			svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)

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
				repo.EXPECT().PartialUpdate(mock.Anything, tc.sheetID, int64(0), diceOnlyUpdate(tc.wantStored)).
					Return(int64(1), tc.updateErr).Once()
			}

			got, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
				SheetID: tc.sheetID,
				OwnerID: tc.ownerID,
				Dice:    tc.dice,
			})

			if tc.wantCode != "" {
				require.Error(t, err)
				code, message := layoutErrCode(t, err)
				assert.Equal(t, tc.wantCode, code)
				if tc.wantMessage != "" {
					assert.Equal(t, tc.wantMessage, message)
				}
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, map[string]any{}, got, "a dice-only save recalculates nothing")
		})
	}
}

// layoutDAGService builds a service whose DAG matches layoutTemplate, so a save
// can carry data and dice together.
func layoutDAGService(t *testing.T, repo sheet.Repository, templateSvc template.Service) sheet.Service {
	t.Helper()
	tmpl := layoutTemplate()
	dag, err := template.BuildDAG(layoutTemplateID, tmpl.Fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return sheet.NewService(repo, templateSvc, sheettest.DAGMap{layoutTemplateID: dag}, testLog)
}

func TestService_PartialUpdate_DataAndDiceTogether(t *testing.T) {
	tmpl := layoutTemplate()
	dexterity := map[string]any{"stats": map[string]any{"dexterity": 16.0}}
	block := &sheet.Dice{Row: map[string]*string{"stats.strength": text("1d20 + @")}}

	t.Run("Error: an invalid block refuses the whole save and writes no data", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		templateSvc := templatemocks.NewMockService(t)
		svc := layoutDAGService(t, repo, templateSvc)
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
		templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()

		got, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: fetchSheetID,
			OwnerID: ownerUserID,
			Data:    dexterity,
			Dice:    &sheet.Dice{Row: map[string]*string{"stats.ghost": nil}},
		})

		code, _ := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeInvalidRequest, code)
		assert.Nil(t, got)
	})

	t.Run("Error: an invalid data delta refuses the whole save before the template is read", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		svc := layoutDAGService(t, repo, templatemocks.NewMockService(t))
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()

		_, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: fetchSheetID,
			OwnerID: ownerUserID,
			Data:    map[string]any{"stats": map[string]any{"ghost": 1.0}},
			Dice:    block,
		})

		code, _ := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeUnknownField, code)
	})

	t.Run("Edge: a data-only save leaves the stored block untouched", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		svc := layoutDAGService(t, repo, templatemocks.NewMockService(t))
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
		repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, int64(0), mock.MatchedBy(func(updates map[string]any) bool {
			_, hasDice := updates["dice"]
			_, hasData := updates["data"]
			return !hasDice && hasData
		})).Return(int64(1), nil).Once()

		_, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: fetchSheetID, OwnerID: ownerUserID, Data: dexterity,
		})

		require.NoError(t, err)
	})

	t.Run("Happy path: data and dice land in one write and the cascade comes back", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		templateSvc := templatemocks.NewMockService(t)
		svc := layoutDAGService(t, repo, templateSvc)
		stored := layoutStoredSheet(ownerUserID)
		stored.Version = 7
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(stored, nil).Once()
		templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
		repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, int64(7), mock.MatchedBy(func(updates map[string]any) bool {
			data, _ := updates["data"].(map[string]any)
			stats, _ := data["stats"].(map[string]any)
			dice, _ := updates["dice"].(*sheet.Dice)
			return stats["dexterity"] == 16.0 && assert.ObjectsAreEqual(block, dice)
		})).Return(int64(8), nil).Once()

		got, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
			SheetID: fetchSheetID, OwnerID: ownerUserID, Data: dexterity, Dice: block,
		})

		require.NoError(t, err)
		assert.Equal(t, map[string]any{"modifiers": map[string]any{"dexterityMod": 3.0}}, got)
	})
}

func TestService_PartialUpdate_DiceRejectsMoreThan64EntriesAfterOwnership(t *testing.T) {
	tooMany := &sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}
	for i := range 33 {
		tooMany.Section[fmt.Sprintf("s%d", i)] = nil
	}
	for i := range 32 {
		tooMany.Row[fmt.Sprintf("r%d", i)] = nil
	}
	tmpl := layoutTemplate()

	t.Run("Error: 65 entries is a 400 and nothing is written", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		templateSvc := templatemocks.NewMockService(t)
		svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
		templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()

		_, err := svc.PartialUpdate(context.Background(), diceSave(tooMany))

		code, message := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeInvalidRequest, code)
		assert.Equal(t, "invalid dice: 65 dice overrides exceed the maximum of 64", message)
	})

	t.Run("Error: a foreign sheet stays 404 even with 65 entries", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog)
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet("someone-else"), nil).Once()

		_, err := svc.PartialUpdate(context.Background(), diceSave(tooMany))

		code, _ := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeNotFound, code)
	})
}

func TestService_PartialUpdate_StoredDiceDoesNotAliasTheCommand(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
	var stored *sheet.Dice
	repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, int64(0), mock.Anything).
		Run(func(_ context.Context, _ string, _ int64, updates map[string]any) {
			stored = updates["dice"].(*sheet.Dice)
		}).
		Return(int64(1), nil).Once()

	cmdDice := &sheet.Dice{Row: map[string]*string{"stats.strength": text("1d20")}}
	_, err := svc.PartialUpdate(context.Background(), diceSave(cmdDice))
	require.NoError(t, err)

	*cmdDice.Row["stats.strength"] = "mutated"
	cmdDice.Row["stats.dexterity"] = nil
	assert.Equal(t, map[string]*string{"stats.strength": text("1d20")}, stored.Row)
}

func TestService_PartialUpdate_ConcurrentDiceSavesEachWriteTheirOwnValidBlock(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
	const callers = 16
	shared := &sheet.Dice{Section: map[string]*string{"stats": text("1d20 + @")}, Row: map[string]*string{"stats.strength": nil}}
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Times(callers)
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Times(callers)
	repo.EXPECT().PartialUpdate(mock.Anything, fetchSheetID, int64(0), diceOnlyUpdate(shared)).Return(int64(1), nil).Times(callers)

	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.PartialUpdate(context.Background(), diceSave(shared))
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}
