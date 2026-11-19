package quota_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/quota/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const validUserID = "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f"

var errStoreFailure = errors.New("connection reset")

type txKey struct{}

func sessionCtx() context.Context {
	return context.WithValue(context.Background(), txKey{}, "session")
}

func isSessionCtx() any {
	return mock.MatchedBy(func(ctx context.Context) bool {
		return ctx.Value(txKey{}) == "session"
	})
}

func newService(t *testing.T, now time.Time) (*quota.Service, *mocks.MockStore) {
	t.Helper()
	store := mocks.NewMockStore(t)
	return quota.NewService(store, func() time.Time { return now }), store
}

func requireAppErr(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	_ = appErrOf(t, err, code)
}

func appErrOf(t *testing.T, err error, code apperr.Code) *apperr.Error {
	t.Helper()
	require.Error(t, err)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr, "error must be an apperr, got %T: %v", err, err)
	require.Equal(t, code, appErr.Code)
	return appErr
}

var invalidUserIDs = []struct {
	name string
	id   string
}{
	{name: "empty id", id: ""},
	{name: "not a UUID", id: "not-a-uuid"},
	{name: "uppercase form of a valid id", id: "0196E4F2-6A1B-7C3D-8E4F-5A6B7C8D9E0F"},
	{name: "urn prefix", id: "urn:uuid:" + validUserID},
	{name: "UUID v4", id: "550e8400-e29b-41d4-a716-446655440000"},
}

func TestServiceReserveTrial(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	for _, tc := range invalidUserIDs {
		t.Run("Edge: invalid id ("+tc.name+") is refused before storage", func(t *testing.T) {
			svc, _ := newService(t, now)
			err := svc.ReserveTrial(context.Background(), tc.id)
			requireAppErr(t, err, apperr.CodeUnauthorized)
		})
	}

	tests := []struct {
		name       string
		reserved   bool
		storeErr   error
		wantCode   apperr.Code
		wantStatus int
	}{
		{name: "Happy path: an unused allowance is reserved", reserved: true},
		{name: "Edge: an exhausted or missing allowance is PLAN_UPGRADE_REQUIRED", reserved: false, wantCode: apperr.CodePlanUpgradeRequired, wantStatus: 403},
		{name: "Error: a store failure is internal", storeErr: errStoreFailure, wantCode: apperr.CodeInternal, wantStatus: 500},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newService(t, now)
			store.EXPECT().ReserveTrial(isSessionCtx(), validUserID).Return(tc.reserved, tc.storeErr).Once()

			err := svc.ReserveTrial(sessionCtx(), validUserID)
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			appErr := appErrOf(t, err, tc.wantCode)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
		})
	}
}

func TestServiceRefundTrial(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	for _, tc := range invalidUserIDs {
		t.Run("Edge: invalid id ("+tc.name+") is refused before storage", func(t *testing.T) {
			svc, _ := newService(t, now)
			err := svc.RefundTrial(context.Background(), tc.id)
			requireAppErr(t, err, apperr.CodeUnauthorized)
		})
	}

	t.Run("Happy path: the refund reaches the store with the session context", func(t *testing.T) {
		svc, store := newService(t, now)
		store.EXPECT().RefundTrial(isSessionCtx(), validUserID).Return(nil).Once()
		require.NoError(t, svc.RefundTrial(sessionCtx(), validUserID))
	})

	t.Run("Error: a store failure is internal", func(t *testing.T) {
		svc, store := newService(t, now)
		store.EXPECT().RefundTrial(mock.Anything, validUserID).Return(errStoreFailure).Once()
		err := svc.RefundTrial(context.Background(), validUserID)
		requireAppErr(t, err, apperr.CodeInternal)
		assert.ErrorIs(t, err, errStoreFailure)
	})
}

