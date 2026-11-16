package billing_test

import (
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/billing"
	"legnaris/backend/internal/billing/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	refundClock      = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	errRefundStorage = errors.New("store down")
)

func paidAt(d time.Duration) *time.Time {
	v := refundClock.Add(-d)
	return &v
}

func activeSub(firstPaid *time.Time) billing.Subscription {
	return billing.Subscription{
		ID:          consumerLocalSub,
		UserID:      consumerUserID,
		Status:      billing.StatusActive,
		FirstPaidAt: firstPaid,
	}
}

func TestService_Refund(t *testing.T) {
	type tc struct {
		name       string
		userID     string
		live       billing.Subscription
		found      bool
		findErr    error
		requestOK  bool
		requestErr error
		skipWrite  bool
		wantCode   apperr.Code
		wantReason string
		wantStatus int
		wantWrite  bool
	}
	tests := []tc{
		{
			name: "Happy path: an active subscription paid one day ago is refunded",
			live: activeSub(paidAt(24 * time.Hour)), found: true, requestOK: true, wantWrite: true,
		},
		{
			name: "Edge: one nanosecond before the 7-day boundary is still inside the window",
			live: activeSub(paidAt(billing.RefundWindow - time.Nanosecond)), found: true, requestOK: true, wantWrite: true,
		},
		{
			name: "Edge: exactly 7 days after the first payment is outside the window",
			live: activeSub(paidAt(billing.RefundWindow)), found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonWindowExpired, wantStatus: 409,
		},
		{
			name: "Edge: a subscription never observed as paid is not eligible",
			live: activeSub(nil), found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonNotSubscribed, wantStatus: 409,
		},
		{
			name:  "Edge: a past_due subscription is not eligible for a refund",
			live:  billing.Subscription{ID: consumerLocalSub, UserID: consumerUserID, Status: billing.StatusPastDue, FirstPaidAt: paidAt(time.Hour)},
			found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonNotSubscribed, wantStatus: 409,
		},
		{
			name: "Edge: a pending subscription is not eligible for a refund",
			live: billing.Subscription{ID: consumerLocalSub, UserID: consumerUserID, Status: billing.StatusPending}, found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonNotSubscribed, wantStatus: 409,
		},
		{
			name:  "Edge: no live subscription is not eligible",
			found: false, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonNotSubscribed, wantStatus: 409,
		},
		{
			name: "Edge: a repeated request while the refund is in progress is already used",
			live: func() billing.Subscription {
				s := activeSub(paidAt(time.Hour))
				s.RefundState = billing.RefundRequested
				return s
			}(), found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonAlreadyUsed, wantStatus: 409,
		},
		{
			name: "Edge: a subscription whose refund right was already used is refused",
			live: func() billing.Subscription {
				s := activeSub(paidAt(time.Hour))
				used := refundClock.Add(-time.Minute)
				s.RefundUsedAt = &used
				return s
			}(), found: true, skipWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonAlreadyUsed, wantStatus: 409,
		},
		{
			name: "Edge: a concurrent request that wins the conditional write makes this one already used",
			live: activeSub(paidAt(time.Hour)), found: true, requestOK: false, wantWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonAlreadyUsed, wantStatus: 409,
		},
		{
			name: "Error: the partial unique index reports the user already used the refund right",
			live: activeSub(paidAt(time.Hour)), found: true, requestErr: billing.ErrRefundAlreadyUsed, wantWrite: true,
			wantCode: apperr.CodeRefundNotEligible, wantReason: billing.ReasonAlreadyUsed, wantStatus: 409,
		},
		{
			name:    "Error: a store failure on the live lookup is an internal error",
			findErr: errRefundStorage, skipWrite: true,
			wantCode: apperr.CodeInternal, wantStatus: 500,
		},
		{
			name: "Error: a store failure on the conditional write is an internal error",
			live: activeSub(paidAt(time.Hour)), found: true, requestErr: errRefundStorage, wantWrite: true,
			wantCode: apperr.CodeInternal, wantStatus: 500,
		},
		{
			name:   "Edge: a malformed user id is refused before the store is read",
			userID: "not-a-uuid", skipWrite: true,
			wantCode: apperr.CodeInvalidRequest, wantStatus: 400,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := mocks.NewMockStore(t)
			userID := tc.userID
			if userID == "" {
				userID = consumerUserID
			}
			if tc.userID == "" {
				store.EXPECT().FindLive(mock.Anything, userID).Return(tc.live, tc.found, tc.findErr).Once()
			}
			if tc.wantWrite {
				store.EXPECT().RequestRefund(mock.Anything, consumerLocalSub, refundClock).Return(tc.requestOK, tc.requestErr).Once()
			}
			svc := billing.NewService(store, nil, billing.Config{Now: func() time.Time { return refundClock }})

			err := svc.Refund(t.Context(), userID)
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HttpStatus())
			if tc.wantReason != "" {
				assert.Equal(t, tc.wantReason, appErr.Details["reason"])
			}
		})
	}

	t.Run("Edge: the refund window opens from the first payment, not from the current period", func(t *testing.T) {
		assert.True(t, billing.RefundOpen(activeSub(paidAt(6*24*time.Hour)), refundClock))
		assert.False(t, billing.RefundOpen(activeSub(paidAt(8*24*time.Hour)), refundClock))
	})

	t.Run("Edge: a subscription whose refund failed no longer has the right open", func(t *testing.T) {
		assert.False(t, billing.RefundOpen(billing.Subscription{Status: billing.StatusActive, FirstPaidAt: paidAt(time.Hour), RefundState: billing.RefundFailed}, refundClock))
	})
}
