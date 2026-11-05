package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	"legnaris/backend/internal/user"
	usermocks "legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

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
	// testDummyHash is the digest the mocked hasher hands out for the
	// unknown-address path. The mock never parses it; it proves the service
	// compares against the hasher's own digest and not against a digest
	// pinned to a fixed cost (TFA-05).
	testDummyHash = "$2a$12$0Zk6vB0z0nM8pQ3fS6tUu1mB7xJ4kQ2wR8dH5nL0cV3yA1eT9gG"

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
	challenges    *mocks.MockChallengeRepository
	mailer        *mocks.MockCodeMailer
	notices       *mocks.MockNoticeRepository
}

// testChallengePolicy is the small, deterministic policy the gated cases run
// against: a three-send day and a ten-challenge account window make both
// bounds reachable without a wait (OTP-18).
var testChallengePolicy = auth.ChallengePolicy{
	Lifetime:         10 * time.Minute,
	Ceiling:          30 * time.Minute,
	ResendCooldown:   60 * time.Second,
	MaxAttempts:      5,
	DailySendCap:     3,
	UserWindow:       time.Hour,
	MaxPerUserWindow: 10,
}

func newAuthService(t *testing.T) (auth.Service, *authMocks) {
	t.Helper()
	return newAuthServiceWithClock(t, nil)
}

// newAuthServiceWithClock builds the service over the shared mock set with an
// explicit time source, so timestamp assertions need no sleeping (PRQ-16).
func newAuthServiceWithClock(t *testing.T, clock auth.Clock) (auth.Service, *authMocks) {
	t.Helper()
	return newAuthServiceWithChecker(t, clock, nil)
}

// newAuthServiceWithChecker is newAuthServiceWithClock with the registration
// email checker injected; a nil checker leaves the check disabled.
func newAuthServiceWithChecker(t *testing.T, clock auth.Clock, checker auth.EmailChecker) (auth.Service, *authMocks) {
	t.Helper()

	m := &authMocks{
		users:         usermocks.NewMockRepository(t),
		refreshTokens: mocks.NewMockRefreshTokenRepository(t),
		hasher:        mocks.NewMockPasswordHasher(t),
		tokens:        mocks.NewMockTokenManager(t),
		generator:     mocks.NewMockRefreshTokenGenerator(t),
		observer:      mocks.NewMockObserver(t),
		csrf:          auth.NewCSRFTokenManager([]byte(testCSRFSecret)),
		challenges:    mocks.NewMockChallengeRepository(t),
		mailer:        mocks.NewMockCodeMailer(t),
		notices:       mocks.NewMockNoticeRepository(t),
	}

	digest, err := auth.NewCodeDigest(strings.Repeat("k", 32))
	require.NoError(t, err)

	service := auth.NewService(
		m.users,
		m.refreshTokens,
		m.hasher,
		m.tokens,
		m.generator,
		m.csrf,
		m.observer,
		auth.Config{RefreshTTL: testRefreshTTL, Now: clock, EmailChecker: checker},
		logger.NewLogger(slog.LevelError),
		auth.ChallengeDeps{
			Repository: m.challenges,
			Generator:  auth.NewCodeGenerator(),
			Digest:     digest,
			Mailer:     m.mailer,
			Notices:    m.notices,
			Observer:   auth.NewNopChallengeObserver(),
			Policy:     testChallengePolicy,
		},
	)
	return service, m
}

// expectChallengeIssuance wires the writes a started code step performs: the
// durable account bound, the daily send cap, the closing of the previous
// challenge for the purpose, the insert of the new one and the send intention
// (OTP-01, OTP-12, OTP-13).
func (m *authMocks) expectChallengeIssuance(userID any, purpose auth.Purpose, email string) {
	m.challenges.EXPECT().CountForUserSince(mock.Anything, userID, mock.Anything).Return(0, nil).Once()
	m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
	m.challenges.EXPECT().CloseActive(mock.Anything, userID, purpose, mock.Anything).Return(nil).Once()
	m.challenges.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(c auth.Challenge) bool {
			return (userID == mock.Anything || c.UserID == userID) && c.Purpose == purpose
		})).
		Return(nil).
		Once()
	m.mailer.EXPECT().
		Record(mock.Anything, mock.Anything, email, purpose, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
}

