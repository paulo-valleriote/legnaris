package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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

	testCSRFSecret = "test-secret"
	testRefreshTTL = 720 * time.Hour
)

type authMocks struct {
	users         *usermocks.MockRepository
	refreshTokens *mocks.MockRefreshTokenRepository
	hasher        *mocks.MockPasswordHasher
	tokens        *mocks.MockTokenManager
	generator     *mocks.MockRefreshTokenGenerator
	observer      *mocks.MockObserver
	csrf          auth.CSRFTokenManager
}

func newAuthService(t *testing.T) (auth.Service, *authMocks) {
	t.Helper()
	return newAuthServiceWithClock(t, nil)
}

// newAuthServiceWithClock builds the service over the shared mock set with an
// explicit time source, so timestamp assertions need no sleeping (PRQ-16).
func newAuthServiceWithClock(t *testing.T, clock auth.Clock) (auth.Service, *authMocks) {
	t.Helper()

	m := &authMocks{
		users:         usermocks.NewMockRepository(t),
		refreshTokens: mocks.NewMockRefreshTokenRepository(t),
		hasher:        mocks.NewMockPasswordHasher(t),
		tokens:        mocks.NewMockTokenManager(t),
		generator:     mocks.NewMockRefreshTokenGenerator(t),
		observer:      mocks.NewMockObserver(t),
		csrf:          auth.NewCSRFTokenManager([]byte(testCSRFSecret)),
	}

	service := auth.NewService(
		m.users,
		m.refreshTokens,
		m.hasher,
		m.tokens,
		m.generator,
		m.csrf,
		m.observer,
		auth.Config{RefreshTTL: testRefreshTTL, Now: clock},
		logger.NewLogger(slog.LevelError),
	)
	return service, m
}

// issueCSRF mints a valid, session-bound CSRF token for the given family so a
// refresh/logout test can present the value the cookie would carry.
func (m *authMocks) issueCSRF(t *testing.T, familyID string) string {
	t.Helper()
	token, err := m.csrf.Issue(familyID)
	require.NoError(t, err)
	return token
}

