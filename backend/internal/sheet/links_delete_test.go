package sheet_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"legnaris/backend/internal/events"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheettest"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"
)

const (
	deletePublishFirstCampaign  = "01900000-0000-7000-8000-0000000000a1"
	deletePublishSecondCampaign = "01900000-0000-7000-8000-0000000000a3"
	deletePublishLinkOwner      = "01900000-0000-7000-8000-0000000000a2"
)

func deletePublishRemoved(t *testing.T) []sheet.RemovedLink {
	t.Helper()
	return []sheet.RemovedLink{
		{CampaignID: deletePublishFirstCampaign, SheetID: updateSheetID, OwnerID: deletePublishLinkOwner},
		{CampaignID: deletePublishSecondCampaign, SheetID: updateSheetID, OwnerID: deletePublishLinkOwner},
	}
}

func deletePublishService(t *testing.T, repo *mocks.MockRepository, cleaner sheet.LinkCleaner, tx sheet.TransactionRunner) sheet.Service {
	t.Helper()
	repo.EXPECT().FetchByID(mock.Anything, updateSheetID).
		Return(&sheet.Sheet{ID: updateSheetID, OwnerID: ownerUserID, Kind: plan.KindNPC}, nil).Once()
	repo.EXPECT().DeleteByID(mock.Anything, updateSheetID).Return(nil).Maybe()
	return sheet.NewService(repo, templatemocks.NewMockService(t), sheettest.DAGMap{}, testLog, events.NoOp{},
		sheet.WithPlanLimits(planFor(t, ownerUserID, plan.Free), tx),
		sheet.WithLinkCleaner(cleaner),
	)
}

func retryingTransaction(t *testing.T) sheet.TransactionRunner {
	t.Helper()
	tx := mocks.NewMockTransactionRunner(t)
	attempts := 0
	tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			attempts++
			err := fn(ctx)
			if attempts == 1 {
				return fn(ctx)
			}
			return err
		},
	).Once()
	return tx
}

func TestService_Delete_LinkPublishing(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo *mocks.MockRepository, cleaner *mocks.MockLinkCleaner, removed []sheet.RemovedLink) sheet.TransactionRunner
		fails bool
	}{
		{
			name: "Happy path: a committed delete publishes every removed link once",
			setup: func(t *testing.T, repo *mocks.MockRepository, cleaner *mocks.MockLinkCleaner, removed []sheet.RemovedLink) sheet.TransactionRunner {
				cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(removed, nil).Once()
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(nil).Once()
				cleaner.EXPECT().PublishSheetUnlinked(plan.KindNPC, removed).Once()
				return runInline(t)
			},
		},
		{
			name: "Edge: a transient retry publishes once with the successful attempt",
			setup: func(t *testing.T, repo *mocks.MockRepository, cleaner *mocks.MockLinkCleaner, removed []sheet.RemovedLink) sheet.TransactionRunner {
				cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(removed, nil).Twice()
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(errors.New("slot busy")).Once()
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(nil).Once()
				cleaner.EXPECT().PublishSheetUnlinked(plan.KindNPC, removed).Once()
				return retryingTransaction(t)
			},
		},
		{
			name: "Edge: an aborted transaction publishes nothing",
			setup: func(t *testing.T, repo *mocks.MockRepository, cleaner *mocks.MockLinkCleaner, removed []sheet.RemovedLink) sheet.TransactionRunner {
				cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(removed, nil).Once()
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(errors.New("slot busy")).Once()
				return runInline(t)
			},
			fails: true,
		},
		{
			name: "Edge: a delete without links publishes nothing",
			setup: func(t *testing.T, repo *mocks.MockRepository, cleaner *mocks.MockLinkCleaner, _ []sheet.RemovedLink) sheet.TransactionRunner {
				cleaner.EXPECT().DeleteForSheet(mock.Anything, updateSheetID).Return(nil, nil).Once()
				repo.EXPECT().ReleaseSlot(mock.Anything, ownerUserID, plan.KindNPC).Return(nil).Once()
				return runInline(t)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := mocks.NewMockRepository(t)
			cleaner := mocks.NewMockLinkCleaner(t)
			removed := deletePublishRemoved(t)
			tx := tc.setup(t, repo, cleaner, removed)
			svc := deletePublishService(t, repo, cleaner, tx)

			err := svc.Delete(context.Background(), sheet.DeleteSheetCommand{SheetID: updateSheetID, OwnerID: ownerUserID})
			if tc.fails {
				var appErr *apperr.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperr.CodeInternal, appErr.Code)
				cleaner.AssertNotCalled(t, "PublishSheetUnlinked", mock.Anything, mock.Anything)
				return
			}
			require.NoError(t, err)
		})
	}
}