// expectRegistrationChallenge wires the writes a registration's verify_email
// code step performs, for an account ID the service mints itself.
func (m *authMocks) expectRegistrationChallenge(email string) {
	m.expectChallengeIssuance(mock.Anything, auth.PurposeVerifyEmail, email)
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

// verifiedAccount is an account whose address was verified, the state a
// renewal requires (D2, O5).
func verifiedAccount(id string) *user.User {
	verifiedAt := time.Unix(1_700_000_000, 0)
	return &user.User{ID: id, EmailVerifiedAt: &verifiedAt}
}

func expectedRefreshMaxAge() int {
	return int(testRefreshTTL.Seconds())
}

func TestService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: creates the account and starts the verify_email challenge", func(t *testing.T) {
		service, m := newAuthService(t)

		var createdUser *user.User
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
		m.expectRegistrationChallenge(testEmail)

		outcome, err := service.Register(ctx, auth.RegisterCommand{
			Email:    "  Player@Example.com ",
			Username: " player ",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, outcome)

		require.NotNil(t, createdUser)
		assert.NoError(t, idgen.Validate(createdUser.ID), "server must mint a canonical UUID v7")
		assert.Equal(t, testEmail, createdUser.Email)
		assert.Nil(t, createdUser.EmailVerifiedAt, "the account is created unverified")
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
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
		m.expectRegistrationChallenge(testEmail)

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
		m.expectRegistrationChallenge("p.layer+news@example.com")

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
		m.expectRegistrationChallenge(testEmail)

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: atLimit})
		require.NoError(t, err)
		require.NotNil(t, outcome)
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

	t.Run("Error: store failure while starting the challenge maps to internal and issues no session", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.challenges.EXPECT().CloseActive(mock.Anything, mock.Anything, auth.PurposeVerifyEmail, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().Insert(mock.Anything, mock.Anything).Return(errors.New("mongo down")).Once()

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assert.Nil(t, outcome)
		assertAppErr(t, err, apperr.CodeInternal)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Happy path: spent daily allowance creates the unverified account and answers deferred with no challenge or session", func(t *testing.T) {
		service, m := newAuthService(t)

		var createdUser *user.User
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
			createdUser = u
			return u.EmailVerifiedAt == nil
		})).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(testChallengePolicy.DailySendCap, nil).Once()

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, outcome)
		assert.Equal(t, auth.DeliveryDeferred, outcome.Delivery)
		assert.Empty(t, outcome.ChallengeID, "no challenge is issued when the allowance is spent")
		assert.Zero(t, outcome.ExpiresIn)
		assert.Zero(t, outcome.ResendAfter)

		require.NotNil(t, createdUser)
		assert.Nil(t, createdUser.EmailVerifiedAt, "the account stays unverified")
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		m.challenges.AssertNotCalled(t, "CloseActive", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		m.mailer.AssertNotCalled(t, "Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Edge: the allowance spent between the check and the write also defers", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.challenges.EXPECT().CloseActive(mock.Anything, mock.Anything, auth.PurposeVerifyEmail, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
		m.mailer.EXPECT().
			Record(mock.Anything, mock.Anything, testEmail, auth.PurposeVerifyEmail, mock.Anything, mock.Anything, mock.Anything).
			Return(auth.ErrSendBudgetExhausted).
			Once()

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, outcome)
		assert.Equal(t, auth.DeliveryDeferred, outcome.Delivery)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Edge: a store failure while checking the allowance is not deferred", func(t *testing.T) {
		service, m := newAuthService(t)

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, errors.New("mongo down")).Once()

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		assert.Nil(t, outcome)
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

func TestService_Login(t *testing.T) {
	ctx := context.Background()

	storedUser := func() *user.User {
		verifiedAt := time.Now()
		return &user.User{ID: validUserID, Email: testEmail, Username: "player", PasswordHash: testHash, EmailVerifiedAt: &verifiedAt}
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

		outcome, err := service.Login(ctx, auth.LoginCommand{Email: "Player@Example.com", Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, outcome)
		session := outcome.Session
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

		outcome, err := service.Login(ctx, auth.LoginCommand{
			Email:    "Another Name <PLAYER@Example.COM>",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, outcome)
		assert.Equal(t, validUserID, outcome.Session.UserID)
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
		require.NotNil(t, first)
		require.NotNil(t, second)
		assert.NotEqual(t, first.Session.CSRFToken, second.Session.CSRFToken)
	})

	t.Run("Error: unknown email returns a generic 401 and records the failure", func(t *testing.T) {
		service, m := newAuthService(t)

		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()
		m.observer.EXPECT().LoginFailure().Return().Once()
		m.hasher.EXPECT().DummyHash().Return(testDummyHash).Once()
		m.hasher.EXPECT().Compare(testDummyHash, testPassword).Return(auth.ErrPasswordMismatch).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)

		m.hasher.AssertNumberOfCalls(t, "Compare", 1)
		m.hasher.AssertCalled(t, "Compare", testDummyHash, testPassword)
	})

	t.Run("Error: an unparseable address is a uniform 401 with no store lookup", func(t *testing.T) {
		service, m := newAuthService(t)

		m.observer.EXPECT().LoginFailure().Return().Once()
		m.hasher.EXPECT().DummyHash().Return(testDummyHash).Once()
		m.hasher.EXPECT().Compare(testDummyHash, testPassword).Return(auth.ErrPasswordMismatch).Once()

		_, err := service.Login(ctx, auth.LoginCommand{Email: "not-an-email", Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidCredentials, appErr.Message)
		m.users.AssertNotCalled(t, "FetchByEmail", mock.Anything, mock.Anything)
		m.hasher.AssertNumberOfCalls(t, "Compare", 1)
	})

	t.Run("Error: an over-long address is a uniform 401 with no store lookup", func(t *testing.T) {
		service, m := newAuthService(t)

		m.observer.EXPECT().LoginFailure().Return().Once()
		m.hasher.EXPECT().DummyHash().Return(testDummyHash).Once()
		m.hasher.EXPECT().Compare(testDummyHash, testPassword).Return(auth.ErrPasswordMismatch).Once()

		overLong := strings.Repeat("a", user.MaxEmailLength-len("@example.com")+1) + "@example.com"
		_, err := service.Login(ctx, auth.LoginCommand{Email: overLong, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)

		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidCredentials, appErr.Message)
		m.users.AssertNotCalled(t, "FetchByEmail", mock.Anything, mock.Anything)
		m.hasher.AssertNumberOfCalls(t, "Compare", 1)
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

func TestService_RegisterChallengeOutcome(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy path: registration starts a verify_email challenge bound to the new account", func(t *testing.T) {
		service, m := newAuthService(t)

		var createdUser *user.User
		createCall := m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				createdUser = u
				return u.Email == testEmail && u.EmailVerifiedAt == nil
			})).
			Return(nil).
			Once()

		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.challenges.EXPECT().CloseActive(mock.Anything, mock.Anything, auth.PurposeVerifyEmail, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().
			Insert(mock.Anything, mock.MatchedBy(func(c auth.Challenge) bool {
				return c.Purpose == auth.PurposeVerifyEmail && c.UserID == createdUser.ID
			})).
			Return(nil).
			NotBefore(createCall).
			Once()
		m.mailer.EXPECT().
			Record(mock.Anything, mock.Anything, testEmail, auth.PurposeVerifyEmail, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()

		outcome, err := service.Register(ctx, auth.RegisterCommand{
			Email:    testEmail,
			Username: "player",
			Password: testPassword,
		})
		require.NoError(t, err)
		require.NotNil(t, outcome)

		assert.NoError(t, idgen.Validate(outcome.ChallengeID))
		assert.Equal(t, int(testChallengePolicy.Lifetime.Seconds()), outcome.ExpiresIn)
		assert.Equal(t, int(testChallengePolicy.ResendCooldown.Seconds()), outcome.ResendAfter)

		require.NotNil(t, createdUser)
		assert.Nil(t, createdUser.EmailVerifiedAt, "the account is created unverified")
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Edge: concurrent registrations of one address both answer success, each superseding the other's code", func(t *testing.T) {
		service, m := newAuthService(t)

		pending := &user.User{ID: validUserID, Email: testEmail, Username: "player"}
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Times(2)
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(user.ErrEmailAlreadyExists).Once()
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pending, nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(true, nil).Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(nil).Once()
		m.expectRegistrationChallenge(testEmail)
		m.expectChallengeIssuance(validUserID, auth.PurposeVerifyEmail, testEmail)

		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				_, errs[index] = service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
			}(i)
		}
		wg.Wait()

		for _, err := range errs {
			assert.NoError(t, err)
		}
		m.challenges.AssertNumberOfCalls(t, "Insert", 2)
	})
}

