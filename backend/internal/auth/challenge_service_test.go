package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/auth/mocks"
	usermocks "legnaris/backend/internal/user/mocks"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// challengeTestNow is the frozen instant every challenge boundary is driven
// from, so expiry, cool-down and the ceiling are crossed with no sleep
// (OTP-20, SC-007).
// otherPurpose is a second, distinct purpose for cases that need one besides
// verify_email.
const otherPurpose auth.Purpose = "other_purpose"

var challengeTestNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

const (
	challengeTestCode = "042917"
	freshIdentityID   = "01900000-0000-7000-8000-000000000009"
)

// challengeTestPolicy is a small, deterministic policy: a three-send day and a
// ten-challenge account window make both bounds reachable in a unit test
// (data-model.md §3).
var challengeTestPolicy = auth.ChallengePolicy{
	Lifetime:         10 * time.Minute,
	Ceiling:          30 * time.Minute,
	ResendCooldown:   60 * time.Second,
	MaxAttempts:      5,
	DailySendCap:     3,
	ResendCapRatio:   0.5,
	UserWindow:       time.Hour,
	MaxPerUserWindow: 10,
}

// fixedCodeGenerator returns one deterministic code, so assertions can compute
// the expected digest (OTP-02).
type fixedCodeGenerator struct{ code string }

func (g fixedCodeGenerator) Generate() (string, error) { return g.code, nil }

// failingCodeGenerator models an entropy-source failure (OTP-17).
type failingCodeGenerator struct{ err error }

func (g failingCodeGenerator) Generate() (string, error) { return "", g.err }

// captureLog records the service's structured lines so a suite can assert the
// code never reaches one (OTP-15).
type captureLog struct{ lines []string }

func (l *captureLog) append(msg string, args []logger.Attribute) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, msg)
	for _, attribute := range args {
		parts = append(parts, attribute.Key+"="+attribute.Value)
	}
	l.lines = append(l.lines, strings.Join(parts, " "))
}

func (l *captureLog) Info(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLog) Debug(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLog) Warn(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLog) Error(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLog) Fatal(_ context.Context, msg string, args ...logger.Attribute) {
	l.append(msg, args)
}

func (l *captureLog) With(string, string) logger.StructuredLogger { return l }

func (l *captureLog) text() string { return strings.Join(l.lines, "\n") }

type challengeFixture struct {
	service   auth.Service
	users     *usermocks.MockRepository
	refresh   *mocks.MockRefreshTokenRepository
	hasher    *mocks.MockPasswordHasher
	tokens    *mocks.MockTokenManager
	gen       *mocks.MockRefreshTokenGenerator
	observer  *mocks.MockObserver
	repo      *mocks.MockChallengeRepository
	mailer    *mocks.MockCodeMailer
	codeObs   *mocks.MockChallengeObserver
	digest    *auth.CodeDigest
	generator auth.CodeGenerator
	log       *captureLog
	now       time.Time
}

// newChallengeFixture builds the service over the mockery mocks with a frozen
// clock and a deterministic code generator.
func newChallengeFixture(t *testing.T) *challengeFixture {
	t.Helper()
	return newChallengeFixtureWith(t, fixedCodeGenerator{code: challengeTestCode}, challengeTestPolicy)
}

func newChallengeFixtureWithGenerator(t *testing.T, generator auth.CodeGenerator) *challengeFixture {
	t.Helper()
	return newChallengeFixtureWith(t, generator, challengeTestPolicy)
}

func newChallengeFixtureWith(t *testing.T, generator auth.CodeGenerator, policy auth.ChallengePolicy) *challengeFixture {
	t.Helper()

	digest, err := auth.NewCodeDigest(strings.Repeat("k", 32))
	require.NoError(t, err)

	f := &challengeFixture{
		users:     usermocks.NewMockRepository(t),
		refresh:   mocks.NewMockRefreshTokenRepository(t),
		hasher:    mocks.NewMockPasswordHasher(t),
		tokens:    mocks.NewMockTokenManager(t),
		gen:       mocks.NewMockRefreshTokenGenerator(t),
		observer:  mocks.NewMockObserver(t),
		repo:      mocks.NewMockChallengeRepository(t),
		mailer:    mocks.NewMockCodeMailer(t),
		codeObs:   mocks.NewMockChallengeObserver(t),
		digest:    digest,
		generator: generator,
		log:       &captureLog{},
		now:       challengeTestNow,
	}

	f.service = auth.NewService(
		f.users,
		f.refresh,
		f.hasher,
		f.tokens,
		f.gen,
		auth.NewCSRFTokenManager([]byte(testCSRFSecret)),
		f.observer,
		auth.Config{RefreshTTL: testRefreshTTL, Now: f.clock},
		f.log,
		auth.ChallengeDeps{
			Repository: f.repo,
			Generator:  f.generator,
			Digest:     f.digest,
			Mailer:     f.mailer,
			Observer:   f.codeObs,
			Policy:     policy,
		},
	)
	return f
}

