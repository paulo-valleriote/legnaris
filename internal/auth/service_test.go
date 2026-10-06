package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"legnaris/internal/auth"
	"legnaris/internal/auth/mocks"
	"legnaris/internal/user"
	usermocks "legnaris/internal/user/mocks"
	"legnaris/shared/apperr"
	"legnaris/shared/idgen"
	"legnaris/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	validUserID  = "01900000-0000-7000-8000-000000000001"
	validFamily  = "01900000-0000-7000-8000-000000000002"
	validTokenID = "01900000-0000-7000-8000-000000000003"
	testEmail    = "player@example.com"
	testPassword = "sup3r-secret"
	testHash     = "bcrypt-hash"
)

type authMocks struct {
	users         *usermocks.MockRepository
	refreshTokens *mocks.MockRefreshTokenRepository
	hasher        *mocks.MockPasswordHasher
	tokens        *mocks.MockTokenManager
	generator     *mocks.MockRefreshTokenGenerator
	observer      *mocks.MockObserver
}

func newAuthService(t *testing.T) (auth.Service, *authMocks) {
	t.Helper()

	m := &authMocks{
		users:         usermocks.NewMockRepository(t),
		refreshTokens: mocks.NewMockRefreshTokenRepository(t),
		hasher:        mocks.NewMockPasswordHasher(t),
		tokens:        mocks.NewMockTokenManager(t),
		generator:     mocks.NewMockRefreshTokenGenerator(t),
		observer:      mocks.NewMockObserver(t),
	}

	service := auth.NewService(
		m.users,
		m.refreshTokens,
		m.hasher,
		m.tokens,
		m.generator,
		m.observer,
		auth.Config{AccessTTL: 15 * time.Minute, RefreshTTL: 720 * time.Hour},
		logger.NewLogger(slog.LevelError),
	)
	return service, m
}

func assertAppErr(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	var appErr *apperr.Error
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

func TestService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: creates a user with a hashed password", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				return u.Email == testEmail &&
					u.Username == "player" &&
					u.PasswordHash == testHash &&
					!u.CreatedAt.IsZero() &&
					u.CreatedAt.Equal(u.UpdatedAt)
			})).
			Return(nil).
			Once()

		created, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "  Player@Example.com ",
			Username: " player ",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.NoError(t, idgen.Validate(created.ID), "server must mint a canonical UUID v7")
		assert.Equal(t, testEmail, created.Email)
		assert.Equal(t, "player", created.Username)
		assert.Equal(t, testHash, created.PasswordHash)
	})

	t.Run("Error: duplicate email maps to conflict", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(user.ErrEmailAlreadyExists).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeConflict)
	})

	t.Run("Error: invalid email is rejected before hashing", func(t *testing.T) {
		service, _ := newAuthService(t)

		_, err := service.Register(ctx, auth.RegisterCommand{Email: "not-an-email", Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInvalidRequest)
	})

	t.Run("Error: empty username is rejected", func(t *testing.T) {
		service, _ := newAuthService(t)

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "   ", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInvalidRequest)
	})

	t.Run("Error: short password is rejected", func(t *testing.T) {
		service, _ := newAuthService(t)

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: "short"})
		assertAppErr(t, err, apperr.CodeInvalidRequest)
	})

	t.Run("Error: hashing failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return("", errors.New("bcrypt exploded")).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: repository failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("mongo down")).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_Login(t *testing.T) {
	ctx := context.Background()

	storedUser := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "player", PasswordHash: testHash}
	}

	t.Run("Happy path: valid credentials issue an access and refresh token pair", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access-token", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw-refresh", nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				return token.UserID == validUserID &&
					token.TokenHash == auth.HashRefreshToken("raw-refresh") &&
					token.FamilyID != "" &&
					!token.Revoked &&
					token.ExpiresAt.After(time.Now())
			})).
			Return(nil).
			Once()

		pair, err := service.Login(ctx, auth.LoginCommand{Email: "Player@Example.com", Password: testPassword})
		require.NoError(t, err)
		assert.Equal(t, "access-token", pair.AccessToken)
		assert.Equal(t, "raw-refresh", pair.RefreshToken)
		assert.Equal(t, "Bearer", pair.TokenType)
		assert.Equal(t, 900, pair.ExpiresIn)
	})

	t.Run("Error: unknown email returns a generic 401 and records the failure", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()
		m.observer.EXPECT().LoginFailure().Return().Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: wrong password returns a generic 401 and records the failure", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, "wrong").Return(auth.ErrPasswordMismatch).Once()
		m.observer.EXPECT().LoginFailure().Return().Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: "wrong"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: repository failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, errors.New("mongo down")).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: token issuing failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{}, errors.New("sign failed")).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: refresh token persistence failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("insert failed")).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_Refresh(t *testing.T) {
	ctx := context.Background()

	activeToken := func() *auth.RefreshToken {
		return &auth.RefreshToken{
			ID:        validTokenID,
			TokenHash: auth.HashRefreshToken("current-refresh"),
			UserID:    validUserID,
			FamilyID:  validFamily,
			ExpiresAt: time.Now().Add(time.Hour),
		}
	}

	t.Run("Happy path: rotates the token, revoking the old one and keeping the family", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, auth.HashRefreshToken("current-refresh")).Return(activeToken(), nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(true, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "new-access", ExpiresIn: 900}, nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				return token.UserID == validUserID &&
					token.FamilyID == validFamily &&
					token.TokenHash == auth.HashRefreshToken("new-refresh")
			})).
			Return(nil).
			Once()

		pair, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		require.NoError(t, err)
		assert.Equal(t, "new-access", pair.AccessToken)
		assert.Equal(t, "new-refresh", pair.RefreshToken)
	})

	t.Run("Error: missing refresh token returns 401", func(t *testing.T) {
		service, _ := newAuthService(t)

		_, err := service.Refresh(ctx, auth.RefreshCommand{})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: unknown refresh token returns 401", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(nil, auth.ErrRefreshTokenNotFound).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "unknown"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: reused revoked token revokes the whole family", func(t *testing.T) {
		service, m := newAuthService(t)

		revoked := activeToken()
		revoked.Revoked = true
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(revoked, nil).Once()
		m.observer.EXPECT().RefreshReuseDetected().Return().Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: expired token returns 401", func(t *testing.T) {
		service, m := newAuthService(t)

		expired := activeToken()
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(expired, nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: concurrent rotation (claim lost) is treated as reuse", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(false, nil).Once()
		m.observer.EXPECT().RefreshReuseDetected().Return().Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: repository failure while claiming rotation maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(false, errors.New("mongo down")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: revokes the presented refresh token", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().RevokeByHash(mock.Anything, auth.HashRefreshToken("raw-refresh")).Return(true, nil).Once()

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"}))
	})

	t.Run("Edge: already revoked token is idempotent", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().RevokeByHash(mock.Anything, mock.Anything).Return(false, nil).Once()

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"}))
	})

	t.Run("Edge: missing refresh token is a no-op", func(t *testing.T) {
		service, _ := newAuthService(t)

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{}))
	})

	t.Run("Error: repository failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().RevokeByHash(mock.Anything, mock.Anything).Return(false, errors.New("mongo down")).Once()

		err := service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}