// TestService_RegisterUsernameTaken covers a username another account holds: the
// answer is the username conflict whatever the address is, so it never depends on
// which addresses are registered, and it sends nothing.
func TestService_RegisterUsernameTaken(t *testing.T) {
	ctx := context.Background()
	cmd := auth.RegisterCommand{Email: "Player@Example.com", Username: "player", Password: testPassword}
	verifiedAt := time.Now()
	pending := func() *user.User { return &user.User{ID: validUserID, Email: testEmail, Username: "old"} }
	verified := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "old", EmailVerifiedAt: &verifiedAt}
	}
	other := func() *user.User { return &user.User{ID: validFamily, Username: "player"} }

	setup := func(t *testing.T, createErr error) (auth.Service, *authMocks) {
		service, m := newAuthService(t)
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(createErr).Once()
		return service, m
	}
	assertTaken := func(t *testing.T, m *authMocks, issued *auth.ChallengeIssued, err error) {
		t.Helper()
		assertAppErr(t, err, apperr.CodeConflict)
		assert.Nil(t, issued)
		assert.Contains(t, err.Error(), auth.MsgUsernameTaken)
		m.users.AssertNotCalled(t, "ReplaceUnverifiedCredentials", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "RevokeAllForUser", mock.Anything, mock.Anything)
		m.notices.AssertNotCalled(t, "Claim", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	}

	t.Run("Happy path: a free address losing the username to another account answers the username conflict", func(t *testing.T) {
		service, m := setup(t, user.ErrUsernameAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()

		issued, err := service.Register(ctx, cmd)
		assertTaken(t, m, issued, err)
	})

	t.Run("Edge: a pending account whose new username another account holds is untouched", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pending(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(other(), nil).Once()

		issued, err := service.Register(ctx, cmd)
		assertTaken(t, m, issued, err)
	})

	t.Run("Edge: a verified address with a taken username answers the same conflict, not the notice", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(verified(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(other(), nil).Once()

		issued, err := service.Register(ctx, cmd)
		assertTaken(t, m, issued, err)
	})

	t.Run("Edge: a verified address registering its own username is still a taken username", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		own := verified()
		own.Username = "player"
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(own, nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(own, nil).Once()

		issued, err := service.Register(ctx, cmd)
		assertTaken(t, m, issued, err)
	})

	t.Run("Edge: a replacement losing the username race answers the username conflict and revokes nothing", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pending(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().
			ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).
			Return(false, user.ErrUsernameAlreadyExists).
			Once()

		issued, err := service.Register(ctx, cmd)
		assertAppErr(t, err, apperr.CodeConflict)
		assert.Nil(t, issued)
		assert.Contains(t, err.Error(), auth.MsgUsernameTaken)
		m.refreshTokens.AssertNotCalled(t, "RevokeAllForUser", mock.Anything, mock.Anything)
	})

	t.Run("Error: an address index that fired with no account to read is the neutral conflict", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()

		issued, err := service.Register(ctx, cmd)
		assertAppErr(t, err, apperr.CodeConflict)
		assert.Nil(t, issued)
		assert.Contains(t, err.Error(), auth.MsgRegistrationConflict)
	})

	t.Run("Error: a failing username lookup answers internal", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pending(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, errors.New("db down")).Once()

		_, err := service.Register(ctx, cmd)
		assertAppErr(t, err, apperr.CodeInternal)
	})
}