func (f *challengeFixture) clock() time.Time { return f.now }

// digestFor computes the digest the service must store for a code.
func (f *challengeFixture) digestFor(userID string, purpose auth.Purpose, code string) string {
	return f.digest.Digest(userID, purpose, code)
}

// validRow builds a usable stored challenge for the account and purpose.
func (f *challengeFixture) validRow(userID string, purpose auth.Purpose) *auth.Challenge {
	return &auth.Challenge{
		ID:           validUserID,
		UserID:       userID,
		Purpose:      purpose,
		CodeHash:     f.digestFor(userID, purpose, challengeTestCode),
		ExpiresAt:    f.now.Add(10 * time.Minute),
		CeilingAt:    f.now.Add(30 * time.Minute),
		Attempts:     1,
		CreatedAt:    f.now,
		LastSentAt:   f.now,
		LastIntentID: validFamily,
	}
}

// expectAccountCount wires the durable per-account bound's window count
// (OTP-14).
func (f *challengeFixture) expectAccountCount(count int) {
	f.repo.EXPECT().
		CountForUserSince(mock.Anything, validUserID, f.now.Add(-challengeTestPolicy.UserWindow)).
		Return(count, nil).
		Once()
}

// expectSentCount wires the daily send cap's count for the current day
// (OTP-13). The precise window origin is pinned by its own test.
func (f *challengeFixture) expectSentCount(count int) {
	f.mailer.EXPECT().
		SentSince(mock.Anything, mock.Anything).
		Return(count, nil).
		Once()
}

// expectResendCount wires the resend share's count for the current day (D5).
func (f *challengeFixture) expectResendCount(count int) {
	f.mailer.EXPECT().
		ResendsSince(mock.Anything, mock.Anything).
		Return(count, nil).
		Once()
}

// expectSignIn wires the session-issuance expectations the verification tail
// shares with the existing sign-in path.
func (f *challengeFixture) expectSignIn() {
	f.tokens.EXPECT().Issue(mock.Anything).Return(auth.AccessToken{Value: "access-token", ExpiresIn: 900}, nil).Once()
	f.gen.EXPECT().Generate().Return("raw-refresh", nil).Once()
	f.refresh.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
}

// TestIssueChallenge_PersistsBeforeSending pins the issuance order and shape:
// the challenge is inserted before the send intention is recorded, the identity
// is canonical, and the digest — never the code — is stored (OTP-01, OTP-02,
// OTP-12).
func TestIssueChallenge_PersistsBeforeSending(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	var inserted auth.Challenge
	insertedFirst := false
	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.repo.EXPECT().CloseActive(mock.Anything, validUserID, auth.PurposeVerifyEmail, challengeTestNow).Return(nil).Once()
	f.repo.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(challenge auth.Challenge) bool {
			inserted = challenge
			insertedFirst = true
			return true
		})).
		Return(nil).
		Once()
	f.mailer.EXPECT().
		Record(mock.Anything, mock.Anything, testEmail, auth.PurposeVerifyEmail, challengeTestCode, 10, challengeTestNow).
		Run(func(context.Context, string, string, auth.Purpose, string, int, time.Time) {
			require.True(t, insertedFirst, "the challenge must exist before its intention is recorded")
		}).
		Return(nil).
		Once()
	f.codeObs.EXPECT().ChallengeIssued(auth.PurposeVerifyEmail).Once()

	issued, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{
		UserID:  validUserID,
		Email:   testEmail,
		Purpose: auth.PurposeVerifyEmail,
	})
	require.NoError(t, err)

	t.Run("Happy path: the response carries the identity and the policy windows", func(t *testing.T) {
		assert.Equal(t, inserted.ID, issued.ChallengeID)
		assert.Equal(t, 600, issued.ExpiresIn)
		assert.Equal(t, 60, issued.ResendAfter)
	})

	t.Run("Happy path: the stored record is canonical, bounded and digest-only", func(t *testing.T) {
		require.NoError(t, idgen.Validate(inserted.ID))
		require.NoError(t, idgen.Validate(inserted.LastIntentID))
		assert.Equal(t, validUserID, inserted.UserID)
		assert.Equal(t, challengeTestNow.Add(10*time.Minute), inserted.ExpiresAt)
		assert.Equal(t, challengeTestNow.Add(30*time.Minute), inserted.CeilingAt)
		assert.Equal(t, challengeTestNow, inserted.CreatedAt)
		assert.Equal(t, challengeTestNow, inserted.LastSentAt)
		assert.Zero(t, inserted.Attempts)
		assert.Nil(t, inserted.ConsumedAt)
		assert.Equal(t, f.digestFor(validUserID, auth.PurposeVerifyEmail, challengeTestCode), inserted.CodeHash)
		assert.NotEqual(t, challengeTestCode, inserted.CodeHash, "the code must never be stored in the clear")
	})

	t.Run("Edge: no log line contains the code and the address is masked", func(t *testing.T) {
		assert.NotContains(t, f.log.text(), challengeTestCode)
		assert.NotContains(t, f.log.text(), testEmail, "the log must mask the recipient")
	})
}

