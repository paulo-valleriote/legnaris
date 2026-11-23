package sheet_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/events"
	eventsmocks "legnaris/backend/internal/events/mocks"
	"legnaris/backend/internal/sheet"
	"legnaris/backend/internal/sheet/mocks"
	"legnaris/backend/internal/sheet/sheeterr"
	"legnaris/backend/internal/sheet/sheettest"
	templatemocks "legnaris/backend/internal/sheet/template/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	resetSheetID    = "01900000-0000-7000-8000-0000000000e1"
	resetRequestID  = "01900000-0000-7000-8000-0000000000c1"
	resetRequestID2 = "01900000-0000-7000-8000-0000000000c2"
	resetForeignOwn = "01900000-0000-7000-8000-0000000000bb"
	resetStoreField = "last_reset_request_id"
)

func resetSheet(version int64, ownerID, lastRequestID string, level float64, items map[string]any) *sheet.Sheet {
	return &sheet.Sheet{
		ID:                 resetSheetID,
		TemplateID:         sheettest.ListTemplateID,
		OwnerID:            ownerID,
		Version:            version,
		LastResetRequestID: lastRequestID,
		Data: map[string]any{
			"level": level,
			"inventory": map[string]any{
				"items": items,
			},
			"inventoryValue": 0.0,
			"inventoryCount": 0.0,
		},
	}
}

func resetCommand(actionID, requestID string) sheet.ResetActionCommand {
	return sheet.ResetActionCommand{
		SheetID:   resetSheetID,
		OwnerID:   ownerUserID,
		ActionID:  actionID,
		RequestID: requestID,
	}
}

func newResetService(t *testing.T, repo sheet.Repository, pub events.Publisher) sheet.Service {
	t.Helper()
	return sheet.NewService(
		repo,
		templatemocks.NewMockService(t),
		sheettest.DAGMap{sheettest.ListTemplateID: sheettest.ListDAG(t)},
		testLog,
		pub,
	)
}

func TestService_Reset(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: a reset writes once and returns the assigned field", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(7, ownerUserID, "", 5, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(7), mock.MatchedBy(func(updates map[string]any) bool {
			data, ok := updates["data"].(map[string]any)
			if !ok {
				return false
			}
			return data["level"] == 4.0 &&
				updates[resetStoreField] == resetRequestID &&
				updates["updated_at"] != nil
		})).Return(int64(8), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"level": 4.0}, effects)
	})

	t.Run("Edge: a replay of the last request id writes nothing and returns 200 with an empty cascade", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).
			Return(resetSheet(8, ownerUserID, resetRequestID, 4, map[string]any{}), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)
		assert.Empty(t, effects)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Edge: a new request id is a new rest", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).
			Return(resetSheet(8, ownerUserID, resetRequestID, 4, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(8), mock.MatchedBy(func(updates map[string]any) bool {
			return updates[resetStoreField] == resetRequestID2
		})).Return(int64(9), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID2))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"level": 3.0}, effects)
	})

	t.Run("Edge: nothing to change is still a 200 and stores the request id", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(1, ownerUserID, "", 0, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(1), mock.MatchedBy(func(updates map[string]any) bool {
			return updates[resetStoreField] == resetRequestID
		})).Return(int64(2), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"level": 0.0}, effects)
	})

	t.Run("Happy path: a list restock returns the assigned field and its derived values", func(t *testing.T) {
		items := map[string]any{
			sheettest.ListRecordID: map[string]any{
				"name": "Rope", "quantity": 1.0, "weight": 1.0, "active": 1.0,
				"optionA": 0.0, "optionB": 0.0, "derivedTotal": 1.0, "scaled": 1.0,
			},
		}
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(2, ownerUserID, "", 1, items), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(2), mock.Anything).Return(int64(3), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionRestock, resetRequestID))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"inventory": map[string]any{
				"items": map[string]any{
					sheettest.ListRecordID: map[string]any{
						"quantity":     2.0,
						"derivedTotal": 2.0,
						"scaled":       2.0,
					},
				},
			},
			"inventoryValue": 2.0,
			"inventoryCount": 1.0,
		}, effects)
	})

	t.Run("Edge: a restock of an empty list returns an empty cascade", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(2, ownerUserID, "", 1, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(2), mock.Anything).Return(int64(3), nil).Once()

		effects, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionRestock, resetRequestID))
		require.NoError(t, err)
		assert.Empty(t, effects)
	})

	t.Run("Error: an unknown action is 404 NOT_FOUND and writes nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(1, ownerUserID, "", 1, map[string]any{}), nil).Once()

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand("ghost", resetRequestID))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: a missing request id is 400 INVALID_REQUEST before any read", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, ""))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		assert.Equal(t, sheet.MsgRequestIDMissing, appErr.Message)
		repo.AssertNotCalled(t, "FetchByID")
	})

	t.Run("Error: a malformed request id is 400 INVALID_REQUEST before any read", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, "not-a-uuid"))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		assert.Equal(t, sheet.MsgRequestIDInvalid, appErr.Message)
		repo.AssertNotCalled(t, "FetchByID")
	})

	t.Run("Error: a malformed sheet id is 400 INVALID_REQUEST before any read", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		cmd := resetCommand(sheettest.ListActionConsume, resetRequestID)
		cmd.SheetID = "not-a-uuid"
		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, cmd)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
		repo.AssertNotCalled(t, "FetchByID")
	})

	t.Run("Error: a foreign sheet is 404 NOT_FOUND and writes nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).
			Return(resetSheet(1, resetForeignOwn, "", 1, map[string]any{}), nil).Once()

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
		repo.AssertNotCalled(t, "PartialUpdate")
	})

	t.Run("Error: a lost version race is 409 CONFLICT with Retry-After 1 second", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(3, ownerUserID, "", 1, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(3), mock.Anything).
			Return(int64(0), sheeterr.ErrVersionConflict).Once()

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeConflict, appErr.Code)
		assert.Equal(t, time.Second, appErr.RetryAfter)
	})

	t.Run("Error: a missing sheet is 404 NOT_FOUND", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(nil, sheeterr.ErrSheetNotFound).Once()

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	})

	t.Run("Error: a failed write maps to 500", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(1, ownerUserID, "", 1, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(1), mock.Anything).Return(int64(0), errors.New("write failed")).Once()

		_, err := newResetService(t, repo, events.NoOp{}).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInternal, appErr.Code)
	})
}

