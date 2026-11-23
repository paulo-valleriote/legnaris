package sheet_test

import (
	"context"
	"errors"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errHookRefused = errors.New("hook refused")

func TestService_CreateWithValues(t *testing.T) {
	tests := []struct {
		name        string
		owner       string
		kind        string
		values      map[string]any
		slotErr     error
		hook        sheet.AfterInsertHook
		wantCreate  bool
		wantCode    apperr.Code
		wantStatus  int
		wantDex     any
		wantMod     any
		wantFlag    any
		wantHookRan bool
	}{
		{name: "Error: an empty owner is refused before any read", owner: "", kind: "character", wantCode: apperr.CodeUnauthorized, wantStatus: 401},
		{name: "Error: an unknown kind is refused before any read", owner: ownerUserID, kind: "vehicle", wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a kind the template refuses is rejected", owner: ownerUserID, kind: string(plan.KindNPC), wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a value for an unknown field is rejected", owner: ownerUserID, kind: "character", values: map[string]any{"stats": map[string]any{"nope": 1}}, wantCode: apperr.CodeUnknownField, wantStatus: 400},
		{name: "Error: a flag value of two is rejected", owner: ownerUserID, kind: "character", values: map[string]any{"features": map[string]any{"enabled": 2}}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Error: a fractional flag value is rejected", owner: ownerUserID, kind: "character", values: map[string]any{"features": map[string]any{"enabled": 0.5}}, wantCode: apperr.CodeInvalidRequest, wantStatus: 400},
		{name: "Edge: a flag value of one is accepted", owner: ownerUserID, kind: "character", values: map[string]any{"features": map[string]any{"enabled": 1}}, wantCreate: true, wantDex: 10, wantFlag: 1},
		{name: "Error: a full slot refuses with SHEET_LIMIT_REACHED and writes nothing", owner: ownerUserID, kind: "character", slotErr: sheeterr.ErrSheetLimitReached, wantCode: apperr.CodeSheetLimitReached, wantStatus: 409},
		{name: "Error: a failing hook aborts the Sheet", owner: ownerUserID, kind: "character", hook: func(context.Context, *sheet.Sheet) error { return errHookRefused }, wantCreate: true, wantHookRan: true, wantCode: apperr.CodeInternal, wantStatus: 500},
		{name: "Edge: an empty value set keeps the seeded defaults", owner: ownerUserID, kind: "character", wantCreate: true, wantDex: 10},
		{name: "Edge: a nil hook is allowed", owner: ownerUserID, kind: "character", values: map[string]any{"stats": map[string]any{"dexterity": 14}}, wantCreate: true, wantDex: 14},
		{name: "Edge: a derived value sent back with the editable ones is ignored and recalculated", owner: ownerUserID, kind: "character", values: map[string]any{"stats": map[string]any{"dexterity": 14}, "modifiers": map[string]any{"dexterityMod": 99}}, wantCreate: true, wantDex: 14, wantMod: 2},
		{name: "Edge: only derived values keep the seeded defaults", owner: ownerUserID, kind: "character", values: map[string]any{"modifiers": map[string]any{"dexterityMod": 99}}, wantCreate: true, wantDex: 10, wantMod: 0},
		{name: "Error: an unknown field next to a derived one is still rejected", owner: ownerUserID, kind: "character", values: map[string]any{"modifiers": map[string]any{"dexterityMod": 99, "nope": 1}}, wantCode: apperr.CodeUnknownField, wantStatus: 400},
		{name: "Happy path: values are validated and merged, and the hook runs in the transaction", owner: ownerUserID, kind: "character", values: map[string]any{"stats": map[string]any{"dexterity": 14}}, hook: func(context.Context, *sheet.Sheet) error { return nil }, wantCreate: true, wantHookRan: true, wantDex: 14},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			if tc.wantCreate || tc.slotErr != nil {
				repo.EXPECT().ReserveSlot(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.slotErr).Once()
			}
			if tc.wantCreate && tc.slotErr == nil {
				repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
			}
			svc := newLimitedService(repo, limitTemplateService(t, plan.KindCharacter), limitDAG(t),
				planFor(t, ownerUserID, plan.Free), runInline(t))

			ran := false
			hook := tc.hook
			if tc.wantHookRan {
				hook = func(ctx context.Context, created *sheet.Sheet) error {
					ran = true
					if tc.hook == nil {
						return nil
					}
					return tc.hook(ctx, created)
				}
			}

			created, err := svc.CreateWithValues(context.Background(), sheet.CreateWithValuesCommand{
				TemplateID: limitTemplateID, Kind: tc.kind, SheetName: "hero", OwnerID: tc.owner, Values: tc.values,
			}, hook)
			assert.Equal(t, tc.wantHookRan, ran)

			if tc.wantCode == "" {
				require.NoError(t, err)
				require.NotNil(t, created)
				assert.NotEmpty(t, created.ID)
				assert.EqualValues(t, tc.wantDex, created.Data["stats"].(map[string]any)["dexterity"])
				if tc.wantMod != nil {
					assert.EqualValues(t, tc.wantMod, created.Data["modifiers"].(map[string]any)["dexterityMod"])
				}
				if tc.wantFlag != nil {
					assert.EqualValues(t, tc.wantFlag, created.Data["features"].(map[string]any)["enabled"])
				}
				return
			}

			require.Error(t, err)
			assert.Nil(t, created)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
		})
	}
}