func assertAppErr(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	var appErr *apperr.Error
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

func expectedRefreshMaxAge() int {
	return int(testRefreshTTL.Seconds())
}

func TestService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: creates the account and signs it in", func(t *testing.T) {
		service, m := newAuthService(t)

		var createdUser *user.User
		var familyID string
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				createdUser = u
				return u.Email == testEmail &&
					u.Username == "player" &&
					u.PasswordHash == testHash &&
					!u.CreatedAt.IsZero() &&
					u.CreatedAt.Equal(u.UpdatedAt)
			})).
			Return(nil).
			Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access-token", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw-refresh", nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				familyID = token.FamilyID
				return token.TokenHash == auth.HashRefreshToken("raw-refresh") &&
					token.FamilyID != "" &&
					!token.Revoked &&
					token.ExpiresAt.After(time.Now())
			})).
			Return(nil).
			Once()

		session, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "  Player@Example.com ",
			Username: " player ",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, session)

		require.NotNil(t, createdUser)
		assert.NoError(t, idgen.Validate(createdUser.ID), "server must mint a canonical UUID v7")
		assert.Equal(t, testEmail, createdUser.Email)

		assert.Equal(t, "access-token", session.AccessToken)
		assert.Equal(t, 900, session.ExpiresIn)
		assert.Equal(t, "raw-refresh", session.RefreshToken)
		assert.Equal(t, expectedRefreshMaxAge(), session.RefreshMaxAge)
		assert.Equal(t, createdUser.ID, session.UserID)
		require.NoError(t, m.csrf.Verify(session.CSRFToken, familyID), "the CSRF token must bind to the session family")
	})

	t.Run("Error: duplicate email maps to conflict before any session is issued", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool { return u.Email == testEmail })).
			Return(user.ErrEmailAlreadyExists).
			Once()

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "Another Name <Player@Example.com>",
			Username: "player",
			Password: testPassword,
		})
		assertAppErr(t, err, apperr.CodeConflict)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgEmailAlreadyExists, appErr.Message)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
	})

	t.Run("Error: invalid email is rejected before hashing", func(t *testing.T) {
		service, m := newAuthService(t)

		_, err := service.Register(ctx, auth.RegisterCommand{Email: "not-an-email", Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInvalidRequest)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidEmail, appErr.Message)
		m.hasher.AssertNotCalled(t, "Hash", mock.Anything)
	})

	t.Run("Error: a 255-byte address is a 400 with the invalid-email message", func(t *testing.T) {
		service, m := newAuthService(t)

		overLong := strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com"
		_, err := service.Register(ctx, auth.RegisterCommand{Email: overLong, Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInvalidRequest)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidEmail, appErr.Message)
		m.hasher.AssertNotCalled(t, "Hash", mock.Anything)
	})

	t.Run("Happy path: a display-name spelling is persisted as the bare canonical address", func(t *testing.T) {
		service, m := newAuthService(t)

		var stored string
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				stored = u.Email
				return true
			})).
			Return(nil).
			Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "Name <Player@Example.COM>",
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
		assert.Equal(t, testEmail, stored)
	})

	t.Run("Edge: dotted and +tag spellings are persisted verbatim after lowercasing", func(t *testing.T) {
		service, m := newAuthService(t)

		var stored string
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				stored = u.Email
				return true
			})).
			Return(nil).
			Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "P.Layer+News@Example.com",
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
		assert.Equal(t, "p.layer+news@example.com", stored)
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

	t.Run("Edge: password at the 72-byte bcrypt limit is accepted", func(t *testing.T) {
		service, m := newAuthService(t)

		atLimit := strings.Repeat("a", 72)
		m.hasher.EXPECT().Hash(atLimit).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		session, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: atLimit})
		require.NoError(t, err)
		assert.Equal(t, "access", session.AccessToken)
	})

	t.Run("Edge: password over the 72-byte bcrypt limit is a 400, not a 500", func(t *testing.T) {
		service, m := newAuthService(t)

		overLimit := strings.Repeat("a", 73)
		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: overLimit})
		assertAppErr(t, err, apperr.CodeInvalidRequest)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgPasswordTooLong, appErr.Message)
		m.hasher.AssertNotCalled(t, "Hash", mock.Anything)
	})

	t.Run("Edge: password with non-ASCII at 8 runes but over 72 bytes is rejected", func(t *testing.T) {
		service, m := newAuthService(t)

		// 19 emoji = 19 runes = 76 bytes (each emoji is 4 bytes)
		// This tests that byte length (not rune count) is used for the 72-byte max check.
		overLimitBytes := strings.Repeat("😀", 19)
		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: overLimitBytes})
		assertAppErr(t, err, apperr.CodeInvalidRequest)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgPasswordTooLong, appErr.Message)
		m.hasher.AssertNotCalled(t, "Hash", mock.Anything)
	})

	t.Run("Error: username too long is rejected", func(t *testing.T) {
		service, _ := newAuthService(t)

		tooLong := strings.Repeat("a", 51)
		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: tooLong, Password: testPassword})
		assertAppErr(t, err, apperr.CodeInvalidRequest)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgUsernameTooLong, appErr.Message)
	})

	t.Run("Error: username with invalid characters is rejected", func(t *testing.T) {
		service, _ := newAuthService(t)

		invalidChars := []string{"user@name", "user name", "user.name", "user/name", "usérname"}
		for _, uc := range invalidChars {
			_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: uc, Password: testPassword})
			assertAppErr(t, err, apperr.CodeInvalidRequest)

			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, auth.MsgUsernameInvalid, appErr.Message)
		}
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

	t.Run("Error: session issuance failure after account creation maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{}, errors.New("sign failed")).Once()

		_, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_Login(t *testing.T) {
	ctx := context.Background()

	storedUser := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "player", PasswordHash: testHash}
	}

	t.Run("Happy path: valid credentials issue a session", func(t *testing.T) {
		service, m := newAuthService(t)

		var familyID string
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access-token", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw-refresh", nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				familyID = token.FamilyID
				return token.UserID == validUserID &&
					token.TokenHash == auth.HashRefreshToken("raw-refresh") &&
					token.FamilyID != ""
			})).
			Return(nil).
			Once()

		session, err := service.Login(ctx, auth.LoginCommand{Email: "Player@Example.com", Password: testPassword})
		require.NoError(t, err)
		assert.Equal(t, "access-token", session.AccessToken)
		assert.Equal(t, 900, session.ExpiresIn)
		assert.Equal(t, "raw-refresh", session.RefreshToken)
		assert.Equal(t, validUserID, session.UserID)
		assert.Equal(t, expectedRefreshMaxAge(), session.RefreshMaxAge)
		require.NoError(t, m.csrf.Verify(session.CSRFToken, familyID))
	})

	t.Run("Happy path: a display-name spelling reaches the canonical account", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		session, err := service.Login(ctx, auth.LoginCommand{
			Email:    "Another Name <PLAYER@Example.COM>",
			Password: testPassword,
		})
		require.NoError(t, err)
		assert.Equal(t, validUserID, session.UserID)
	})

	t.Run("Edge: dotted and +tag spellings are passed through unchanged", func(t *testing.T) {
		service, m := newAuthService(t)

		const tagged = "p.layer+news@example.com"
		m.users.EXPECT().FetchByEmail(mock.Anything, tagged).Return(storedUser(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: "P.Layer+News@Example.com", Password: testPassword})
		require.NoError(t, err)
	})

	t.Run("Edge: each login mints a distinct CSRF token", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(storedUser(), nil).Twice()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Twice()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Twice()
		m.generator.EXPECT().Generate().Return("raw", nil).Twice()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Twice()

		first, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		require.NoError(t, err)
		second, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		require.NoError(t, err)
		assert.NotEqual(t, first.CSRFToken, second.CSRFToken)
	})

	t.Run("Error: unknown email returns a generic 401 and records the failure", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()
		m.observer.EXPECT().LoginFailure().Return().Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: an unparseable address is a uniform 401 with no store lookup", func(t *testing.T) {
		service, m := newAuthService(t)

		m.observer.EXPECT().LoginFailure().Return().Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: "not-an-email", Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidCredentials, appErr.Message)
		m.users.AssertNotCalled(t, "FetchByEmail", mock.Anything, mock.Anything)
	})

	t.Run("Error: an over-long address is a uniform 401 with no store lookup", func(t *testing.T) {
		service, m := newAuthService(t)

		m.observer.EXPECT().LoginFailure().Return().Once()

		overLong := strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com"
		_, err := service.Login(ctx, auth.LoginCommand{Email: overLong, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidCredentials, appErr.Message)
		m.users.AssertNotCalled(t, "FetchByEmail", mock.Anything, mock.Anything)
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

	t.Run("Happy path: rotates the refresh and CSRF values, keeping the family", func(t *testing.T) {
		service, m := newAuthService(t)

		presentedCSRF := m.issueCSRF(t, validFamily)

		var familyID string
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, auth.HashRefreshToken("current-refresh")).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(true, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "new-access", ExpiresIn: 900}, nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				familyID = token.FamilyID
				return token.UserID == validUserID &&
					token.FamilyID == validFamily &&
					token.TokenHash == auth.HashRefreshToken("new-refresh")
			})).
			Return(nil).
			Once()

		session, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: presentedCSRF})
		require.NoError(t, err)
		assert.Equal(t, "new-access", session.AccessToken)
		assert.Equal(t, "new-refresh", session.RefreshToken)
		assert.Equal(t, validUserID, session.UserID)
		assert.NotEqual(t, presentedCSRF, session.CSRFToken, "rotation must change the CSRF token")
		assert.Equal(t, validFamily, familyID)
		require.NoError(t, m.csrf.Verify(session.CSRFToken, validFamily))
	})

	t.Run("Error: missing refresh token returns 401 before any fetch", func(t *testing.T) {
		service, m := newAuthService(t)

		_, err := service.Refresh(ctx, auth.RefreshCommand{})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		m.refreshTokens.AssertNotCalled(t, "FetchByHash", mock.Anything, mock.Anything)
	})

	t.Run("Error: unknown refresh token returns 401", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(nil, auth.ErrRefreshTokenNotFound).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "unknown"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Edge: an unknown credential is 401 even with an invalid CSRF token (presence before CSRF)", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(nil, auth.ErrRefreshTokenNotFound).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "unknown", CSRFToken: "garbage"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: a missing CSRF token is 403 and touches nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh"})
		assertAppErr(t, err, apperr.CodeForbidden)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Error: a foreign CSRF token is 403 and touched nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		foreign := m.issueCSRF(t, "01900000-0000-7000-8000-0000000000ff")
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: foreign})
		assertAppErr(t, err, apperr.CodeForbidden)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: reused revoked token revokes the whole family", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		revoked := activeToken()
		revoked.Revoked = true
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(revoked, nil).Once()
		m.observer.EXPECT().RefreshReuseDetected().Return().Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: an expired token is 401", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		expired := activeToken()
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(expired, nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
	})

	t.Run("Error: concurrent rotation (claim lost) returns 401 without revoking the family", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(false, nil).Once()
		// New behavior: do NOT call RefreshReuseDetected or RevokeFamily.
		// The successor token we created remains valid for the other client.

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		m.observer.AssertNotCalled(t, "RefreshReuseDetected")
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	})

	t.Run("Error: repository failure while claiming rotation maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(false, errors.New("mongo down")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: access issuing failure leaves the rotation unclaimed", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{}, errors.New("sign failed")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: successor persistence failure leaves the old token unclaimed", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("insert failed")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestService_Logout(t *testing.T) {
	ctx := context.Background()

	storedToken := func() *auth.RefreshToken {
		return &auth.RefreshToken{
			ID:        validTokenID,
			TokenHash: auth.HashRefreshToken("raw-refresh"),
			UserID:    validUserID,
			FamilyID:  validFamily,
			ExpiresAt: time.Now().Add(time.Hour),
		}
	}

	t.Run("Happy path: logout revokes the presented token's family", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, auth.HashRefreshToken("raw-refresh")).Return(storedToken(), nil).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh", CSRFToken: csrfToken}))
	})

	t.Run("Edge: logout with an already rotated token still revokes the family", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		rotated := storedToken()
		rotated.Revoked = true
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, auth.HashRefreshToken("raw-refresh")).Return(rotated, nil).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh", CSRFToken: csrfToken}))
	})

	t.Run("Edge: unknown token is an idempotent no-op that owes no CSRF", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(nil, auth.ErrRefreshTokenNotFound).Once()

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"}))
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	})

	t.Run("Edge: missing refresh token is a no-op", func(t *testing.T) {
		service, _ := newAuthService(t)

		require.NoError(t, service.Logout(ctx, auth.LogoutCommand{}))
	})

	t.Run("Error: a missing CSRF token on a resolved session is 403 and revokes nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(storedToken(), nil).Once()

		err := service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"})
		assertAppErr(t, err, apperr.CodeForbidden)
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	})

	t.Run("Error: a foreign CSRF token is 403 and revokes nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		foreign := m.issueCSRF(t, "01900000-0000-7000-8000-0000000000ff")
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(storedToken(), nil).Once()

		err := service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh", CSRFToken: foreign})
		assertAppErr(t, err, apperr.CodeForbidden)
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	})

	t.Run("Error: fetch failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(nil, errors.New("mongo down")).Once()

		err := service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh"})
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	})

	t.Run("Error: family revocation failure maps to internal", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(storedToken(), nil).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, mock.Anything).Return(errors.New("mongo down")).Once()

		err := service.Logout(ctx, auth.LogoutCommand{RefreshToken: "raw-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

// TestService_Clock pins the auth timestamps to the injected clock: registration
// writes exact instants and the refresh expiry boundary is crossed without
// sleeping (PRQ-16, SC-007).
func TestService_Clock(t *testing.T) {
	ctx := context.Background()

	frozen := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	now := frozen

	t.Run("Happy path: registration stamps created_at, updated_at and expires_at from the clock", func(t *testing.T) {
		service, m := newAuthServiceWithClock(t, func() time.Time { return now })

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				return u.CreatedAt.Equal(frozen) && u.UpdatedAt.Equal(frozen) && u.EmailVerifiedAt == nil
			})).
			Return(nil).
			Once()
		m.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw-refresh", nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				return token.CreatedAt.Equal(frozen) && token.ExpiresAt.Equal(frozen.Add(testRefreshTTL))
			})).
			Return(nil).
			Once()

		session, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		require.NoError(t, err)
		assert.Equal(t, "access", session.AccessToken)
	})

	t.Run("Edge: refresh is valid one nanosecond before expiry and expired at the exact instant", func(t *testing.T) {
		service, m := newAuthServiceWithClock(t, func() time.Time { return now })
		csrfToken := m.issueCSRF(t, validFamily)

		expiresAt := frozen.Add(time.Hour)
		active := func() *auth.RefreshToken {
			return &auth.RefreshToken{
				ID:        validTokenID,
				UserID:    validUserID,
				FamilyID:  validFamily,
				ExpiresAt: expiresAt,
				CreatedAt: frozen,
			}
		}

		now = expiresAt.Add(-time.Nanosecond)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(active(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: validUserID}, nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(true, nil).Once()

		session, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		require.NoError(t, err, "one nanosecond before expiry must still rotate")
		assert.Equal(t, "access", session.AccessToken)

		now = expiresAt
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(active(), nil).Once()

		_, err = service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		m.tokens.AssertNumberOfCalls(t, "Issue", 1)
	})
}

