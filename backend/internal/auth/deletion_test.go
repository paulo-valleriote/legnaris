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

// TestService_DeleteAccount covers the deletion request: it marks the account once,
// repeats are a no-op, and an unknown or malformed identity never reaches the store.
func TestService_DeleteAccount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		userID   string
		repeat   bool
		setup    func(m *authMocks)
		wantCode apperr.Code
	}{
		{
			name:   "Happy path: an active account is marked deleting",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().MarkDeleting(mock.Anything, validUserID, mock.Anything).Return(nil).Once()
			},
		},
		{
			name:   "Edge: a repeated request for a deleting account is still accepted",
			userID: validUserID,
			repeat: true,
			setup: func(m *authMocks) {
				m.users.EXPECT().MarkDeleting(mock.Anything, validUserID, mock.Anything).Return(nil).Twice()
			},
		},
		{
			name:     "Edge: a malformed identity is refused before the store",
			userID:   "not-a-uuid",
			setup:    func(m *authMocks) {},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:   "Error: an unknown account is unauthorized",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().MarkDeleting(mock.Anything, validUserID, mock.Anything).Return(user.ErrUserNotFound).Once()
			},
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:   "Error: a store failure is internal",
			userID: validUserID,
			setup: func(m *authMocks) {
				m.users.EXPECT().MarkDeleting(mock.Anything, validUserID, mock.Anything).Return(errors.New("mongo down")).Once()
			},
			wantCode: apperr.CodeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, m := newAuthService(t)
			tc.setup(m)

			err := service.DeleteAccount(ctx, tc.userID)
			if tc.repeat {
				require.NoError(t, err)
				err = service.DeleteAccount(ctx, tc.userID)
			}
			if tc.wantCode == "" {
				require.NoError(t, err)
				return
			}
			assertAppErr(t, err, tc.wantCode)
		})
	}
}

// TestService_EnsureActive covers the gate every authenticated route runs: an active
// account passes, a deleting or dead one is forbidden, an unknown one is unauthorized.
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

// TestService_DeletingAccountSignsIn pins that a deleting account can neither sign
// in nor renew a session: a correct password is refused like a wrong one, and a
// renewal revokes the family.
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