// TestIssueChallenge_SendWindow pins the exact day origin the cap counts from
// (OTP-13).
func TestIssueChallenge_SendWindow(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	f.expectAccountCount(0)
	f.mailer.EXPECT().
		SentSince(mock.Anything, challengeTestPolicy.DayStart(challengeTestNow)).
		Return(challengeTestPolicy.DailySendCap, nil).
		Once()
	f.codeObs.EXPECT().SendCapBlocked().Once()

	_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

	assertAppErr(t, err, apperr.CodeEmailUnavailable)
}

// TestIssueChallenge_CapReached pins the daily send cap: no challenge is
// written, no intention is recorded, and the answer is the typed unavailable
// outcome with a retry hint (OTP-13, SC-004).
func TestIssueChallenge_CapReached(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	f.expectAccountCount(0)
	f.expectSentCount(challengeTestPolicy.DailySendCap)
	f.codeObs.EXPECT().SendCapBlocked().Once()

	issued, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

	assert.Nil(t, issued)
	assertAppErr(t, err, apperr.CodeEmailUnavailable)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 503, appErr.HttpStatus())
	assert.Equal(t, challengeTestPolicy.DayStart(challengeTestNow).Add(24*time.Hour).Sub(challengeTestNow), appErr.RetryAfter,
		"the hint points at the next day's window")
	assert.NotContains(t, err.Error(), testEmail, "the refusal reveals no account state")
}

// TestIssueChallenge_CapRace pins the cap being reached between the check and
// the intention write: the record path's sentinel still answers unavailable
// (OTP-13).
func TestIssueChallenge_CapRace(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.repo.EXPECT().CloseActive(mock.Anything, mock.Anything, mock.Anything, challengeTestNow).Return(nil).Once()
	f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
	f.mailer.EXPECT().
		Record(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(auth.ErrSendBudgetExhausted).
		Once()
	f.codeObs.EXPECT().SendCapBlocked().Once()

	_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

	assertAppErr(t, err, apperr.CodeEmailUnavailable)
}

// TestIssueChallenge_StoreFailures pins that every store failure is an internal
// failure and never a code failure (OTP-17).
func TestIssueChallenge_StoreFailures(t *testing.T) {
	ctx := context.Background()
	storeErr := errors.New("store down")

	tests := []struct {
		name string
		wire func(f *challengeFixture)
	}{
		{
			name: "Error: the send-count read fails",
			wire: func(f *challengeFixture) {
				f.expectAccountCount(0)
				f.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, storeErr).Once()
			},
		},
		{
			name: "Error: closing the previous challenge fails",
			wire: func(f *challengeFixture) {
				f.expectAccountCount(0)
				f.expectSentCount(0)
				f.repo.EXPECT().CloseActive(mock.Anything, mock.Anything, mock.Anything, challengeTestNow).Return(storeErr).Once()
			},
		},
		{
			name: "Error: inserting the challenge fails",
			wire: func(f *challengeFixture) {
				f.expectAccountCount(0)
				f.expectSentCount(0)
				f.repo.EXPECT().CloseActive(mock.Anything, mock.Anything, mock.Anything, challengeTestNow).Return(nil).Once()
				f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(storeErr).Once()
			},
		},
		{
			name: "Error: recording the intention fails",
			wire: func(f *challengeFixture) {
				f.expectAccountCount(0)
				f.expectSentCount(0)
				f.repo.EXPECT().CloseActive(mock.Anything, mock.Anything, mock.Anything, challengeTestNow).Return(nil).Once()
				f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
				f.mailer.EXPECT().
					Record(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(storeErr).
					Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)

			_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

			assertAppErr(t, err, apperr.CodeInternal)
		})
	}

	t.Run("Error: the code generator's entropy source fails", func(t *testing.T) {
		f := newChallengeFixtureWithGenerator(t, failingCodeGenerator{err: storeErr})
		f.expectAccountCount(0)
		f.expectSentCount(0)

		_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

		assertAppErr(t, err, apperr.CodeInternal)
	})
}