// TestService_RegisterMinDuration proves every outcome waits for the configured
// floor, so response time does not tell the outcomes apart, and that a cancelled
// request is not held.
func TestService_RegisterMinDuration(t *testing.T) {
	const floor = 80 * time.Millisecond
	newService := func(t *testing.T) (auth.Service, *authMocks) {
		_, m := newAuthService(t)
		return auth.NewService(
			m.users, m.refreshTokens, m.hasher, m.tokens, m.generator, m.csrf, m.observer,
			auth.Config{RefreshTTL: testRefreshTTL, RegisterMinDuration: floor},
			logger.NewLogger(slog.LevelError),
			auth.ChallengeDeps{Repository: m.challenges, Mailer: m.mailer, Notices: m.notices, Policy: testChallengePolicy, Digest: nil, Generator: auth.NewCodeGenerator()},
		), m
	}
	cmd := auth.RegisterCommand{Email: "Player@Example.com", Username: "player", Password: testPassword}

	t.Run("Edge: a rejected input still waits for the floor", func(t *testing.T) {
		service, _ := newService(t)

		started := time.Now()
		_, err := service.Register(context.Background(), auth.RegisterCommand{Email: "not-an-email", Username: "player", Password: testPassword})
		require.Error(t, err)
		assert.GreaterOrEqual(t, time.Since(started), floor)
	})

	t.Run("Edge: a verified address answer waits for the floor", func(t *testing.T) {
		service, m := newService(t)
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(user.ErrEmailAlreadyExists).Once()
		account := verifiedAccount(validUserID)
		account.Email = testEmail
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(account, nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(false, nil).Once()

		started := time.Now()
		_, err := service.Register(context.Background(), cmd)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(started), floor)
	})

	t.Run("Edge: a cancelled request is not held", func(t *testing.T) {
		service, _ := newService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		started := time.Now()
		_, err := service.Register(ctx, auth.RegisterCommand{Email: "not-an-email", Username: "player", Password: testPassword})
		require.Error(t, err)
		assert.Less(t, time.Since(started), floor)
	})
}

