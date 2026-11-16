package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
)

func TestService_RequestAccountDeletion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		userID   string
		setup    func(m *authMocks)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: an active account receives the deletion code at its address",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Email: testEmail}, nil).Once()
				m.expectChallengeIssuance(validUserID, auth.PurposeDeleteAccount, testEmail)
			},
		},
		{
			name:     "Edge: a malformed identity is refused before the store",
			userID:   "not-a-uuid",
			setup:    func(m *authMocks) {},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:   "Error: a deleting account gets no second code",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Email: testEmail, Status: user.StatusDeleting}, nil).Once()
			},
			wantCode: apperr.CodeForbidden,
		},
		{
			name:   "Error: an unknown account is unauthorized",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()
			},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:   "Error: a store failure is internal",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, errors.New("mongo down")).Once()
			},
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, m := newAuthService(t)
			tc.setup(m)

			issued, err := service.RequestAccountDeletion(ctx, tc.userID)
			if tc.wantCode == "" {
				require.NoError(t, err)
				require.NotNil(t, issued)
				assert.NotEmpty(t, issued.ChallengeID)
				return
			}
			assertAppErr(t, err, tc.wantCode)
		})
	}
}

func TestService_DeleteAccount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		cmd        auth.DeleteAccountCommand
		wire       func(f *challengeFixture)
		wantCode   apperr.Code
		wantResult string
	}{
		{
			name: "Happy path: the account's deletion code marks it deleting",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, auth.PurposeDeleteAccount)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
				f.users.EXPECT().MarkDeleting(mock.Anything, validUserID, challengeTestNow).Return(nil).Once()
			},
			wantResult: auth.ResultVerified,
		},
		{
			name: "Edge: a verify_email code of the same account is refused and not consumed",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(f.validRow(validUserID, auth.PurposeVerifyEmail), nil).Once()
				f.expectAccountCount(0)
			},
			wantCode:   apperr.CodeOTPInvalid,
			wantResult: auth.ResultInvalid,
		},
		{
			name: "Edge: another account's deletion code is refused and not consumed",
			cmd:  auth.DeleteAccountCommand{UserID: freshIdentityID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(f.validRow(validUserID, auth.PurposeDeleteAccount), nil).Once()
				f.expectAccountCount(0)
			},
			wantCode:   apperr.CodeOTPInvalid,
			wantResult: auth.ResultInvalid,
		},
		{
			name: "Edge: a wrong code is refused and the account stays active",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: "000000"},
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(f.validRow(validUserID, auth.PurposeDeleteAccount), nil).Once()
				f.expectAccountCount(0)
			},
			wantCode:   apperr.CodeOTPInvalid,
			wantResult: auth.ResultInvalid,
		},
		{
			name: "Edge: a spent code is refused",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, auth.PurposeDeleteAccount)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(false, nil).Once()
			},
			wantCode:   apperr.CodeOTPInvalid,
			wantResult: auth.ResultConsumed,
		},
		{
			name: "Edge: an unknown challenge is refused as unknown",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, auth.ErrChallengeUnusable).Once()
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(nil, auth.ErrChallengeNotFound).Once()
			},
			wantCode:   apperr.CodeOTPInvalid,
			wantResult: auth.ResultUnknown,
		},
		{
			name:     "Edge: a malformed account identity is refused before the store",
			cmd:      auth.DeleteAccountCommand{UserID: "not-a-uuid", ChallengeID: validUserID, Code: challengeTestCode},
			wire:     func(f *challengeFixture) {},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name: "Error: an account removed after the code was sent is unauthorized",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, auth.PurposeDeleteAccount)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
				f.users.EXPECT().MarkDeleting(mock.Anything, validUserID, challengeTestNow).Return(user.ErrUserNotFound).Once()
			},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name: "Error: a store failure is internal",
			cmd:  auth.DeleteAccountCommand{UserID: validUserID, ChallengeID: validUserID, Code: challengeTestCode},
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, auth.PurposeDeleteAccount)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
				f.users.EXPECT().MarkDeleting(mock.Anything, validUserID, challengeTestNow).Return(errors.New("mongo down")).Once()
			},
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)
			if tc.wantResult != "" {
				f.codeObs.EXPECT().ChallengeVerified(tc.wantResult).Once()
			}

			err := f.service.DeleteAccount(ctx, tc.cmd)
			if tc.wantCode == "" {
				require.NoError(t, err)
				assert.NotContains(t, f.log.text(), challengeTestCode)
				return
			}
			assertAppErr(t, err, tc.wantCode)
		})
	}
}

func TestVerifyChallenge_RefusesDeletionCode(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
		Return(f.validRow(validUserID, auth.PurposeDeleteAccount), nil).Once()
	f.expectAccountCount(0)
	f.codeObs.EXPECT().ChallengeVerified(auth.ResultInvalid).Once()

	session, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode})

	assert.Nil(t, session)
	assertAppErr(t, err, apperr.CodeOTPInvalid)
}

func TestService_EnsureActive(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		fetch    func(m *authMocks)
		wantCode apperr.Code
	}{
		{
			name: "Happy path: an active account passes the gate",
			fetch: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
			},
		},
		{
			name: "Error: a deleting account is forbidden",
			fetch: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Status: user.StatusDeleting}, nil).Once()
			},
			wantCode: apperr.CodeForbidden,
		},
		{
			name: "Error: a dead deletion keeps the account forbidden",
			fetch: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID, Status: user.StatusDeletionDead}, nil).Once()
			},
			wantCode: apperr.CodeForbidden,
		},
		{
			name: "Error: an account that no longer exists is unauthorized",
			fetch: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()
			},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name: "Error: a store failure is internal",
			fetch: func(m *authMocks) {
				m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, errors.New("mongo down")).Once()
			},
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, m := newAuthService(t)
			tc.fetch(m)

			err := service.EnsureActive(ctx, validUserID)
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			assertAppErr(t, err, tc.wantCode)
		})
	}
}

func TestService_DeletingAccountSignsIn(t *testing.T) {
	ctx := context.Background()
	deleting := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "player", PasswordHash: testHash, Status: user.StatusDeleting}
	}

	t.Run("Error: a correct password for a deleting account is refused with the uniform credential error", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(deleting(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.observer.EXPECT().LoginFailure().Return().Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		assert.Nil(t, result)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Error: a renewal for a deleting account revokes its family and issues nothing", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(&auth.RefreshToken{
			ID:        validTokenID,
			UserID:    validUserID,
			FamilyID:  validFamily,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(deleting(), nil).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		session, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		assert.Nil(t, session)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})
}