// TestVerifyChallenge_HappyPath pins the successful consumption: the attempt is
// claimed, the digest is compared, the row is consumed, the account's
// verification timestamp is recorded and a session is issued (OTP-04, OTP-07).
func TestVerifyChallenge_HappyPath(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)
	row := f.validRow(validUserID, otherPurpose)

	f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
	f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(nil).Once()
	f.expectSignIn()
	f.codeObs.EXPECT().ChallengeVerified(auth.ResultVerified).Once()

	session, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode})
	require.NoError(t, err)

	t.Run("Happy path: a session exactly as sign-in issues", func(t *testing.T) {
		require.NotNil(t, session)
		assert.Equal(t, "access-token", session.AccessToken)
		assert.Equal(t, "raw-refresh", session.RefreshToken)
		assert.Equal(t, validUserID, session.UserID)
	})

	t.Run("Edge: no log line contains the code", func(t *testing.T) {
		assert.NotContains(t, f.log.text(), challengeTestCode)
	})
}

// TestVerifyChallenge_UniformFailures drives every failure kind and asserts one
// identical typed outcome, with only the observer's classification differing
// (OTP-04, OTP-05, OTP-06, OTP-08, SC-002).
func TestVerifyChallenge_UniformFailures(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		challenge  string
		code       string
		wire       func(f *challengeFixture)
		wantResult string
	}{
		{
			name:       "Edge: a wrong code is counted and refused",
			challenge:  validUserID,
			code:       "000000",
			wantResult: auth.ResultInvalid,
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(f.validRow(validUserID, otherPurpose), nil).Once()
				f.expectAccountCount(0)
			},
		},
		{
			name:       "Edge: an exhausted budget is refused before comparison",
			challenge:  validUserID,
			code:       challengeTestCode,
			wantResult: auth.ResultExhausted,
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, auth.ErrChallengeUnusable).Once()
				exhausted := f.validRow(validUserID, otherPurpose)
				exhausted.Attempts = challengeTestPolicy.MaxAttempts
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(exhausted, nil).Once()
			},
		},
		{
			name:       "Edge: an expired challenge is refused",
			challenge:  validUserID,
			code:       challengeTestCode,
			wantResult: auth.ResultExpired,
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, auth.ErrChallengeUnusable).Once()
				expired := f.validRow(validUserID, otherPurpose)
				expired.ExpiresAt = challengeTestNow
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(expired, nil).Once()
			},
		},
		{
			name:       "Edge: an unknown identity is refused as unknown",
			challenge:  validUserID,
			code:       challengeTestCode,
			wantResult: auth.ResultUnknown,
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, auth.ErrChallengeUnusable).Once()
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(nil, auth.ErrChallengeNotFound).Once()
			},
		},
		{
			name:       "Edge: a code losing the race is refused as consumed",
			challenge:  validUserID,
			code:       challengeTestCode,
			wantResult: auth.ResultConsumed,
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(false, nil).Once()
			},
		},
		{
			name:       "Edge: a malformed identity is refused without touching the store",
			challenge:  "not-a-uuid",
			code:       challengeTestCode,
			wantResult: auth.ResultUnknown,
			wire:       func(*challengeFixture) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)
			f.codeObs.EXPECT().ChallengeVerified(tc.wantResult).Once()

			session, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: tc.challenge, Code: tc.code})

			assert.Nil(t, session, "no failure may issue a session")
			assertAppErr(t, err, apperr.CodeOTPInvalid)
			assert.NotContains(t, err.Error(), challengeTestCode)
		})
	}
}

// TestVerifyChallenge_StoreFailures pins that a store failure is never reported
// as an invalid code (OTP-17).
func TestVerifyChallenge_StoreFailures(t *testing.T) {
	ctx := context.Background()
	storeErr := errors.New("store down")

	tests := []struct {
		name string
		wire func(f *challengeFixture)
	}{
		{
			name: "Error: the attempt claim fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, storeErr).Once()
			},
		},
		{
			name: "Error: the diagnostic read fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(nil, auth.ErrChallengeUnusable).Once()
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(nil, storeErr).Once()
			},
		},
		{
			name: "Error: the consume claim fails",
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(false, storeErr).Once()
			},
		},
		{
			name: "Error: recording the verification timestamp fails",
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).
					Return(row, nil).Once()
				f.expectAccountCount(0)
				f.repo.EXPECT().Consume(mock.Anything, validUserID, row.CodeHash, challengeTestNow).Return(true, nil).Once()
				f.users.EXPECT().MarkEmailVerified(mock.Anything, validUserID, challengeTestNow).Return(storeErr).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)

			session, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode})

			assert.Nil(t, session)
			assertAppErr(t, err, apperr.CodeInternal)
		})
	}
}