// TestService_RegisterVerifiedAddress covers a registration against an address
// that already belongs to a verified account: it answers like a first
// registration (a 201 shape whose challenge identity no code can redeem, or the
// deferred answer when the allowance is spent), the owner is told by a code-free
// notice at most once per window, and the response never reveals the address is
// registered.
func TestService_RegisterVerifiedAddress(t *testing.T) {
	ctx := context.Background()
	register := func(service auth.Service) (*auth.ChallengeIssued, error) {
		return service.Register(ctx, auth.RegisterCommand{Email: "Player@Example.com", Username: "player", Password: testPassword})
	}
	setup := func(t *testing.T, createErr error) (auth.Service, *authMocks) {
		service, m := newAuthService(t)
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(createErr).Once()
		account := verifiedAccount(validUserID)
		account.Email = testEmail
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(account, nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		return service, m
	}

	t.Run("Happy path: the owner is notified once and the caller gets a 201-shaped answer", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(true, nil).Once()
		m.mailer.EXPECT().
			Record(mock.Anything, mock.Anything, testEmail, auth.PurposeAccountExists, "", 10, mock.Anything).
			Return(nil).
			Once()

		issued, err := register(service)
		require.NoError(t, err)
		require.NotNil(t, issued)
		assert.NoError(t, idgen.Validate(issued.ChallengeID))
		assert.Equal(t, int(testChallengePolicy.Lifetime.Seconds()), issued.ExpiresIn)
		assert.Equal(t, int(testChallengePolicy.ResendCooldown.Seconds()), issued.ResendAfter)
		assert.NotEqual(t, auth.DeliveryDeferred, issued.Delivery)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Edge: a throttled notice still answers the same and sends nothing", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(false, nil).Once()

		issued, err := register(service)
		require.NoError(t, err)
		assert.NoError(t, idgen.Validate(issued.ChallengeID))
		m.mailer.AssertNotCalled(t, "Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Edge: a spent daily allowance answers deferred like a first registration and claims nothing", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(testChallengePolicy.DailySendCap, nil).Once()

		issued, err := register(service)
		require.NoError(t, err)
		assert.Equal(t, auth.DeliveryDeferred, issued.Delivery)
		m.notices.AssertNotCalled(t, "Claim", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Edge: the cap reached while recording answers deferred", func(t *testing.T) {
		service, m := setup(t, user.ErrEmailAlreadyExists)
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(true, nil).Once()
		m.mailer.EXPECT().Record(mock.Anything, mock.Anything, testEmail, auth.PurposeAccountExists, "", 10, mock.Anything).Return(auth.ErrSendBudgetExhausted).Once()

		issued, err := register(service)
		require.NoError(t, err)
		assert.Equal(t, auth.DeliveryDeferred, issued.Delivery)
	})

	failures := []struct {
		name  string
		setup func(m *authMocks)
	}{
		{name: "Error: a failing allowance check answers internal", setup: func(m *authMocks) {
			m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, errors.New("db down")).Once()
		}},
		{name: "Error: a failing claim answers internal", setup: func(m *authMocks) {
			m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
			m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(false, errors.New("db down")).Once()
		}},
		{name: "Error: a failing outbox write answers internal", setup: func(m *authMocks) {
			m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
			m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(true, nil).Once()
			m.mailer.EXPECT().Record(mock.Anything, mock.Anything, testEmail, auth.PurposeAccountExists, "", 10, mock.Anything).Return(errors.New("db down")).Once()
		}},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			service, m := setup(t, user.ErrEmailAlreadyExists)
			tc.setup(m)

			_, err := register(service)
			assertAppErr(t, err, apperr.CodeInternal)
		})
	}
}