func TestServiceReserveDaily(t *testing.T) {
	tests := []struct {
		name        string
		now         time.Time
		wantDay     string
		wantExpires time.Time
		reserved    bool
		storeErr    error
		wantCode    apperr.Code
		wantStatus  int
		wantRetry   time.Duration
	}{
		{
			name:        "Happy path: a reservation opens the UTC day and keeps its counter 48 h",
			now:         time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			wantDay:     "2026-11-14",
			wantExpires: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			reserved:    true,
		},
		{
			name:        "Edge: 23:59:59.999 belongs to the closing day",
			now:         time.Date(2026, 11, 14, 23, 59, 59, 999_000_000, time.UTC),
			wantDay:     "2026-11-14",
			wantExpires: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			reserved:    true,
		},
		{
			name:        "Edge: exactly midnight opens the next day",
			now:         time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			wantDay:     "2026-11-15",
			wantExpires: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC),
			reserved:    true,
		},
		{
			name:        "Edge: a local time zone is converted to UTC before the day is chosen",
			now:         time.Date(2026, 11, 14, 22, 30, 0, 0, time.FixedZone("UTC-3", -3*60*60)),
			wantDay:     "2026-11-15",
			wantExpires: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC),
			reserved:    true,
		},
		{
			name:        "Edge: a full day is QUOTA_EXCEEDED with Retry-After until the next UTC midnight",
			now:         time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			wantDay:     "2026-11-14",
			wantExpires: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			reserved:    false,
			wantCode:    apperr.CodeQuotaExceeded,
			wantStatus:  429,
			wantRetry:   14 * time.Hour,
		},
		{
			name:        "Edge: a full day one second before midnight waits one second",
			now:         time.Date(2026, 11, 14, 23, 59, 59, 0, time.UTC),
			wantDay:     "2026-11-14",
			wantExpires: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			reserved:    false,
			wantCode:    apperr.CodeQuotaExceeded,
			wantStatus:  429,
			wantRetry:   time.Second,
		},
		{
			name:        "Error: a store failure is internal and carries no Retry-After",
			now:         time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			wantDay:     "2026-11-14",
			wantExpires: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			storeErr:    errStoreFailure,
			wantCode:    apperr.CodeInternal,
			wantStatus:  500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newService(t, tc.now)
			store.EXPECT().
				ReserveDaily(isSessionCtx(), validUserID, tc.wantDay, tc.wantExpires).
				Return(tc.reserved, tc.storeErr).
				Once()

			day, err := svc.ReserveDaily(sessionCtx(), validUserID)
			if tc.wantCode == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.wantDay, day)
				return
			}
			appErr := appErrOf(t, err, tc.wantCode)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
			assert.Equal(t, tc.wantRetry, appErr.RetryAfter)
			assert.Empty(t, day)
		})
	}

	for _, tc := range invalidUserIDs {
		t.Run("Edge: invalid id ("+tc.name+") is refused before storage", func(t *testing.T) {
			svc, _ := newService(t, time.Now())
			_, err := svc.ReserveDaily(context.Background(), tc.id)
			requireAppErr(t, err, apperr.CodeUnauthorized)
		})
	}
}

func TestServiceRefundDaily(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 5, 0, 0, time.UTC)

	invalidDays := []struct {
		name string
		day  string
	}{
		{name: "empty", day: ""},
		{name: "unpadded month", day: "2026-1-14"},
		{name: "timestamp instead of a day", day: "2026-11-14T00:00:00Z"},
		{name: "impossible date", day: "2026-02-30"},
		{name: "day first", day: "14/11/2026"},
	}
	for _, tc := range invalidDays {
		t.Run("Edge: malformed day ("+tc.name+") is refused before storage", func(t *testing.T) {
			svc, _ := newService(t, now)
			err := svc.RefundDaily(context.Background(), validUserID, tc.day)
			requireAppErr(t, err, apperr.CodeInvalidRequest)
		})
	}

	t.Run("Happy path: a refund after midnight lands on the reserved day", func(t *testing.T) {
		svc, store := newService(t, now)
		store.EXPECT().RefundDaily(isSessionCtx(), validUserID, "2026-11-14").Return(nil).Once()
		require.NoError(t, svc.RefundDaily(sessionCtx(), validUserID, "2026-11-14"))
	})

	t.Run("Error: a store failure is internal", func(t *testing.T) {
		svc, store := newService(t, now)
		store.EXPECT().RefundDaily(mock.Anything, validUserID, "2026-11-14").Return(errStoreFailure).Once()
		requireAppErr(t, svc.RefundDaily(context.Background(), validUserID, "2026-11-14"), apperr.CodeInternal)
	})

	for _, tc := range invalidUserIDs {
		t.Run("Edge: invalid id ("+tc.name+") is refused before storage", func(t *testing.T) {
			svc, _ := newService(t, now)
			requireAppErr(t, svc.RefundDaily(context.Background(), tc.id, "2026-11-14"), apperr.CodeUnauthorized)
		})
	}
}

