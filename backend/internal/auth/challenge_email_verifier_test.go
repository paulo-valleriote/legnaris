package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	verifiedEmail    = "ana@example.com"
	verifiedClientIP = "203.0.113.7"
)

func expectVerifyPath(f *challengeFixture) {
	row := f.validRow(validUserID, otherPurpose)
	f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
}

func verifyCommand() auth.VerifyChallengeCommand {
	return auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode, ClientIP: verifiedClientIP}
}

func TestVerifyChallenge_EmailVerifiedHook(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: the first verification reports the account, email and client address", func(t *testing.T) {
		verifier := mocks.NewMockEmailVerifier(t)
		f := newChallengeFixtureWithVerifier(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy, verifier)
		expectVerifyPath(f)
		f.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Email: verifiedEmail}, nil).Once()
		f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(nil).Once()
		verifier.EXPECT().EmailVerified(mock.Anything, validUserID, verifiedEmail, verifiedClientIP).Once()
		f.expectSignIn()
		f.codeObs.EXPECT().ChallengeVerified(auth.ResultVerified).Once()

		session, err := f.service.VerifyChallenge(ctx, verifyCommand())

		require.NoError(t, err)
		assert.Equal(t, validUserID, session.UserID)
	})

	t.Run("Edge: an already verified account does not report again", func(t *testing.T) {
		verifier := mocks.NewMockEmailVerifier(t)
		f := newChallengeFixtureWithVerifier(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy, verifier)
		expectVerifyPath(f)
		verifiedAt := challengeTestNow.Add(-time.Hour)
		f.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Email: verifiedEmail, EmailVerifiedAt: &verifiedAt}, nil).Once()
		f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(nil).Once()
		f.expectSignIn()
		f.codeObs.EXPECT().ChallengeVerified(auth.ResultVerified).Once()

		_, err := f.service.VerifyChallenge(ctx, verifyCommand())

		require.NoError(t, err)
	})

	t.Run("Edge: a failed account read still verifies and reports nothing", func(t *testing.T) {
		verifier := mocks.NewMockEmailVerifier(t)
		f := newChallengeFixtureWithVerifier(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy, verifier)
		expectVerifyPath(f)
		f.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, errors.New("connection reset")).Once()
		f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(nil).Once()
		f.expectSignIn()
		f.codeObs.EXPECT().ChallengeVerified(auth.ResultVerified).Once()

		session, err := f.service.VerifyChallenge(ctx, verifyCommand())

		require.NoError(t, err)
		assert.Equal(t, validUserID, session.UserID)
	})

	t.Run("Edge: a missing account reports nothing and the write refuses it as before", func(t *testing.T) {
		verifier := mocks.NewMockEmailVerifier(t)
		f := newChallengeFixtureWithVerifier(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy, verifier)
		expectVerifyPath(f)
		f.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()
		f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(user.ErrUserNotFound).Once()

		_, err := f.service.VerifyChallenge(ctx, verifyCommand())

		requireServiceError(t, err, apperr.CodeInternal)
	})

	t.Run("Error: a failed verification write reports nothing", func(t *testing.T) {
		verifier := mocks.NewMockEmailVerifier(t)
		f := newChallengeFixtureWithVerifier(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy, verifier)
		expectVerifyPath(f)
		f.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Email: verifiedEmail}, nil).Once()
		f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(errors.New("write failed")).Once()

		_, err := f.service.VerifyChallenge(ctx, verifyCommand())

		requireServiceError(t, err, apperr.CodeInternal)
	})
}

func requireServiceError(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr), "error must be an apperr, got %T: %v", err, err)
	require.Equal(t, code, appErr.Code)
}