// TestService_RefreshRequiresAccount covers the account lookup that renewal now
// performs after the existing credential rules and before issuing anything: a
// missing account revokes the family and answers 401, a store failure answers
// 500, and a success names the loaded account (PRQ-05, PRQ-06, PRQ-07).
func TestService_RefreshRequiresAccount(t *testing.T) {
	ctx := context.Background()

	activeToken := func() *auth.RefreshToken {
		return &auth.RefreshToken{
			ID:        validTokenID,
			UserID:    validUserID,
			FamilyID:  validFamily,
			ExpiresAt: time.Now().Add(time.Hour),
		}
	}

	requireMessage := func(t *testing.T, err error, code apperr.Code, message string) {
		t.Helper()
		assertAppErr(t, err, code)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, message, appErr.Message)
	}

	t.Run("Error: a missing account is 401 with the family revoked and nothing issued", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		requireMessage(t, err, apperr.CodeUnauthorized, auth.MsgInvalidRefreshToken)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Edge: a revoked credential keeps its existing outcome even when the account is missing", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		revoked := activeToken()
		revoked.Revoked = true
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(revoked, nil).Once()
		m.observer.EXPECT().RefreshReuseDetected().Return().Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		requireMessage(t, err, apperr.CodeUnauthorized, auth.MsgInvalidRefreshToken)
		m.users.AssertNotCalled(t, "FetchByID", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Edge: an expired credential keeps its existing outcome even when the account is missing", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		expired := activeToken()
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(expired, nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		requireMessage(t, err, apperr.CodeUnauthorized, auth.MsgInvalidRefreshToken)
		m.users.AssertNotCalled(t, "FetchByID", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Error: an account-store failure is 500, issues nothing and leaves the family untouched", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, errors.New("mongo down")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		requireMessage(t, err, apperr.CodeInternal, auth.MsgInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
	})

	t.Run("Error: a failed family revocation on the missing-account path is 500 and issues nothing", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(nil, user.ErrUserNotFound).Once()
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(errors.New("mongo down")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		requireMessage(t, err, apperr.CodeInternal, auth.MsgInternal)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
	})

	t.Run("Happy path: issuance names the loaded account and rotates in the same family", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)

		const loadedAccountID = "01900000-0000-7000-8000-0000000000aa"

		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(&user.User{ID: loadedAccountID}, nil).Once()
		m.tokens.EXPECT().Issue(loadedAccountID).Return(auth.AccessToken{Value: "new-access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(token *auth.RefreshToken) bool {
				return token.UserID == loadedAccountID && token.FamilyID == validFamily
			})).
			Return(nil).
			Once()
		m.refreshTokens.EXPECT().RevokeByID(mock.Anything, validTokenID, mock.Anything).Return(true, nil).Once()

		session, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		require.NoError(t, err)
		assert.Equal(t, loadedAccountID, session.UserID, "the session must belong to the loaded account")
		assert.Equal(t, "new-access", session.AccessToken)
	})
}
