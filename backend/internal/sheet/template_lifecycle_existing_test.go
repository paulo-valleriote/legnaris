package sheet_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func lifecycleStoredSheet() *sheet.Sheet {
	return &sheet.Sheet{
		ID:         updateSheetID,
		TemplateID: limitTemplateID,
		OwnerID:    ownerUserID,
		Kind:       plan.KindCharacter,
		Name:       "Hero",
		Version:    3,
		Data: map[string]any{
			"stats":     map[string]any{"dexterity": 10.0},
			"modifiers": map[string]any{"dexterityMod": 0.0},
			"features":  map[string]any{"enabled": 0.0},
		},
	}
}

func TestService_ExistingSheetKeepsWorkingByTemplateState(t *testing.T) {
	tests := []struct {
		name  string
		state template.State
	}{
		{name: "Happy path: a Draft Template outside development still reads, edits and rolls", state: template.StateDraft},
		{name: "Happy path: a Retired Template still reads, edits and rolls", state: template.StateRetired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("read", func(t *testing.T) {
				repo := mocks.NewMockRepository(t)
				repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(lifecycleStoredSheet(), nil).Once()
				svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), false)

				stored, err := svc.FetchByID(context.Background(), updateSheetID, ownerUserID)

				require.NoError(t, err)
				require.NotNil(t, stored)
				assert.Equal(t, limitTemplateID, stored.TemplateID)
			})

			t.Run("edit", func(t *testing.T) {
				repo := mocks.NewMockRepository(t)
				repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(lifecycleStoredSheet(), nil).Once()
				repo.EXPECT().PartialUpdate(mock.Anything, updateSheetID, int64(3), mock.Anything).Return(int64(4), nil).Once()
				svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), false)

				cascade, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
					SheetID: updateSheetID,
					OwnerID: ownerUserID,
					Data:    map[string]any{"stats": map[string]any{"dexterity": 14}},
				})

				require.NoError(t, err)
				assert.InDelta(t, 2.0, cascade["modifiers"].(map[string]any)["dexterityMod"], 1e-9)
			})

			t.Run("roll", func(t *testing.T) {
				repo := mocks.NewMockRepository(t)
				repo.EXPECT().FetchByID(mock.Anything, updateSheetID).Return(lifecycleStoredSheet(), nil).Once()
				repo.EXPECT().PartialUpdate(mock.Anything, updateSheetID, int64(3), mock.Anything).Return(int64(4), nil).Once()
				svc := newLifecycleService(t, repo, lifecycleTemplateService(t, tc.state), false)
				expression := "1d20 + @"

				cascade, err := svc.PartialUpdate(context.Background(), sheet.PartialUpdateSheetCommand{
					SheetID: updateSheetID,
					OwnerID: ownerUserID,
					Dice: &sheet.Dice{
						Row: map[string]*string{"stats.dexterity": &expression},
					},
				})

				require.NoError(t, err)
				assert.Empty(t, cascade)
			})
		})
	}
}