// TestService_RegisterReplacesUnverified covers a registration whose address
// belongs to an account that never verified it: the pending account is replaced
// in place, its sessions are revoked and a new code supersedes the old one, while
// a verified account keeps the 409 (LGN-154, D9).
func TestService_RegisterReplacesUnverified(t *testing.T) {
	ctx := context.Background()

	pendingAccount := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "old-name", PasswordHash: "old-hash"}
	}
	// duplicateCreate makes the first insert collide with the existing address.
	duplicateCreate := func(m *authMocks) {
		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().Create(mock.Anything, mock.Anything).Return(user.ErrEmailAlreadyExists).Once()
	}
	register := func(service auth.Service) (*auth.ChallengeIssued, error) {
		return service.Register(ctx, auth.RegisterCommand{Email: "Player@Example.com", Username: "player", Password: testPassword})
	}

	t.Run("Happy path: replaces username and password, revokes sessions and starts a fresh verify_email code", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().
			ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).
			Return(true, nil).
			Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(nil).Once()
		m.expectChallengeIssuance(validUserID, auth.PurposeVerifyEmail, testEmail)

		issued, err := register(service)
		require.NoError(t, err)
		require.NotNil(t, issued)
		assert.NotEmpty(t, issued.ChallengeID, "the replaced account gets a fresh challenge, same shape as a first registration")
		assert.NotEqual(t, auth.DeliveryDeferred, issued.Delivery)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})

	t.Run("Edge: re-submitting the pending account's own username is accepted", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(&user.User{ID: validUserID}, nil).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(true, nil).Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(nil).Once()
		m.expectChallengeIssuance(validUserID, auth.PurposeVerifyEmail, testEmail)

		issued, err := register(service)
		require.NoError(t, err)
		assert.NotEmpty(t, issued.ChallengeID)
	})

	t.Run("Edge: a verified account is never replaced and answers like a first registration", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(verifiedAccount(validUserID), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Once()
		m.notices.EXPECT().Claim(mock.Anything, auth.PurposeAccountExists, testEmail, mock.Anything, mock.Anything).Return(false, nil).Once()

		issued, err := register(service)
		require.NoError(t, err)
		require.NotNil(t, issued)
		m.users.AssertNotCalled(t, "ReplaceUnverifiedCredentials", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "RevokeAllForUser", mock.Anything, mock.Anything)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	})

	t.Run("Edge: an account removed between the insert and the read answers conflict", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()

		_, err := register(service)
		assertAppErr(t, err, apperr.CodeConflict)
		m.users.AssertNotCalled(t, "ReplaceUnverifiedCredentials", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: a read failure while looking up the address is internal", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, errors.New("mongo down")).Once()

		_, err := register(service)
		assertAppErr(t, err, apperr.CodeInternal)
	})

	t.Run("Error: a read failure while checking the username is internal and replaces nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, errors.New("mongo down")).Once()

		_, err := register(service)
		assertAppErr(t, err, apperr.CodeInternal)
		m.users.AssertNotCalled(t, "ReplaceUnverifiedCredentials", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: a failed credential replace is internal and revokes nothing", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(false, errors.New("mongo down")).Once()

		_, err := register(service)
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeAllForUser", mock.Anything, mock.Anything)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	})

	t.Run("Error: a failed session revocation is internal and starts no challenge", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(true, nil).Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(errors.New("mongo down")).Once()

		_, err := register(service)
		assertAppErr(t, err, apperr.CodeInternal)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	})

	t.Run("Error: a store failure while starting the new challenge is internal", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(true, nil).Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, errors.New("mongo down")).Once()

		issued, err := register(service)
		assertAppErr(t, err, apperr.CodeInternal)
		assert.Nil(t, issued)
	})

	t.Run("Edge: a spent daily allowance defers the replacement with no challenge or session", func(t *testing.T) {
		service, m := newAuthService(t)

		duplicateCreate(m)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(pendingAccount(), nil).Once()
		m.users.EXPECT().FetchByUsername(mock.Anything, "player").Return(nil, user.ErrUserNotFound).Once()
		m.users.EXPECT().ReplaceUnverifiedCredentials(mock.Anything, validUserID, "player", testHash, mock.Anything).Return(true, nil).Once()
		m.refreshTokens.EXPECT().RevokeAllForUser(mock.Anything, validUserID).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(testChallengePolicy.DailySendCap, nil).Once()

		issued, err := register(service)
		require.NoError(t, err)
		assert.Equal(t, auth.DeliveryDeferred, issued.Delivery)
		assert.Empty(t, issued.ChallengeID)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{}, errors.New("sign failed")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: successor persistence failure leaves the old token unclaimed", func(t *testing.T) {
		service, m := newAuthService(t)

		csrfToken := m.issueCSRF(t, validFamily)
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(activeToken(), nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("new-refresh", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("insert failed")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})
}

// TestService_RefreshUnverifiedAccount pins that renewal no longer consults
// the verification state: an account without a verification timestamp rotates
// in the same family like any other.
func TestService_RefreshUnverifiedAccount(t *testing.T) {
	ctx := context.Background()

	unverifiedFetch := func(m *authMocks) {
		m.refreshTokens.EXPECT().FetchByHash(mock.Anything, mock.Anything).Return(&auth.RefreshToken{
			ID:        validTokenID,
			UserID:    validUserID,
			FamilyID:  validFamily,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil).Once()
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).
			Return(&user.User{ID: validUserID, Email: testEmail, Username: "player"}, nil).Once()
	}

	t.Run("Error: an unverified account is refused 401 and its family is revoked", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)
		unverifiedFetch(m)
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		session, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		assert.Nil(t, session)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		m.refreshTokens.AssertNotCalled(t, "RevokeByID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: the refusal of an unverified account is the uniform refresh message", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)
		unverifiedFetch(m)
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(nil).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, auth.MsgInvalidRefreshToken, appErr.Message)
	})

	t.Run("Error: a family revocation failure for an unverified account is internal and issues nothing", func(t *testing.T) {
		service, m := newAuthService(t)
		csrfToken := m.issueCSRF(t, validFamily)
		unverifiedFetch(m)
		m.refreshTokens.EXPECT().RevokeFamily(mock.Anything, validFamily).Return(errors.New("mongo down")).Once()

		_, err := service.Refresh(ctx, auth.RefreshCommand{RefreshToken: "current-refresh", CSRFToken: csrfToken})
		assertAppErr(t, err, apperr.CodeInternal)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
	})
}

