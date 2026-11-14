package quota_test

import (
	"context"
	"testing"

	"legnaris/backend/internal/quota"
	"legnaris/backend/internal/quota/mocks"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTrialGranterGrant(t *testing.T) {
	for _, tc := range invalidUserIDs {
		t.Run("Edge: invalid id ("+tc.name+") is refused before storage", func(t *testing.T) {
			store := mocks.NewMockStore(t)

			err := quota.NewTrialGranter(store).Grant(context.Background(), tc.id)

			requireAppErr(t, err, apperr.CodeUnauthorized)
		})
	}

	t.Run("Happy path: grants the lifetime allowance to the account", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().GrantTrial(mock.Anything, validUserID, quota.TrialAllowance).Return(nil).Once()

		require.NoError(t, quota.NewTrialGranter(store).Grant(context.Background(), validUserID))
	})

	t.Run("Error: a store failure is hidden behind an internal error", func(t *testing.T) {
		store := mocks.NewMockStore(t)
		store.EXPECT().GrantTrial(mock.Anything, validUserID, quota.TrialAllowance).Return(errStoreFailure).Once()

		err := quota.NewTrialGranter(store).Grant(context.Background(), validUserID)

		appErr := appErrOf(t, err, apperr.CodeInternal)
		assert.Equal(t, quota.MsgQuotaUnavailable, appErr.Message)
		assert.ErrorIs(t, err, errStoreFailure)
	})
}