// TestVerifyChallenge_BindsAccountAndPurpose proves a digest issued for another
// account or purpose cannot be verified here (OTP-03).
func TestVerifyChallenge_BindsAccountAndPurpose(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	// The stored digest binds verify_email, so the same code presented against
	// a row of that purpose but a digest computed for another account fails.
	row := f.validRow(validUserID, auth.PurposeVerifyEmail)
	row.CodeHash = f.digestFor(freshIdentityID, auth.PurposeVerifyEmail, challengeTestCode)
	f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.codeObs.EXPECT().ChallengeVerified(auth.ResultInvalid).Once()

	_, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode})

	assertAppErr(t, err, apperr.CodeOTPInvalid)
}

// TestResendChallenge_RecordsSameCode pins the resend: the code is not rotated,
// the identity is unchanged, the deadline moves and a new intention carries the
// same payload (OTP-10, SC-005).
func TestResendChallenge_RecordsSameCode(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	row.LastSentAt = challengeTestNow.Add(-2 * time.Minute)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.expectResendCount(0)
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, challengeTestNow.Add(10*time.Minute)).
		Return(true, nil).
		Once()
	f.mailer.EXPECT().Requeue(mock.Anything, validFamily, challengeTestNow).Return(freshIdentityID, nil).Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSent).Once()

	result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})
	require.NoError(t, err)

	t.Run("Happy path: the answer carries the retry window", func(t *testing.T) {
		require.NotNil(t, result)
		assert.Equal(t, 60, result.ResendAfter)
	})

	t.Run("Edge: the code and the challenge identity are unchanged", func(t *testing.T) {
		// A resend re-records the stored intention (same payload, fresh row)
		// and never consumes or rotates the challenge.
		assert.Contains(t, f.log.text(), "challenge resent")
		assert.NotContains(t, f.log.text(), challengeTestCode)
	})
}

// TestResendChallenge_ClampsToCeiling pins the deadline never passing the
// challenge's stored absolute ceiling, no matter how late the resend is: an old
// challenge whose ceiling falls inside the configured lifetime clamps to it
// (OTP-10).
func TestResendChallenge_ClampsToCeiling(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	// The challenge was created 25 minutes ago, so its absolute ceiling
	// (created_at + 30m) is only five minutes away — inside the ten-minute
	// lifetime, which forces the clamp.
	row.CreatedAt = challengeTestNow.Add(-25 * time.Minute)
	row.CeilingAt = row.CreatedAt.Add(challengeTestPolicy.Ceiling)
	row.ExpiresAt = row.CeilingAt
	row.LastSentAt = challengeTestNow.Add(-time.Hour)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.expectResendCount(0)
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, row.CeilingAt).
		Return(true, nil).
		Once()
	f.mailer.EXPECT().Requeue(mock.Anything, validFamily, challengeTestNow).Return(freshIdentityID, nil).Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSent).Once()

	_, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})
	require.NoError(t, err)
}

// TestResendChallenge_InsideCooldown pins the cool-down suppression: the client
// hears the same accepted answer and no intention is recorded (OTP-10, SC-005).
func TestResendChallenge_InsideCooldown(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	row.LastSentAt = challengeTestNow.Add(-time.Second)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.expectResendCount(0)
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, mock.Anything).
		Return(false, nil).
		Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSuppressed).Once()

	result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

	require.NoError(t, err)
	assert.Equal(t, 60, result.ResendAfter, "the retry window is still reported")
}