// TestService_LoginUnverifiedAccount covers the sign-in of an account whose
// address was never verified: the password is proven, then the verify_email
// step starts instead of a session, bounded by the same daily and per-account
// limits as registration (D2, TFA-06, OTP-13, OTP-14).
func TestService_LoginUnverifiedAccount(t *testing.T) {
	ctx := context.Background()

	unverified := func() *user.User {
		return &user.User{ID: validUserID, Email: testEmail, Username: "player", PasswordHash: testHash}
	}

	t.Run("Happy path: correct password starts a verify_email challenge and issues no session", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.expectChallengeIssuance(validUserID, auth.PurposeVerifyEmail, testEmail)

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Session, "an unverified sign-in never carries a session")
		require.NotNil(t, result.Challenge)
		assert.NoError(t, idgen.Validate(result.Challenge.ChallengeID))
		assert.Equal(t, int(testChallengePolicy.Lifetime.Seconds()), result.Challenge.ExpiresIn)
		assert.Equal(t, int(testChallengePolicy.ResendCooldown.Seconds()), result.Challenge.ResendAfter)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.generator.AssertNotCalled(t, "Generate")
		m.refreshTokens.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("Happy path: a verified account keeps the session and starts no challenge", func(t *testing.T) {
		service, m := newAuthService(t)
		verifiedAt := time.Now()
		stored := unverified()
		stored.EmailVerifiedAt = &verifiedAt
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(stored, nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.tokens.EXPECT().Issue(validUserID).Return(auth.AccessToken{Value: "access", ExpiresIn: 900}, nil).Once()
		m.generator.EXPECT().Generate().Return("raw", nil).Once()
		m.refreshTokens.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, result.Session)
		assert.Nil(t, result.Challenge)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
		m.mailer.AssertNotCalled(t, "Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: wrong password is the uniform 401 and starts no challenge", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, "wrong").Return(errors.New("mismatch")).Once()
		m.observer.EXPECT().LoginFailure().Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: "wrong"})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		assert.Nil(t, result)
		m.challenges.AssertNotCalled(t, "CountForUserSince", mock.Anything, mock.Anything, mock.Anything)
		m.mailer.AssertNotCalled(t, "SentSince", mock.Anything, mock.Anything)
	})

	t.Run("Error: unknown email is the uniform 401 and starts no challenge", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(nil, user.ErrUserNotFound).Once()
		m.hasher.EXPECT().DummyHash().Return(testDummyHash).Once()
		m.hasher.EXPECT().Compare(testDummyHash, testPassword).Return(errors.New("mismatch")).Once()
		m.observer.EXPECT().LoginFailure().Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeUnauthorized)
		assert.Nil(t, result)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	})

	t.Run("Error: spent daily allowance answers 503 EMAIL_UNAVAILABLE with a retry hint and no session", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, validUserID, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(testChallengePolicy.DailySendCap, nil).Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeEmailUnavailable)
		assert.Nil(t, result)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Greater(t, appErr.RetryAfter, time.Duration(0))
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
		m.challenges.AssertNotCalled(t, "Insert", mock.Anything, mock.Anything)
	})

	t.Run("Edge: one send below the cap still starts the challenge", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, validUserID, mock.Anything).Return(0, nil).Once()
		m.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(testChallengePolicy.DailySendCap-1, nil).Once()
		m.challenges.EXPECT().CloseActive(mock.Anything, validUserID, auth.PurposeVerifyEmail, mock.Anything).Return(nil).Once()
		m.challenges.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
		m.mailer.EXPECT().Record(mock.Anything, mock.Anything, testEmail, auth.PurposeVerifyEmail, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, result.Challenge)
	})

	t.Run("Error: the per-account bound answers 429 RATE_LIMITED and sends nothing", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, validUserID, mock.Anything).
			Return(testChallengePolicy.MaxPerUserWindow, nil).Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeRateLimited)
		assert.Nil(t, result)
		m.mailer.AssertNotCalled(t, "Record", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Error: a store failure while starting the challenge is internal and issues no session", func(t *testing.T) {
		service, m := newAuthService(t)
		m.users.EXPECT().FetchByEmail(mock.Anything, testEmail).Return(unverified(), nil).Once()
		m.hasher.EXPECT().Compare(testHash, testPassword).Return(nil).Once()
		m.challenges.EXPECT().CountForUserSince(mock.Anything, validUserID, mock.Anything).Return(0, errors.New("mongo down")).Once()

		result, err := service.Login(ctx, auth.LoginCommand{Email: testEmail, Password: testPassword})
		assertAppErr(t, err, apperr.CodeInternal)
		assert.Nil(t, result)
		m.tokens.AssertNotCalled(t, "Issue", mock.Anything)
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

	t.Run("Happy path: registration stamps created_at and updated_at from the clock", func(t *testing.T) {
		service, m := newAuthServiceWithClock(t, func() time.Time { return now })

		m.hasher.EXPECT().Hash(testPassword).Return(testHash, nil).Once()
		m.users.EXPECT().
			Create(mock.Anything, mock.MatchedBy(func(u *user.User) bool {
				return u.CreatedAt.Equal(frozen) && u.UpdatedAt.Equal(frozen) && u.EmailVerifiedAt == nil
			})).
			Return(nil).
			Once()
		m.expectRegistrationChallenge(testEmail)

		outcome, err := service.Register(ctx, auth.RegisterCommand{Email: testEmail, Username: "player", Password: testPassword})
		require.NoError(t, err)
		require.NotNil(t, outcome)
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(validUserID), nil).Once()
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
		m.users.EXPECT().FetchByID(mock.Anything, validUserID).Return(verifiedAccount(loadedAccountID), nil).Once()
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
