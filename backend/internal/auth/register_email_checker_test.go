package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	"legnaris/backend/shared/apperr"
)

// errEmailRefused stands in for the disposable checker's refusal cause.
var errEmailRefused = errors.New("e-mail domain cannot receive mail")

func TestService_RegisterEmailChecker(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: an accepted address registers and starts the verify_email step", func(t *testing.T) {
		checker := mocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, testEmail).Return(nil).Once()
		service, m := newAuthServiceWithChecker(t, nil, checker)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.expectRegistrationChallenge(testEmail)

		outcome, err := service.Register(ctx, auth.RegisterCommand{
			Email:    testEmail,
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, outcome)
	})

	t.Run("Happy path: the checker receives the canonical address", func(t *testing.T) {
		checker := mocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, testEmail).Return(nil).Once()
		service, m := newAuthServiceWithChecker(t, nil, checker)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.expectRegistrationChallenge(testEmail)

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "  Player@Example.com ",
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
	})

	t.Run("Error: a refused address answers DISPOSABLE_EMAIL and writes nothing", func(t *testing.T) {
		checker := mocks.NewMockEmailChecker(t)
		checker.EXPECT().Check(mock.Anything, testEmail).Return(errEmailRefused).Once()
		service, _ := newAuthServiceWithChecker(t, nil, checker)

		// No hasher, user, challenge or mailer expectation: any write fails the test.
		outcome, err := service.Register(ctx, auth.RegisterCommand{
			Email:    testEmail,
			Username: "player",
			Password: testPassword,
		})

		require.Nil(t, outcome)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeDisposableEmail, appErr.Code)
		assert.Equal(t, auth.MsgDisposableEmail, appErr.Message)
		assert.ErrorIs(t, err, errEmailRefused)
	})

	t.Run("Edge: a malformed address is rejected as invalid before the checker runs", func(t *testing.T) {
		checker := mocks.NewMockEmailChecker(t)
		service, _ := newAuthServiceWithChecker(t, nil, checker)

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "not-an-address",
			Username: "player",
			Password: testPassword,
		})

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperr.CodeInvalidRequest, appErr.Code)
	})

	t.Run("Edge: a nil checker leaves registration unchanged", func(t *testing.T) {
		service, m := newAuthServiceWithChecker(t, nil, nil)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.expectRegistrationChallenge(testEmail)

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    testEmail,
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
	})
}