// TestResendChallenge_UniformSuppression pins that an unknown, malformed,
// consumed, expired or exhausted identity answers exactly like a suppressed
// resend and records nothing (OTP-11).
func TestResendChallenge_UniformSuppression(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		challenge string
		wire      func(f *challengeFixture)
	}{
		{
			name:      "Edge: a malformed identity never reaches the store",
			challenge: "not-a-uuid",
			wire:      func(*challengeFixture) {},
		},
		{
			name:      "Edge: an unknown identity records nothing",
			challenge: validUserID,
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(nil, auth.ErrChallengeNotFound).Once()
			},
		},
		{
			name:      "Edge: a consumed identity records nothing",
			challenge: validUserID,
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				consumedAt := challengeTestNow
				row.ConsumedAt = &consumedAt
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
			},
		},
		{
			name:      "Edge: an expired identity records nothing",
			challenge: validUserID,
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				row.ExpiresAt = challengeTestNow
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
			},
		},
		{
			name:      "Edge: an exhausted identity records nothing",
			challenge: validUserID,
			wire: func(f *challengeFixture) {
				row := f.validRow(validUserID, otherPurpose)
				row.Attempts = challengeTestPolicy.MaxAttempts
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)
			f.codeObs.EXPECT().ResendRequested(auth.ResendSuppressed).Once()

			result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: tc.challenge})

			require.NoError(t, err, "every suppression answers the accepted outcome")
			assert.Equal(t, 60, result.ResendAfter)
		})
	}
}

// TestResendChallenge_CapReached pins the unavailable outcome and that the
// deadline never moves once the cap refuses (OTP-13).
func TestResendChallenge_CapReached(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Once()
	f.expectAccountCount(0)
	f.expectSentCount(challengeTestPolicy.DailySendCap)
	f.codeObs.EXPECT().SendCapBlocked().Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendCapped).Once()

	result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

	assert.Nil(t, result)
	assertAppErr(t, err, apperr.CodeEmailUnavailable)
}

// TestResendChallenge_ConcurrentPairSuppressesOne pins that two racing resends
// record exactly one send: the loser's Extend reports false (OTP-10).
func TestResendChallenge_ConcurrentPairSuppressesOne(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)

	row := f.validRow(validUserID, otherPurpose)
	f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(row, nil).Twice()
	f.repo.EXPECT().
		CountForUserSince(mock.Anything, validUserID, challengeTestNow.Add(-challengeTestPolicy.UserWindow)).
		Return(0, nil).
		Twice()
	f.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, nil).Twice()
	f.mailer.EXPECT().ResendsSince(mock.Anything, mock.Anything).Return(0, nil).Twice()
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, mock.Anything).
		Return(true, nil).
		Once()
	f.repo.EXPECT().
		Extend(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.ResendCooldown, mock.Anything).
		Return(false, nil).
		Once()
	f.mailer.EXPECT().Requeue(mock.Anything, validFamily, challengeTestNow).Return(freshIdentityID, nil).Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSent).Once()
	f.codeObs.EXPECT().ResendRequested(auth.ResendSuppressed).Once()

	first, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})
	require.NoError(t, err)
	second, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})
	require.NoError(t, err)

	assert.Equal(t, first.ResendAfter, second.ResendAfter, "both answers are indistinguishable")
}

// TestResendChallenge_StoreFailures pins the internal failure mapping on the
// resend path (OTP-17).
func TestResendChallenge_StoreFailures(t *testing.T) {
	ctx := context.Background()
	storeErr := errors.New("store down")

	tests := []struct {
		name string
		wire func(f *challengeFixture)
	}{
		{
			name: "Error: the fetch fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(nil, storeErr).Once()
			},
		},
		{
			name: "Error: the send count fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(f.validRow(validUserID, otherPurpose), nil).Once()
				f.expectAccountCount(0)
				f.mailer.EXPECT().SentSince(mock.Anything, mock.Anything).Return(0, storeErr).Once()
			},
		},
		{
			name: "Error: the extend fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(f.validRow(validUserID, otherPurpose), nil).Once()
				f.expectAccountCount(0)
				f.expectSentCount(0)
				f.expectResendCount(0)
				f.repo.EXPECT().Extend(mock.Anything, validUserID, challengeTestNow, mock.Anything, mock.Anything).Return(false, storeErr).Once()
			},
		},
		{
			name: "Error: the requeue fails",
			wire: func(f *challengeFixture) {
				f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(f.validRow(validUserID, otherPurpose), nil).Once()
				f.expectAccountCount(0)
				f.expectSentCount(0)
				f.expectResendCount(0)
				f.repo.EXPECT().Extend(mock.Anything, validUserID, challengeTestNow, mock.Anything, mock.Anything).Return(true, nil).Once()
				f.mailer.EXPECT().Requeue(mock.Anything, validFamily, challengeTestNow).Return("", storeErr).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			tc.wire(f)

			result, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

			assert.Nil(t, result)
			assertAppErr(t, err, apperr.CodeInternal)
		})
	}
}

