package sheet_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"
	logmocks "legnaris/backend/shared/logger/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	patchSheetID    = "01900000-0000-7000-8000-0000000000d1"
	patchTemplateID = "patch_test_v1"
	patchRequestID  = "req-patch-1"
)

// patchDAG is a small template with one text leaf, one editable number and one
// derived number, so a PATCH exercises a cascade and a string the log must hide.
func patchDAG(t *testing.T) *template.DAG {
	t.Helper()
	fields := template.FieldGroup{
		"identity": {Children: template.FieldGroup{
			"characterName": {Schema: &template.FieldSchema{Type: "text", IsEditable: true}},
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
	dag, err := template.BuildDAG(patchTemplateID, fields, engine.NewFormulaEngine())
	require.NoError(t, err)
	return dag
}

// patchSheet is the stored state a PATCH reads: the given version, name and
// dexterity, with the derived modifier already consistent with dexterity.
func patchSheet(version int64, name string, dexterity float64) *sheet.Sheet {
	return &sheet.Sheet{
		ID:         patchSheetID,
		TemplateID: patchTemplateID,
		OwnerID:    ownerUserID,
		Version:    version,
		Data: map[string]any{
			"identity":  map[string]any{"characterName": name},
			"stats":     map[string]any{"dexterity": dexterity},
			"modifiers": map[string]any{"dexterityMod": math.Floor((dexterity - 10) / 2)},
		},
	}
}

func dexterityPatch(value float64) sheet.PartialUpdateSheetCommand {
	return sheet.PartialUpdateSheetCommand{
		SheetID:   patchSheetID,
		OwnerID:   ownerUserID,
		RequestID: patchRequestID,
		Data:      map[string]any{"stats": map[string]any{"dexterity": value}},
	}
}

// loggedLine is one structured record the service emitted, keyed by attribute.
type loggedLine struct {
	level string
	msg   string
	attrs map[string]string
}

// newPatchLogger returns a strict logger mock that accepts only the two PATCH
// records with their exact attribute counts. Any other log call fails the test.
// Each accepted call is appended to the returned slice.
func newPatchLogger(t *testing.T) (*logmocks.MockStructuredLogger, *[]loggedLine) {
	t.Helper()
	log := logmocks.NewMockStructuredLogger(t)
	lines := &[]loggedLine{}

	register := func(level, msg string, attrCount int) {
		args := []any{mock.Anything, msg}
		for range attrCount {
			args = append(args, mock.Anything)
		}
		log.On(level, args...).Maybe().Run(func(call mock.Arguments) {
			attrs := make(map[string]string, len(call)-2)
			for _, raw := range call[2:] {
				attr := raw.(logger.Attribute)
				attrs[attr.Key] = attr.Value
			}
			*lines = append(*lines, loggedLine{level: level, msg: msg, attrs: attrs})
		})
	}
	register("Info", "sheet patched", 10)
	register("Warn", "sheet patch version conflict", 9)
	return log, lines
}

func newPatchService(t *testing.T, repo sheet.Repository, log logger.StructuredLogger, dag *template.DAG) sheet.Service {
	t.Helper()
	return sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{patchTemplateID: dag}, log)
}

func TestService_PartialUpdate_CompareAndSwap(t *testing.T) {
	ctx := context.Background()
	dag := patchDAG(t)

	t.Run("Happy path: the first attempt wins, bumps the version and logs the write at Info", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).Return(nil).Once()

		cascade, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"modifiers": map[string]any{"dexterityMod": 4.0}}, cascade)

		require.Len(t, *lines, 1)
		assert.Equal(t, "Info", (*lines)[0].level)
		assert.Equal(t, "sheet patched", (*lines)[0].msg)
		assert.Equal(t, map[string]string{
			"sheet_id":       patchSheetID,
			"template_id":    patchTemplateID,
			"request_id":     patchRequestID,
			"version_before": "3",
			"version_after":  "4",
			"attempt":        "1",
			"delta_paths":    "stats.dexterity",
			"delta_values":   "stats.dexterity=18",
			"cascade_paths":  "modifiers.dexterityMod",
			"cascade_values": "modifiers.dexterityMod=4",
		}, (*lines)[0].attrs)
	})

	t.Run("Edge: a sheet without a version field is written with expected version 0 and logs 0 to 1", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(0, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(0), mock.Anything).Return(nil).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		require.NoError(t, err)
		require.Len(t, *lines, 1)
		assert.Equal(t, "0", (*lines)[0].attrs["version_before"])
		assert.Equal(t, "1", (*lines)[0].attrs["version_after"])
	})

	t.Run("Edge: a text leaf in the delta is logged by path only, never by value", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(0, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(0), mock.Anything).Return(nil).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, sheet.PartialUpdateSheetCommand{
			SheetID:   patchSheetID,
			OwnerID:   ownerUserID,
			RequestID: patchRequestID,
			Data:      map[string]any{"identity": map[string]any{"characterName": "Bryn the Secret"}},
		})
		require.NoError(t, err)
		require.Len(t, *lines, 1)
		assert.Equal(t, "identity.characterName", (*lines)[0].attrs["delta_paths"])
		assert.Empty(t, (*lines)[0].attrs["delta_values"])
		for key, value := range (*lines)[0].attrs {
			assert.NotContains(t, value, "Bryn", "attribute %s leaked a string value", key)
		}
	})

	t.Run("Edge: a lost race re-reads the sheet and recalculates over the new state", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)

		var writtenVersions []int64
		var writtenData []map[string]any
		record := func(_ context.Context, _ string, version int64, updates map[string]any) error {
			writtenVersions = append(writtenVersions, version)
			writtenData = append(writtenData, updates["data"].(map[string]any))
			if len(writtenVersions) == 1 {
				return sheet.ErrVersionConflict
			}
			return nil
		}

		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(4, "Bryn", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).RunAndReturn(record).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(4), mock.Anything).RunAndReturn(record).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		require.NoError(t, err)

		assert.Equal(t, []int64{3, 4}, writtenVersions)
		require.Len(t, writtenData, 2)
		assert.Equal(t, map[string]any{"characterName": "Bryn"}, writtenData[1]["identity"],
			"the retry must keep the name the concurrent writer stored")
		assert.Equal(t, map[string]any{"dexterity": 18.0}, writtenData[1]["stats"])

		require.Len(t, *lines, 2)
		assert.Equal(t, "Warn", (*lines)[0].level)
		assert.Equal(t, "3", (*lines)[0].attrs["version_before"])
		assert.Equal(t, "1", (*lines)[0].attrs["attempt"])
		assert.Equal(t, "Info", (*lines)[1].level)
		assert.Equal(t, "4", (*lines)[1].attrs["version_before"])
		assert.Equal(t, "5", (*lines)[1].attrs["version_after"])
		assert.Equal(t, "2", (*lines)[1].attrs["attempt"])
	})

	t.Run("Edge: the sheet is deleted between a lost race and the retry, so the retry is a 404", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, _ := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).Return(sheet.ErrVersionConflict).Once()
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(nil, sheet.ErrSheetNotFound).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		assert.Equal(t, 404, appErr.HttpStatus())
	})

	t.Run("Error: every attempt loses the race and the request is CONFLICT after three attempts", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		for version := int64(3); version <= 5; version++ {
			repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(version, "Aria", 14), nil).Once()
			repo.EXPECT().PartialUpdate(ctx, patchSheetID, version, mock.Anything).Return(sheet.ErrVersionConflict).Once()
		}

		cascade, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		require.Error(t, err)
		assert.Nil(t, cascade)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeConflict, appErr.Code)
		assert.Equal(t, 409, appErr.HttpStatus())

		require.Len(t, *lines, 3)
		for i, line := range *lines {
			assert.Equal(t, "Warn", line.level, "attempt %d must not log Info", i+1)
			assert.Equal(t, strconv.Itoa(i+1), line.attrs["attempt"])
		}
	})

	t.Run("Error: a storage failure is not retried and stays an internal error", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).Return(errors.New("connection reset")).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInternal, appErr.Code)
		assert.Empty(t, *lines)
	})

	t.Run("Edge: a template with no DAG seen on a fresh read surfaces without a second write", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		replaced := patchSheet(4, "Aria", 14)
		replaced.TemplateID = "gone_v1"
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, patchSheetID, int64(3), mock.Anything).Return(sheet.ErrVersionConflict).Once()
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(replaced, nil).Once()

		_, err := newPatchService(t, repo, log, dag).PartialUpdate(ctx, dexterityPatch(18))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		require.Len(t, *lines, 1)
		assert.Equal(t, "Warn", (*lines)[0].level)
	})
}

func TestService_Preview_NeverWritesOrRetries(t *testing.T) {
	ctx := context.Background()
	dag := patchDAG(t)

	t.Run("Happy path: a preview reads once, returns the cascade and writes nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		log, lines := newPatchLogger(t)
		repo.EXPECT().FetchByID(ctx, patchSheetID).Return(patchSheet(3, "Aria", 14), nil).Once()

		cascade, err := newPatchService(t, repo, log, dag).Preview(ctx, dexterityPatch(18))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"modifiers": map[string]any{"dexterityMod": 4.0}}, cascade)
		assert.Empty(t, *lines)
		repo.AssertNotCalled(t, "PartialUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
