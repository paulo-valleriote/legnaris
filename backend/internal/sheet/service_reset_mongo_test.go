package sheet_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/sheettest"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_Reset_MongoWritesOnce(t *testing.T) {
	ctx := context.Background()
	const sheetID = "01900000-0000-7000-8000-0000000000f1"

	repo := newRepositoryTest(t)
	pub := eventsmocks.NewMockPublisher(t)
	got := &[]events.Event{}
	pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
		*got = append(*got, ev)
	}).Maybe()

	tmpl := sheettest.ListTemplate()
	templateSvc := templatemocks.NewMockService(t)
	templateSvc.EXPECT().
		FetchTemplate(mock.Anything, mock.Anything).
		Return(&tmpl, nil).
		Maybe()

	svc := sheet.NewService(
		repo,
		templateSvc,
		sheettest.DAGMap{sheettest.ListTemplateID: sheettest.ListDAG(t)},
		testLog,
		pub,
	)

	items := map[string]any{
		sheettest.ListRecordID: map[string]any{
			"name": "Rope", "quantity": 1.0, "weight": 1.0, "active": 1.0,
			"optionA": 0.0, "optionB": 0.0, "derivedTotal": 1.0, "scaled": 1.0,
		},
	}
	require.NoError(t, repo.Create(ctx, &sheet.Sheet{
		ID:         sheetID,
		TemplateID: sheettest.ListTemplateID,
		OwnerID:    ownerUserID,
		Name:       "Reset",
		Data: map[string]any{
			"level": 1.0,
			"inventory": map[string]any{
				"items": items,
			},
			"inventoryValue": 1.0,
			"inventoryCount": 1.0,
		},
	}))

	effects, err := svc.Reset(ctx, sheet.ResetActionCommand{
		SheetID:   sheetID,
		OwnerID:   ownerUserID,
		ActionID:  sheettest.ListActionRestock,
		RequestID: resetRequestID,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, effects)

	stored, err := repo.FetchByID(ctx, sheetID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stored.Version, "one reset is exactly one compare-and-swap write")
	assert.Equal(t, resetRequestID, stored.LastResetRequestID)
	record, ok := stored.Data["inventory"].(map[string]any)["items"].(map[string]any)[sheettest.ListRecordID].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 2.0, record["quantity"], 1e-9)
	assert.InDelta(t, 2.0, record["derivedTotal"], 1e-9)
	assert.InDelta(t, 2.0, stored.Data["inventoryValue"], 1e-9)
	updatedAt := stored.UpdatedAt

	require.Len(t, *got, 1)
	event := (*got)[0]
	assert.Equal(t, "sheet.updated", event.Type)
	assert.Equal(t, int64(1), event.AggregateVersion)
	assert.Equal(t, map[string]any{
		"changed_keys":  []string{sheettest.ListPath + "." + sheettest.ListRecordID + ".quantity"},
		"sheet_version": int64(1),
	}, event.Data)

	replayed, err := svc.Reset(ctx, sheet.ResetActionCommand{
		SheetID:   sheetID,
		OwnerID:   ownerUserID,
		ActionID:  sheettest.ListActionRestock,
		RequestID: resetRequestID,
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"inventory": map[string]any{
			"items": map[string]any{
				sheettest.ListRecordID: map[string]any{
					"quantity": 2.0, "derivedTotal": 2.0, "scaled": 2.0,
				},
			},
		},
		"inventoryValue": 2.0,
		"inventoryCount": 1.0,
	}, replayed)

	stored, err = repo.FetchByID(ctx, sheetID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stored.Version, "a replay must not write")
	assert.Equal(t, updatedAt, stored.UpdatedAt)
	assert.Len(t, *got, 1, "a replay must not emit")

	_, err = svc.Reset(ctx, sheet.ResetActionCommand{
		SheetID:   sheetID,
		OwnerID:   ownerUserID,
		ActionID:  sheettest.ListActionRestock,
		RequestID: resetRequestID2,
	})
	require.NoError(t, err)

	stored, err = repo.FetchByID(ctx, sheetID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stored.Version)
	assert.Equal(t, resetRequestID2, stored.LastResetRequestID)
	record, ok = stored.Data["inventory"].(map[string]any)["items"].(map[string]any)[sheettest.ListRecordID].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 3.0, record["quantity"], 1e-9)
	assert.Len(t, *got, 2)
}