func TestService_Reset_Concurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("Edge: two resets that read the same version apply once and the loser gets 409", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		var (
			mu      sync.Mutex
			version int64
			writes  int
		)
		var arrived sync.WaitGroup
		arrived.Add(2)
		release := make(chan struct{})
		go func() {
			arrived.Wait()
			close(release)
		}()

		repo.EXPECT().FetchByID(mock.Anything, resetSheetID).RunAndReturn(
			func(_ context.Context, _ string) (*sheet.Sheet, error) {
				mu.Lock()
				snapshot := resetSheet(version, ownerUserID, "", 5, map[string]any{})
				mu.Unlock()
				arrived.Done()
				<-release
				return snapshot, nil
			},
		)
		repo.EXPECT().PartialUpdate(mock.Anything, resetSheetID, mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, _ string, expected int64, _ map[string]any) (int64, error) {
				mu.Lock()
				defer mu.Unlock()
				writes++
				if expected != version {
					return 0, sheeterr.ErrVersionConflict
				}
				version++
				return version, nil
			},
		)

		svc := newResetService(t, repo, events.NoOp{})
		errs := make([]error, 2)
		var calls sync.WaitGroup
		calls.Add(2)
		for i, requestID := range []string{resetRequestID, resetRequestID2} {
			go func() {
				defer calls.Done()
				_, errs[i] = svc.Reset(ctx, resetCommand(sheettest.ListActionConsume, requestID))
			}()
		}
		calls.Wait()

		failed := 0
		for _, err := range errs {
			if err == nil {
				continue
			}
			failed++
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeConflict, appErr.Code)
			assert.Equal(t, time.Second, appErr.RetryAfter)
		}
		assert.Equal(t, 1, failed, "exactly one of the two resets must lose the CAS")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 2, writes, "the loser must not retry the write")
		assert.Equal(t, int64(1), version, "only the winner bumps the version")
	})
	t.Run("Edge: a PATCH racing a reset on the same version loses with 409", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)

		var (
			mu      sync.Mutex
			version int64
			writes  int
		)
		var arrived sync.WaitGroup
		arrived.Add(2)
		release := make(chan struct{})
		go func() {
			arrived.Wait()
			close(release)
		}()

		repo.EXPECT().FetchByID(mock.Anything, resetSheetID).RunAndReturn(
			func(_ context.Context, _ string) (*sheet.Sheet, error) {
				mu.Lock()
				snapshot := resetSheet(version, ownerUserID, "", 5, map[string]any{})
				mu.Unlock()
				arrived.Done()
				<-release
				return snapshot, nil
			},
		)
		repo.EXPECT().PartialUpdate(mock.Anything, resetSheetID, mock.Anything, mock.Anything).RunAndReturn(
			func(_ context.Context, _ string, expected int64, _ map[string]any) (int64, error) {
				mu.Lock()
				defer mu.Unlock()
				writes++
				if expected != version {
					return 0, sheeterr.ErrVersionConflict
				}
				version++
				return version, nil
			},
		)

		svc := newResetService(t, repo, events.NoOp{})
		errs := make([]error, 2)
		var calls sync.WaitGroup
		calls.Add(2)
		go func() {
			defer calls.Done()
			_, errs[0] = svc.Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		}()
		go func() {
			defer calls.Done()
			_, errs[1] = svc.PartialUpdate(ctx, sheet.PartialUpdateSheetCommand{
				SheetID:   resetSheetID,
				OwnerID:   ownerUserID,
				RequestID: "req-patch-race",
				Data:      map[string]any{"level": 6.0},
			})
		}()
		calls.Wait()

		failed := 0
		for _, err := range errs {
			if err == nil {
				continue
			}
			failed++
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperr.CodeConflict, appErr.Code)
		}
		assert.Equal(t, 1, failed, "exactly one of the two writes must lose the CAS")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, int64(1), version)
	})
}