// TestChallengeAccountBound pins the durable per-account bound on all three
// paths: past the configured window the answer is the typed rate-limit outcome
// with a retry hint before any further work, and a store failure is internal
// (OTP-14).
func TestChallengeAccountBound(t *testing.T) {
	ctx := context.Background()

	t.Run("Edge: issuance past the bound is refused before any work", func(t *testing.T) {
		f := newChallengeFixture(t)
		f.expectAccountCount(challengeTestPolicy.MaxPerUserWindow)

		_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

		assertAppErr(t, err, apperr.CodeRateLimited)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, challengeTestPolicy.UserWindow, appErr.RetryAfter, "the refusal carries a retry hint")
	})

	t.Run("Edge: verification past the bound is refused", func(t *testing.T) {
		f := newChallengeFixture(t)
		row := f.validRow(validUserID, otherPurpose)
		f.repo.EXPECT().ClaimAttempt(mock.Anything, validUserID, challengeTestNow, challengeTestPolicy.MaxAttempts).Return(row, nil).Once()
		f.expectAccountCount(challengeTestPolicy.MaxPerUserWindow)

		session, err := f.service.VerifyChallenge(ctx, auth.VerifyChallengeCommand{ChallengeID: validUserID, Code: challengeTestCode})

		assert.Nil(t, session)
		assertAppErr(t, err, apperr.CodeRateLimited)
	})

	t.Run("Edge: resend past the bound is refused without extending", func(t *testing.T) {
		f := newChallengeFixture(t)
		f.repo.EXPECT().Fetch(mock.Anything, validUserID).Return(f.validRow(validUserID, otherPurpose), nil).Once()
		f.expectAccountCount(challengeTestPolicy.MaxPerUserWindow)

		_, err := f.service.ResendChallenge(ctx, auth.ResendChallengeCommand{ChallengeID: validUserID})

		assertAppErr(t, err, apperr.CodeRateLimited)
	})

	t.Run("Happy path: one below the bound proceeds", func(t *testing.T) {
		f := newChallengeFixture(t)
		f.expectAccountCount(challengeTestPolicy.MaxPerUserWindow - 1)
		f.expectSentCount(0)
		f.repo.EXPECT().CloseActive(mock.Anything, validUserID, otherPurpose, challengeTestNow).Return(nil).Once()
		f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
		f.mailer.EXPECT().Record(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		f.codeObs.EXPECT().ChallengeIssued(otherPurpose).Once()

		_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

		require.NoError(t, err)
	})

	t.Run("Error: a store failure during the count is internal", func(t *testing.T) {
		f := newChallengeFixture(t)
		f.repo.EXPECT().
			CountForUserSince(mock.Anything, validUserID, challengeTestNow.Add(-challengeTestPolicy.UserWindow)).
			Return(0, errors.New("store down")).
			Once()

		_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

		assertAppErr(t, err, apperr.CodeInternal)
	})
}

// TestService_ChallengeClockIsInjected pins that the challenge path reads the
// injected clock for both the account window and the daily-cap origin, so no
// boundary needs a sleep (OTP-20, SC-007).
func TestService_ChallengeClockIsInjected(t *testing.T) {
	ctx := context.Background()
	f := newChallengeFixture(t)
	// Step the clock to a different UTC day, so a window derived from the real
	// system clock could not match the expectation below.
	f.now = challengeTestNow.Add(72 * time.Hour)

	f.repo.EXPECT().
		CountForUserSince(mock.Anything, validUserID, f.now.Add(-challengeTestPolicy.UserWindow)).
		Return(0, nil).
		Once()
	f.mailer.EXPECT().
		SentSince(mock.Anything, challengeTestPolicy.DayStart(f.now)).
		Return(challengeTestPolicy.DailySendCap, nil).
		Once()
	f.codeObs.EXPECT().SendCapBlocked().Once()

	_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})

	assertAppErr(t, err, apperr.CodeEmailUnavailable)
}

// TestChallengeLogsMaskTheAddress rules the disclosure helper: the first
// local-part character, three asterisks and the domain, with *** for anything
// unusable (OTP-15).
func TestChallengeLogsMaskTheAddress(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "Happy path: a full address keeps one character and the domain", email: "player@example.com", want: "p***@example.com"},
		{name: "Edge: a single-character local part is masked", email: "a@b.co", want: "a***@b.co"},
		{name: "Edge: an address without a domain becomes ***", email: "player", want: "***"},
		{name: "Edge: a trailing @ becomes ***", email: "player@", want: "***"},
		{name: "Edge: an empty value becomes ***", email: "", want: "***"},
	}

	ctx := context.Background()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			f.expectAccountCount(0)
			f.expectSentCount(0)
			f.repo.EXPECT().CloseActive(mock.Anything, validUserID, otherPurpose, challengeTestNow).Return(nil).Once()
			f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
			f.mailer.EXPECT().Record(mock.Anything, mock.Anything, tc.email, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			f.codeObs.EXPECT().ChallengeIssued(otherPurpose).Once()

			_, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: tc.email, Purpose: otherPurpose})
			require.NoError(t, err)

			assert.Contains(t, f.log.text(), "to="+tc.want)
			if tc.email != "" {
				assert.NotContains(t, f.log.text(), "to="+tc.email)
			}
		})
	}
}

