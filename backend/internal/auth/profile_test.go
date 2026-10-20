package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestService_Profile covers the identity read: it reports the account's
// identifier, canonical address, display name and the derived verification
// state, never the stored instant, and it answers the uniform unauthorized
// outcome for an account that no longer exists (TFA-08, TFA-09, TFA-10).
func TestService_Profile(t *testing.T) {
	ctx := context.Background()

	verifiedAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("Happy path: an unverified account reports the derived false state", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{
			ID:       validUserID,
			Email:    testEmail,
			Username: "player",
		}, nil).Once()

		profile, err := service.Profile(ctx, validUserID)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, validUserID, profile.ID)
		assert.Equal(t, testEmail, profile.Email)
		assert.Equal(t, "player", profile.Username)
		assert.False(t, profile.EmailVerified)
	})

	t.Run("Happy path: a verified account reports the derived true state", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{
			ID:              validUserID,
			Email:           testEmail,
			Username:        "player",
			EmailVerifiedAt: &verifiedAt,
		}, nil).Once()

		profile, err := service.Profile(ctx, validUserID)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.True(t, profile.EmailVerified)
	})

	t.Run("Error: an account that no longer exists is unauthorized, never not found", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()

		profile, err := service.Profile(ctx, validUserID)
		require.Error(t, err)
		assert.Nil(t, profile)

		assertAppErr(t, err, apperr.CodeUnauthorized)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgAccountUnavailable, appErr.Message)
		assert.Equal(t, 401, appErr.HttpStatus())
	})

	t.Run("Error: a malformed identity is unauthorized and reaches no store", func(t *testing.T) {
		service, m := newAuthService(t)

		profile, err := service.Profile(ctx, "not-a-uuid")
		require.Error(t, err)
		assert.Nil(t, profile)

		assertAppErr(t, err, apperr.CodeUnauthorized)
		m.users.AssertNotCalled(t, "FetchByID", mock.Anything, mock.Anything)
	})

	t.Run("Error: a store failure is an internal failure, never unauthorized", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, errors.New("mongo down")).Once()

		profile, err := service.Profile(ctx, validUserID)
		require.Error(t, err)
		assert.Nil(t, profile)

		assertAppErr(t, err, apperr.CodeInternal)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInternal, appErr.Message)
		assert.Equal(t, 500, appErr.HttpStatus())
	})
}