func TestServiceTrialStatus(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		row       quota.TrialRow
		storeErr  error
		want      quota.TrialStatus
		wantCode  apperr.Code
		wantError bool
	}{
		{
			name: "Happy path: one generation used of three",
			row:  quota.TrialRow{UserID: validUserID, Allowance: 3, Used: 1},
			want: quota.TrialStatus{Allowance: 3, Used: 1, Remaining: 2},
		},
		{
			name: "Edge: an exhausted allowance has no remaining generations",
			row:  quota.TrialRow{UserID: validUserID, Allowance: 3, Used: 3},
			want: quota.TrialStatus{Allowance: 3, Used: 3, Remaining: 0},
		},
		{
			name: "Edge: a missing row reads as no allowance",
			row:  quota.TrialRow{},
			want: quota.TrialStatus{},
		},
		{
			name: "Edge: a counter above its allowance never reports negative remaining",
			row:  quota.TrialRow{UserID: validUserID, Allowance: 3, Used: 5},
			want: quota.TrialStatus{Allowance: 3, Used: 5, Remaining: 0},
		},
		{
			name:      "Error: a store failure is internal",
			storeErr:  errStoreFailure,
			wantCode:  apperr.CodeInternal,
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newService(t, now)
			store.EXPECT().FetchTrial(mock.Anything, validUserID).Return(tc.row, tc.storeErr).Once()

			got, err := svc.TrialStatus(context.Background(), validUserID)
			if tc.wantError {
				requireAppErr(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("Edge: invalid id is refused before storage", func(t *testing.T) {
		svc, _ := newService(t, now)
		_, err := svc.TrialStatus(context.Background(), "")
		requireAppErr(t, err, apperr.CodeUnauthorized)
	})
}

func TestServiceDailyStatus(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		used      int
		storeErr  error
		want      quota.DailyStatus
		wantError bool
	}{
		{
			name: "Happy path: five of thirty used, reset at the next UTC midnight",
			now:  time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			used: 5,
			want: quota.DailyStatus{
				Limit: 30, Used: 5, Remaining: 25,
				ResetsAt: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Edge: a full day has no remaining generations",
			now:  time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			used: 30,
			want: quota.DailyStatus{
				Limit: 30, Used: 30, Remaining: 0,
				ResetsAt: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Edge: no counter row for today reads as an unused day",
			now:  time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			used: 0,
			want: quota.DailyStatus{
				Limit: 30, Used: 0, Remaining: 30,
				ResetsAt: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Edge: one second before midnight still reports the closing day's reset",
			now:  time.Date(2026, 11, 14, 23, 59, 59, 0, time.UTC),
			used: 12,
			want: quota.DailyStatus{
				Limit: 30, Used: 12, Remaining: 18,
				ResetsAt: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Error: a store failure is internal",
			now:       time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
			storeErr:  errStoreFailure,
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newService(t, tc.now)
			store.EXPECT().FetchDaily(mock.Anything, validUserID, mock.AnythingOfType("string")).Return(tc.used, tc.storeErr).Once()

			got, err := svc.DailyStatus(context.Background(), validUserID)
			if tc.wantError {
				requireAppErr(t, err, apperr.CodeInternal)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("Edge: invalid id is refused before storage", func(t *testing.T) {
		svc, _ := newService(t, time.Now())
		_, err := svc.DailyStatus(context.Background(), "")
		requireAppErr(t, err, apperr.CodeUnauthorized)
	})
}