// TestIssueChallenge_UsesConfiguredLifetime pins the copy rule behind the send
// intent contract: the recorded payload's expires_in_minutes is the configured
// lifetime, not a hardcoded constant, and the code never reaches a log line
// (OTP-15, R11).
func TestIssueChallenge_UsesConfiguredLifetime(t *testing.T) {
	ctx := context.Background()

	policy := challengeTestPolicy
	policy.Lifetime = 25 * time.Minute
	f := newChallengeFixtureWith(t, fixedCodeGenerator{code: challengeTestCode}, policy)

	f.expectAccountCount(0)
	f.expectSentCount(0)
	f.repo.EXPECT().CloseActive(mock.Anything, validUserID, otherPurpose, challengeTestNow).Return(nil).Once()
	f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
	f.mailer.EXPECT().
		Record(mock.Anything, mock.Anything, testEmail, otherPurpose, challengeTestCode, 25, challengeTestNow).
		Return(nil).
		Once()
	f.codeObs.EXPECT().ChallengeIssued(otherPurpose).Once()

	issued, err := f.service.IssueChallenge(ctx, auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: otherPurpose})
	require.NoError(t, err)
	assert.Equal(t, 1500, issued.ExpiresIn, "the claimed lifetime follows the configured policy")
	assert.NotContains(t, f.log.text(), challengeTestCode)
}

// TestChallengeService_ObserverDefaultsToNop pins that a service built without
// a challenge observer still runs, so an unwired caller stays correct.
func TestChallengeService_ObserverDefaultsToNop(t *testing.T) {
	service := auth.NewService(
		usermocks.NewMockRepository(t),
		mocks.NewMockRefreshTokenRepository(t),
		mocks.NewMockPasswordHasher(t),
		mocks.NewMockTokenManager(t),
		mocks.NewMockRefreshTokenGenerator(t),
		auth.NewCSRFTokenManager([]byte(testCSRFSecret)),
		mocks.NewMockObserver(t),
		auth.Config{RefreshTTL: testRefreshTTL, Now: func() time.Time { return challengeTestNow }},
		logger.NewLogger(slog.LevelError),
		auth.ChallengeDeps{},
	)
	require.NotNil(t, service)
}

// TestIssueChallenge_ConcurrentIssuance covers two issuances for one account
// racing between the close and the insert: the loser retries a bounded number of
// times and then fails closed rather than leaving two active codes.
func TestIssueChallenge_ConcurrentIssuance(t *testing.T) {
	ctx := context.Background()
	cmd := auth.IssueChallengeCommand{UserID: validUserID, Email: testEmail, Purpose: auth.PurposeVerifyEmail}

	tests := []struct {
		name      string
		conflicts int
		wantErr   bool
	}{
		{name: "Happy path: no conflict inserts once", conflicts: 0},
		{name: "Edge: one lost race is retried and succeeds", conflicts: 1},
		{name: "Edge: the last allowed attempt still succeeds", conflicts: 2},
		{name: "Error: persistent conflict answers internal", conflicts: 3, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newChallengeFixture(t)
			f.expectAccountCount(0)
			f.expectSentCount(0)
			attempts := tc.conflicts
			if !tc.wantErr {
				attempts++
			}
			f.repo.EXPECT().CloseActive(mock.Anything, validUserID, auth.PurposeVerifyEmail, challengeTestNow).Return(nil).Times(attempts)
			if tc.conflicts > 0 {
				f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(auth.ErrChallengeActive).Times(tc.conflicts)
			}
			if !tc.wantErr {
				f.repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()
				f.mailer.EXPECT().
					Record(mock.Anything, mock.Anything, testEmail, auth.PurposeVerifyEmail, challengeTestCode, 10, challengeTestNow).
					Return(nil).
					Once()
				f.codeObs.EXPECT().ChallengeIssued(auth.PurposeVerifyEmail).Once()
			}

			issued, err := f.service.IssueChallenge(ctx, cmd)
			if tc.wantErr {
				assertAppErr(t, err, apperr.CodeInternal)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, issued.ChallengeID)
		})
	}
}