func TestService_Reset_EmitsSheetUpdated(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: the event lists the changed target paths and the new version", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		pub := eventsmocks.NewMockPublisher(t)
		got := &[]events.Event{}
		pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
			*got = append(*got, ev)
		}).Once()

		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(4, ownerUserID, "", 5, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(4), mock.Anything).Return(int64(5), nil).Once()

		_, err := newResetService(t, repo, pub).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)

		require.Len(t, *got, 1)
		ev := (*got)[0]
		assert.Equal(t, "sheet.updated", ev.Type)
		assert.Equal(t, "sheet", ev.AggregateType)
		assert.Equal(t, resetSheetID, ev.AggregateID)
		assert.Equal(t, int64(5), ev.AggregateVersion)
		assert.Equal(t, map[string]any{"changed_keys": []string{"level"}, "sheet_version": int64(5)}, ev.Data)
	})

	t.Run("Edge: a replay emits nothing", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		pub := eventsmocks.NewMockPublisher(t)

		repo.EXPECT().FetchByID(ctx, resetSheetID).
			Return(resetSheet(5, ownerUserID, resetRequestID, 4, map[string]any{}), nil).Once()

		_, err := newResetService(t, repo, pub).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)
		pub.AssertNotCalled(t, "Publish", mock.Anything)
	})

	t.Run("Edge: a no-op action still emits with an empty changed-key list", func(t *testing.T) {
		repo := mocks.NewMockRepository(t)
		pub := eventsmocks.NewMockPublisher(t)
		got := &[]events.Event{}
		pub.EXPECT().Publish(mock.Anything).Run(func(ev events.Event) {
			*got = append(*got, ev)
		}).Once()

		repo.EXPECT().FetchByID(ctx, resetSheetID).Return(resetSheet(1, ownerUserID, "", 0, map[string]any{}), nil).Once()
		repo.EXPECT().PartialUpdate(ctx, resetSheetID, int64(1), mock.Anything).Return(int64(2), nil).Once()

		_, err := newResetService(t, repo, pub).Reset(ctx, resetCommand(sheettest.ListActionConsume, resetRequestID))
		require.NoError(t, err)

		require.Len(t, *got, 1)
		assert.Equal(t, map[string]any{"changed_keys": []string{}, "sheet_version": int64(2)}, (*got)[0].Data)
	})
}
