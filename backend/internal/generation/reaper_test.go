package generation_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/backend/internal/generation"
	"legnaris/backend/internal/generation/mocks"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var reapNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func reapDeps(t *testing.T) (generation.ReapDeps, *mocks.MockLeaseStore, *mocks.MockQuota) {
	t.Helper()
	store := mocks.NewMockLeaseStore(t)
	quota := mocks.NewMockQuota(t)
	tx := mocks.NewMockTransactor(t)
	tx.EXPECT().WithTransaction(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
	store.EXPECT().OldestRunning(mock.Anything).Return(time.Time{}, false, nil).Maybe()
	return generation.ReapDeps{Store: store, Quota: quota, Tx: tx}, store, quota
}

func TestNewReapQueue(t *testing.T) {
	tests := []struct {
		name       string
		attemptCap int
		batch      int
		wantErr    bool
	}{
		{name: "Happy path: a valid attempt cap and batch build the queue", attemptCap: 5, batch: 10},
		{name: "Error: a zero attempt cap is refused", attemptCap: 0, batch: 10, wantErr: true},
		{name: "Error: a zero batch is refused", attemptCap: 5, batch: 0, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _ := reapDeps(t)
			queue, err := generation.NewReapQueue(deps, tc.attemptCap, tc.batch)
			if tc.wantErr {
				require.ErrorIs(t, err, sweeper.ErrInvalidQueue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, generation.ReapQueueName, queue.Name)
			assert.Equal(t, generation.ReapInterval, queue.Interval)
		})
	}
}

func TestReapQueue_Run(t *testing.T) {
	expired := generation.Document{ID: jobID, UserID: userID, Status: generation.StatusRunning,
		ReapAttempts: 2, Quota: generation.Reservation{Kind: generation.ReservationDaily, DayUTC: "2026-11-14"}}

	t.Run("Happy path: an expired job is failed as lease_expired and its daily unit refunded", func(t *testing.T) {
		deps, store, quota := reapDeps(t)
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 10).Return([]generation.Document{expired}, nil).Once()
		store.EXPECT().FailExpired(mock.Anything, jobID, reapNow, generation.ErrorCodeLeaseExpired).
			Return(expired, true, nil).Once()
		quota.EXPECT().RefundDaily(mock.Anything, userID, "2026-11-14").Return(nil).Once()

		queue, err := generation.NewReapQueue(deps, 5, 10)
		require.NoError(t, err)
		runQueueOnce(t, queue, reapNow)
	})

	t.Run("Edge: a job settled elsewhere between claim and fail refunds nothing", func(t *testing.T) {
		deps, store, _ := reapDeps(t)
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 10).Return([]generation.Document{expired}, nil).Once()
		store.EXPECT().FailExpired(mock.Anything, jobID, reapNow, generation.ErrorCodeLeaseExpired).
			Return(generation.Document{}, false, nil).Once()

		queue, err := generation.NewReapQueue(deps, 5, 10)
		require.NoError(t, err)
		runQueueOnce(t, queue, reapNow)
	})

	t.Run("Edge: no expired job means nothing is failed", func(t *testing.T) {
		deps, store, _ := reapDeps(t)
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 10).Return(nil, nil).Once()

		queue, err := generation.NewReapQueue(deps, 5, 10)
		require.NoError(t, err)
		runQueueOnce(t, queue, reapNow)
	})

	t.Run("Error: a failed fail-write is rescheduled, not lost", func(t *testing.T) {
		deps, store, _ := reapDeps(t)
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 10).Return([]generation.Document{expired}, nil).Once()
		store.EXPECT().FailExpired(mock.Anything, jobID, reapNow, generation.ErrorCodeLeaseExpired).
			Return(generation.Document{}, false, errors.New("mongo down")).Once()
		store.EXPECT().RescheduleExpired(mock.Anything, jobID, mock.Anything).Return(nil).Once()

		queue, err := generation.NewReapQueue(deps, 5, 10)
		require.NoError(t, err)
		runQueueOnce(t, queue, reapNow)
	})
}

func runQueueOnce(t *testing.T, queue sweeper.Queue, now time.Time) {
	t.Helper()
	sw, err := sweeper.New([]sweeper.Queue{queue}, "test-claimer", noopObserver{}, logger.NewLogger(slog.LevelError))
	require.NoError(t, err)
	require.NoError(t, sw.RunOnce(context.Background(), now))
}

func TestReapStore(t *testing.T) {
	t.Run("Happy path: claimed documents carry their reap attempts into the job", func(t *testing.T) {
		deps, store, _ := reapDeps(t)
		doc := generation.Document{ID: jobID, ReapAttempts: 3}
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 4).Return([]generation.Document{doc}, nil).Once()
		store.EXPECT().FailExpired(mock.Anything, jobID, reapNow, generation.ErrorCodeLeaseExpired).
			Return(generation.Document{}, false, nil).Once()

		queue, err := generation.NewReapQueue(deps, 5, 4)
		require.NoError(t, err)
		assert.Equal(t, generation.ReapQueueName, queue.Name)
		runQueueOnce(t, queue, reapNow)
	})

	t.Run("Error: a store failure on claim is reported, not swallowed", func(t *testing.T) {
		deps, store, _ := reapDeps(t)
		store.EXPECT().ClaimExpired(mock.Anything, reapNow, sweeper.Lease, 4).Return(nil, errors.New("down")).Once()

		queue, err := generation.NewReapQueue(deps, 5, 4)
		require.NoError(t, err)
		sw, err := sweeper.New([]sweeper.Queue{queue}, "test-claimer", noopObserver{}, logger.NewLogger(slog.LevelError))
		require.NoError(t, err)
		assert.Error(t, sw.RunOnce(context.Background(), reapNow))
	})
}
