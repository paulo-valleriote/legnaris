package sheet_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

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

func TestService_SetDice(t *testing.T) {
	tmpl := layoutTemplate()
	full := &sheet.Dice{
		Section: map[string]*string{"stats": text("1d20 + @"), "config": nil},
		Row:     map[string]*string{"skills.acrobatics": text("1d20 + @.total"), "stats.strength": nil},
	}
	emptyBlock := sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}}

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
		want         sheet.Dice
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
			name:       "Edge: nil block clears without consulting the template",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			dice:       nil,
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
			want:       emptyBlock,
		},
		{
			name:       "Edge: empty block object clears",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			dice:       &sheet.Dice{},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
			want:       emptyBlock,
		},
		{
			name:       "Edge: block of empty maps clears",
			ownerID:    ownerUserID,
			sheetID:    fetchSheetID,
			dice:       &sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{}},
			stored:     layoutStoredSheet(ownerUserID),
			wantFetch:  true,
			wantUpdate: true,
			want:       emptyBlock,
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
			want:         sheet.Dice{Section: map[string]*string{"stats": nil}, Row: map[string]*string{}},
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
			want:         sheet.Dice{Section: map[string]*string{}, Row: map[string]*string{"baseArmorClass": text("1d20 + @")}},
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
			want:         *full,
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
				repo.EXPECT().UpdateDice(mock.Anything, tc.sheetID, tc.wantStored).Return(tc.updateErr).Once()
			}

			got, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{
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
				assert.Equal(t, sheet.Dice{}, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestService_SetDice_RejectsMoreThan64EntriesAfterOwnership(t *testing.T) {
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

		_, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: tooMany})

		code, message := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeInvalidRequest, code)
		assert.Equal(t, "invalid dice: 65 dice overrides exceed the maximum of 64", message)
	})

	t.Run("Error: a foreign sheet stays 404 even with 65 entries", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		svc := sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog)
		repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet("someone-else"), nil).Once()

		_, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: tooMany})

		code, _ := layoutErrCode(t, err)
		assert.Equal(t, apperr.CodeNotFound, code)
	})
}

func TestService_SetDice_ReturnedBlockDoesNotAliasTheCommand(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Once()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
	repo.EXPECT().UpdateDice(mock.Anything, fetchSheetID, mock.Anything).Return(nil).Once()

	cmdDice := &sheet.Dice{Row: map[string]*string{"stats.strength": text("1d20")}}
	got, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: cmdDice})
	require.NoError(t, err)

	*cmdDice.Row["stats.strength"] = "mutated"
	cmdDice.Row["stats.dexterity"] = nil
	assert.Equal(t, map[string]*string{"stats.strength": text("1d20")}, got.Row)
}

func TestService_SetDice_IsIdempotent(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)

	block := &sheet.Dice{Section: map[string]*string{"stats": nil}, Row: map[string]*string{"stats.strength": text("2d6")}}
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Twice()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Twice()
	repo.EXPECT().UpdateDice(mock.Anything, fetchSheetID, block).Return(nil).Twice()

	cmd := sheet.SetDiceCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: block}
	first, err := svc.SetDice(context.Background(), cmd)
	require.NoError(t, err)
	second, err := svc.SetDice(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestService_SetDice_TouchesNeitherDataNorLayout(t *testing.T) {
	tmpl := layoutTemplate()
	stored := layoutStoredSheet(ownerUserID)
	stored.Layout = &sheet.Layout{Pinned: []string{"stats.strength"}}
	dataBefore := fmt.Sprint(stored.Data)

	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(stored, nil).Once()
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Once()
	repo.EXPECT().UpdateDice(mock.Anything, fetchSheetID, mock.Anything).Return(nil).Once()

	_, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{
		SheetID: fetchSheetID,
		OwnerID: ownerUserID,
		Dice:    &sheet.Dice{Row: map[string]*string{"stats.strength": text("1d20")}},
	})

	require.NoError(t, err)
	assert.Equal(t, dataBefore, fmt.Sprint(stored.Data))
	assert.Equal(t, &sheet.Layout{Pinned: []string{"stats.strength"}}, stored.Layout)
}

func TestService_SetDice_ConcurrentCallsEachReturnTheirOwnValidBlock(t *testing.T) {
	tmpl := layoutTemplate()
	repo := mocks.NewMockRepository(t)
	templateSvc := templatemocks.NewMockService(t)
	svc := sheet.NewService(repo, templateSvc, sheettest.DAGMap{}, testLog)
	const callers = 16
	repo.EXPECT().FetchByID(mock.Anything, fetchSheetID).Return(layoutStoredSheet(ownerUserID), nil).Times(callers)
	templateSvc.EXPECT().FetchTemplate(mock.Anything, mock.Anything).Return(&tmpl, nil).Times(callers)
	repo.EXPECT().UpdateDice(mock.Anything, fetchSheetID, mock.Anything).Return(nil).Times(callers)

	shared := &sheet.Dice{Section: map[string]*string{"stats": text("1d20 + @")}, Row: map[string]*string{"stats.strength": nil}}
	var wg sync.WaitGroup
	results := make(chan sheet.Dice, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := svc.SetDice(context.Background(), sheet.SetDiceCommand{SheetID: fetchSheetID, OwnerID: ownerUserID, Dice: shared})
			assert.NoError(t, err)
			results <- got
		}()
	}
	wg.Wait()
	close(results)

	for got := range results {
		assert.Equal(t, *shared, got)
	}
}
