package sheet_test

import (
	"context"
	"strings"
	"testing"

	"legnaris/backend/internal/engine"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	"legnaris/backend/internal/sheet/template"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/maputils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	fifthResetTemplateID = "fifth_character_v1"
	fifthResetActionID   = "long_rest"

	fifthResetShortRecordID = "r0123456789abcdef0123456789abcdef"
	fifthResetLongRecordID  = "r0123456789abcdef0123456789abcdee"
	fifthResetNoneRecordID  = "r0123456789abcdef0123456789abcde1"
)

func fifthSpentCharacterData(t *testing.T, tmpl template.Template, dag *template.DAG) map[string]any {
	t.Helper()

	seeded := sheettest.SeedTemplateData(tmpl.Fields)
	flat, err := maputils.FlattenPathKeys(sheettest.SeedTemplateData(tmpl.Fields))
	require.NoError(t, err)
	for key, value := range flat {
		if _, isList := value.(map[string]any); isList {
			delete(flat, key)
		}
	}
	derived, err := dag.Recalculate(map[string]any{}, flat)
	require.NoError(t, err)
	require.NoError(t, maputils.ApplyUpdates(seeded, derived))

	calculation, err := dag.CalculateSave(seeded, map[string]any{
		"identity.level":                                      float64(5),
		"abilities.constitution.score":                        float64(14),
		"combat.hp.base":                                      float64(10),
		"combat.hp.perLevel":                                  float64(6),
		"combat.hp.misc":                                      float64(5),
		"combat.hp.current":                                   float64(4),
		"hitDice.primary.spent":                               float64(2),
		"hitDice.d6.extra":                                    float64(2),
		"hitDice.d6.spent":                                    float64(2),
		"hitDice.d8.extra":                                    float64(1),
		"hitDice.d8.spent":                                    float64(1),
		"exhaustion.level":                                    float64(3),
		"spellcasting.pact.count":                             float64(2),
		"spellcasting.pact.level":                             float64(3),
		"spellcasting.pact.used":                              float64(2),
		"spellcasting.slot1.total":                            float64(4),
		"spellcasting.slot1.spent":                            float64(3),
		"status.deathSaves.successes":                         float64(2),
		"status.deathSaves.failures":                          float64(1),
		"resources.list." + fifthResetShortRecordID + ".max":  float64(3),
		"resources.list." + fifthResetShortRecordID + ".used": float64(2),
		"resources.list." + fifthResetShortRecordID + ".rechargeShort": float64(1),
		"resources.list." + fifthResetLongRecordID + ".max":            float64(2),
		"resources.list." + fifthResetLongRecordID + ".used":           float64(1),
		"resources.list." + fifthResetLongRecordID + ".rechargeLong":   float64(1),
		"resources.list." + fifthResetNoneRecordID + ".max":            float64(5),
		"resources.list." + fifthResetNoneRecordID + ".used":           float64(3),
		"resources.list." + fifthResetNoneRecordID + ".rechargeNone":   float64(1),
	})
	require.NoError(t, err)
	return calculation.Merged
}

func fifthResetValue(t *testing.T, effects map[string]any, path string) float64 {
	t.Helper()
	value, ok := maputils.Get(effects, strings.Split(path, "."))
	require.True(t, ok, "missing effect %s", path)
	number, ok := maputils.ToFloat64(value)
	require.True(t, ok, "effect %s is not numeric", path)
	return number
}

func TestService_Reset_FifthCharacterReplay(t *testing.T) {
	ctx := context.Background()

	registry, err := template.BuildRegistry(engine.NewFormulaEngine())
	require.NoError(t, err)
	tmpl, ok := registry.FetchByID(fifthResetTemplateID)
	require.True(t, ok)
	dag, ok := registry.DAG(fifthResetTemplateID)
	require.True(t, ok)

	spent := &sheet.Sheet{
		ID:         resetSheetID,
		TemplateID: fifthResetTemplateID,
		OwnerID:    ownerUserID,
		Version:    4,
		Data:       fifthSpentCharacterData(t, *tmpl, dag),
	}

	repo := mocks.NewMockRepository(t)
	var stored map[string]any
	repo.EXPECT().FetchByID(ctx, resetSheetID).Return(spent, nil).Once()
	repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(4), mock.MatchedBy(func(updates map[string]any) bool {
		data, ok := updates["data"].(map[string]any)
		if !ok {
			return false
		}
		stored = data
		return updates[resetStoreField] == resetRequestID
	})).Return(int64(5), nil).Once()

	svc := sheet.NewService(
		repo,
		templatemocks.NewMockService(t),
		registry,
		testLog,
		events.NoOp{},
	)
	first, err := svc.Reset(ctx, resetCommand(fifthResetActionID, resetRequestID))
	require.NoError(t, err)
	assert.InDelta(t, 49.0, fifthResetValue(t, first, "combat.hp.current"), 1e-9)
	assert.InDelta(t, 2.0, fifthResetValue(t, first, "exhaustion.level"), 1e-9)
	require.NotNil(t, stored)

	rested := &sheet.Sheet{
		ID:                 resetSheetID,
		TemplateID:         fifthResetTemplateID,
		OwnerID:            ownerUserID,
		Version:            5,
		LastResetRequestID: resetRequestID,
		Data:               stored,
	}
	repo.EXPECT().FetchByID(ctx, resetSheetID).Return(rested, nil).Once()

	replay, err := svc.Reset(ctx, resetCommand(fifthResetActionID, resetRequestID))
	require.NoError(t, err)
	repo.AssertNumberOfCalls(t, "PartialUpdate", 1)

	assert.InDelta(t, 49.0, fifthResetValue(t, replay, "combat.hp.current"), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(t, replay, "hitDice.primary.spent"), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(t, replay, "spellcasting.slot1.spent"), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(t, replay, "spellcasting.pact.used"), 1e-9)
	assert.InDelta(t, 2.0, fifthResetValue(t, replay, "exhaustion.level"), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(t, replay, "status.deathSaves.successes"), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(
		t,
		replay,
		"resources.list."+fifthResetShortRecordID+".used",
	), 1e-9)
	assert.InDelta(t, 0.0, fifthResetValue(
		t,
		replay,
		"resources.list."+fifthResetLongRecordID+".used",
	), 1e-9)
	assert.InDelta(t, 3.0, fifthResetValue(
		t,
		replay,
		"resources.list."+fifthResetNoneRecordID+".used",
	), 1e-9)
}
