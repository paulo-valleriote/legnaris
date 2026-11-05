package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

// minPasswordLength is the shortest accepted password in bytes.
// bcrypt.GenerateFromPassword rejects anything over 72 bytes.
const minPasswordLength = 8

// maxPasswordLength is the longest accepted password in bytes.
const maxPasswordLength = 72

// Username validation constants.
const maxUsernameLength = 50

// usernameRegex allows alphanumeric, underscore, hyphen. No leading/trailing spaces (trimmed before check).
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// attrUserID is the structured-log field carrying the authenticated user id.
const attrUserID = "user_id"

type RegisterCommand struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginCommand struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshCommand and LogoutCommand carry the refresh credential read from the
// __Host-legnaris_refresh cookie and the CSRF token read from the X-CSRF-Token
// header. Neither has a JSON body by design (AUTH-05, AUTH-09).
type RefreshCommand struct {
	RefreshToken string
	CSRFToken    string
}

type LogoutCommand struct {
	RefreshToken string
	CSRFToken    string
}

// Session is the material Register, Login and Refresh hand to the controller.
// Only the controller decides what becomes a cookie and what becomes a body
// (AUTH-01, AUTH-02, AUTH-03). RefreshToken and CSRFToken never reach a body.
type Session struct {
	AccessToken   string
	ExpiresIn     int
	RefreshToken  string
	RefreshMaxAge int
	CSRFToken     string
	UserID        string
}

// LoginResult is what a password sign-in answers. A verified account receives
// the Session; an unverified one receives a started verify_email code step and
// no credential at all, so the session is never issued before the address is
// proven (D2, TFA-06). Exactly one of the two fields is non-nil.
type LoginResult struct {
	Session   *Session
	Challenge *ChallengeIssued
}

// SessionResponse is the login, registration and refresh body. It has no
// refresh field, so the credential appears only in Set-Cookie (AUTH-01,
// AUTH-02, AUTH-03, AUTH-08).
type SessionResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Config carries the refresh-token lifetime resolved from the environment.
// The access-token TTL is a TokenManager concern, not a service one.
type Config struct {
	RefreshTTL time.Duration
	// Now is the single clock for timestamps written or compared here. Nil
	// means the system clock (PRQ-16).
	Now Clock
	// EmailChecker refuses disposable or undeliverable domains at registration.
	// Nil disables the check.
	EmailChecker EmailChecker
}

// EmailChecker decides whether an address may open an account. Check returns a
// non-nil error to refuse it; the service runs it before any write, so a refused
// registration leaves no account behind (LGN-151, D1).
type EmailChecker interface {
	Check(ctx context.Context, email string) error
}

// ChallengeDeps bundles the challenge collaborators so adding them does not
// grow NewService's parameter list without bound (T017). The service owns the
// policy; the store, the code toolkit, the mailer port and the observer are
// injected.
type ChallengeDeps struct {
	Repository ChallengeRepository
	Generator  CodeGenerator
	Digest     *CodeDigest
	Mailer     CodeMailer
	Observer   ChallengeObserver
	Policy     ChallengePolicy
}

type Service interface {
	Register(ctx context.Context, cmd RegisterCommand) (*ChallengeIssued, error)
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	Refresh(ctx context.Context, cmd RefreshCommand) (*Session, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
	IssueChallenge(ctx context.Context, cmd IssueChallengeCommand) (*ChallengeIssued, error)
	VerifyChallenge(ctx context.Context, cmd VerifyChallengeCommand) (*Session, error)
	ResendChallenge(ctx context.Context, cmd ResendChallengeCommand) (*ResendResult, error)
	Profile(ctx context.Context, userID string) (*user.UserDTO, error)
}

type ServiceImpl struct {
	users         user.Repository
	refreshTokens RefreshTokenRepository
	hasher        PasswordHasher
	tokens        TokenManager
	generator     RefreshTokenGenerator
	csrf          CSRFTokenManager
	observer      Observer
	log           logger.StructuredLogger
	cfg           Config
	clock         Clock
	challenge     ChallengeDeps
}

func NewService(
	users user.Repository,
	refreshTokens RefreshTokenRepository,
	hasher PasswordHasher,
	tokens TokenManager,
	generator RefreshTokenGenerator,
	csrf CSRFTokenManager,
	observer Observer,
	cfg Config,
	log logger.StructuredLogger,
	challenge ChallengeDeps,
) Service {
	if challenge.Observer == nil {
		challenge.Observer = NewNopChallengeObserver()
	}
	return &ServiceImpl{
		users:         users,
		refreshTokens: refreshTokens,
		hasher:        hasher,
		tokens:        tokens,
		generator:     generator,
		csrf:          csrf,
		observer:      observer,
		log:           log,
		cfg:           cfg,
		clock:         cfg.Now,
		challenge:     challenge,
	}
}

// Register creates the account and starts the verify_email code step; it never
// issues a session, because the address must be confirmed first. A duplicate
// email still answers 409 before any code is sent (AUTH-02, TFA-01).
func (s *ServiceImpl) Register(ctx context.Context, cmd RegisterCommand) (*ChallengeIssued, error) {
	email, err := user.CanonicalEmail(cmd.Email)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidEmail, err)
	}
	if s.cfg.EmailChecker != nil {
		if err := s.cfg.EmailChecker.Check(ctx, email); err != nil {
			return nil, apperr.NewError(apperr.CodeDisposableEmail, MsgDisposableEmail, err)
		}
	}
	username := strings.TrimSpace(cmd.Username)
	if username == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgUsernameRequired, nil)
	}
	if len(username) > maxUsernameLength {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgUsernameTooLong, nil)
	}
	if !usernameRegex.MatchString(username) {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgUsernameInvalid, nil)
	}
	if len(cmd.Password) < minPasswordLength {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgPasswordTooShort, nil)
	}
	if len(cmd.Password) > maxPasswordLength {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgPasswordTooLong, nil)
	}

	hashed, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	id, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	now := s.clock.Now()
	newUser := &user.User{
		ID:           id,
		Email:        email,
		Username:     username,
		PasswordHash: hashed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, newUser); err != nil {
		if errors.Is(err, user.ErrEmailAlreadyExists) {
			return nil, apperr.NewError(apperr.CodeConflict, MsgEmailAlreadyExists, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	issued, err := s.startChallenge(ctx, newUser.ID, newUser.Email, PurposeVerifyEmail)
	if err != nil {
		if isDailyAllowanceRefusal(err) {
			return s.deferredRegistration(ctx, newUser.ID), nil
		}
		return nil, err
	}

	s.log.Info(ctx, "registration challenge started", logger.Attribute{Key: attrUserID, Value: newUser.ID})
	return issued, nil
}

// isDailyAllowanceRefusal reports whether err is the send cap's refusal. It is
// the only unavailable outcome an issuance raises during registration.
func isDailyAllowanceRefusal(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.CodeEmailUnavailable
}

// deferredRegistration answers a registration whose code could not be queued:
// the account already exists unverified, and no challenge or session is issued.
// The next sign-in starts the verify_email step (D1, D2).
func (s *ServiceImpl) deferredRegistration(ctx context.Context, userID string) *ChallengeIssued {
	s.log.Info(ctx, "registration deferred by daily allowance", logger.Attribute{Key: attrUserID, Value: userID})
	return &ChallengeIssued{Delivery: DeliveryDeferred}
}

// Login proves the password and then answers by the account's verification
// state: a verified account gets a session, an unverified one starts the
// verify_email step. Every credential failure answers the one uniform outcome
// and starts nothing (TFA-02, TFA-05, TFA-06).
func (s *ServiceImpl) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	email, err := user.CanonicalEmail(cmd.Email)
	if err != nil {
		// An unparseable address is indistinguishable from an unknown one, so the
		// response never reveals whether an account could have existed (PRQ-09,
		// PRQ-10, PRQ-17). The password work is still spent, so the timing
		// class is the same as the known-account path (TFA-05).
		s.spendPasswordWork(cmd.Password)
		return nil, s.invalidCredentials(ctx, "unparseable email")
	}

	stored, err := s.users.FetchByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			s.spendPasswordWork(cmd.Password)
			return nil, s.invalidCredentials(ctx, "unknown email")
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.hasher.Compare(stored.PasswordHash, cmd.Password); err != nil {
		return nil, s.invalidCredentials(ctx, "password mismatch")
	}

	if stored.EmailVerifiedAt == nil {
		return s.loginChallenge(ctx, stored)
	}

	session, err := s.signIn(ctx, stored.ID)
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "login succeeded", logger.Attribute{Key: attrUserID, Value: stored.ID})
	return &LoginResult{Session: session}, nil
}

// loginChallenge starts the verify_email step for an account whose password
// was proven but whose address was never verified. The daily allowance and the
// per-account bound apply exactly as they do to registration: a refusal reaches
// the caller as its typed error (503 EMAIL_UNAVAILABLE or 429 RATE_LIMITED).
func (s *ServiceImpl) loginChallenge(ctx context.Context, account *user.User) (*LoginResult, error) {
	issued, err := s.startChallenge(ctx, account.ID, account.Email, PurposeVerifyEmail)
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "login challenge started", logger.Attribute{Key: attrUserID, Value: account.ID})
	return &LoginResult{Challenge: issued}, nil
}

// startChallenge starts the code step a gated registration or sign-in owes. The
// purpose is bound here, from the operation that reached this point, and is
// never read from a command the client controls (TFA-03).
func (s *ServiceImpl) startChallenge(ctx context.Context, userID, email string, purpose Purpose) (*ChallengeIssued, error) {
	if s.challenge.Repository == nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, nil)
	}
	return s.IssueChallenge(ctx, IssueChallengeCommand{
		UserID:  userID,
		Email:   email,
		Purpose: purpose,
	})
}

// spendPasswordWork performs the password comparison the unknown-account path
// owes, so the response time of a sign-in for an address that does not exist
// falls into the same class as the one for an address whose password is wrong.
// The comparison runs against the hasher's dummy hash — a bcrypt digest at the
// configured work factor that no account stores — and its result is discarded
// (TFA-05, R5).
func (s *ServiceImpl) spendPasswordWork(password string) {
	_ = s.hasher.Compare(s.hasher.DummyHash(), password)
}

// Refresh consumes the presented cookie credential, verifies the CSRF token
// against the resolved family before any mutation, and rotates both the refresh
// and CSRF values together (AUTH-03, AUTH-10, AUTH-11, AUTH-12).
func (s *ServiceImpl) Refresh(ctx context.Context, cmd RefreshCommand) (*Session, error) {
	stored, err := s.resolveCredential(ctx, cmd)
	if err != nil {
		return nil, err
	}

	// Renewal must not survive the account it names: load it after the existing
	// credential rules and before the first issuance, so a missing account can
	// never receive a credential (PRQ-05, PRQ-06).
	account, err := s.users.FetchByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, s.revokeRenewal(ctx, stored.FamilyID, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	// An account that never verified its address holds no renewable session
	// (D2, O5): sessions minted before verification existed are contained here.
	if account.EmailVerifiedAt == nil {
		return nil, s.revokeRenewal(ctx, stored.FamilyID, nil)
	}

	access, err := s.tokens.Issue(account.ID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	issued, err := s.issueRefreshToken(ctx, account.ID, stored.FamilyID)
	if err != nil {
		return nil, err
	}

	// Atomically claim the rotation. If another request already rotated this token,
	// the claim fails (MatchedCount=0). In that case, we do NOT revoke the family
	// (which would invalidate the valid successor created by the other request).
	// Instead, we return an error indicating the token was already rotated.
	// The client that lost the race will need to re-authenticate, but the other
	// client's valid tokens remain intact.
	claimed, err := s.refreshTokens.RevokeByID(ctx, stored.ID, issued.id)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !claimed {
		// Concurrent rotation: another request already rotated this token.
		// The successor token we created is valid and in the family.
		// Do NOT revoke the family — that would invalidate the other client's tokens.
		// Return an error; the client must re-authenticate (their refresh token is now revoked).
		s.log.Info(ctx, "refresh concurrent rotation detected", logger.Attribute{Key: attrUserID, Value: account.ID})
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, nil)
	}

	session, err := s.newSession(ctx, account.ID, issued.raw, issued.familyID, access)
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "refresh rotated", logger.Attribute{Key: attrUserID, Value: account.ID})
	return session, nil
}

// revokeRenewal ends the family of a credential whose account may not renew
// it and answers the uniform refresh refusal. The missing-account and the
// unverified-account paths share it, so neither is distinguishable by status or
// body (TFA-06, TFA-10).
func (s *ServiceImpl) revokeRenewal(ctx context.Context, familyID string, cause error) error {
	if err := s.refreshTokens.RevokeFamily(ctx, familyID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, cause)
}

// resolveCredential applies the credential rules to the presented cookie
// credential, in the order they decide: the credential must be present, known,
// carry a valid CSRF token for its family, not be revoked and not be expired.
// An unknown credential is refused before the CSRF token is even looked at
// (AUTH-03, AUTH-10, AUTH-11, AUTH-12).
func (s *ServiceImpl) resolveCredential(ctx context.Context, cmd RefreshCommand) (*RefreshToken, error) {
	if cmd.RefreshToken == "" {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgRefreshTokenRequired, nil)
	}

	stored, err := s.refreshTokens.FetchByHash(ctx, HashRefreshToken(cmd.RefreshToken))
	if err != nil {
		if errors.Is(err, ErrRefreshTokenNotFound) {
			return nil, apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.csrf.Verify(cmd.CSRFToken, stored.FamilyID); err != nil {
		return nil, apperr.NewError(apperr.CodeForbidden, MsgCSRFInvalid, err)
	}

	if stored.Revoked {
		return nil, s.reuseDetected(ctx, stored)
	}
	if !s.clock.Now().Before(stored.ExpiresAt) {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, nil)
	}

	return stored, nil
}

// Logout revokes the resolved session's family. A missing or unknown credential
// is an idempotent no-op and owes no CSRF check; a resolved session must present
// a valid CSRF token (AUTH-04, AUTH-06, AUTH-10).
func (s *ServiceImpl) Logout(ctx context.Context, cmd LogoutCommand) error {
	if cmd.RefreshToken == "" {
		return nil
	}

	stored, err := s.refreshTokens.FetchByHash(ctx, HashRefreshToken(cmd.RefreshToken))
	if err != nil {
		// Logout is idempotent: an unknown token has no session left to end.
		if errors.Is(err, ErrRefreshTokenNotFound) {
			return nil
		}
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.csrf.Verify(cmd.CSRFToken, stored.FamilyID); err != nil {
		return apperr.NewError(apperr.CodeForbidden, MsgCSRFInvalid, err)
	}

	if err := s.refreshTokens.RevokeFamily(ctx, stored.FamilyID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	s.log.Info(ctx, "logout succeeded")
	return nil
}

// signIn mints a brand-new session for a user: a fresh family of refresh
// credentials plus the CSRF token bound to it. It is the shared tail of
// Register and Login.
func (s *ServiceImpl) signIn(ctx context.Context, userID string) (*Session, error) {
	access, err := s.tokens.Issue(userID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	issued, err := s.issueRefreshToken(ctx, userID, "")
	if err != nil {
		return nil, err
	}

	return s.newSession(ctx, userID, issued.raw, issued.familyID, access)
}

// newSession binds a freshly minted CSRF token to the session's family so the
// controller can set the refresh and CSRF cookies together (AUTH-09, AUTH-12).
func (s *ServiceImpl) newSession(ctx context.Context, userID, rawRefresh, familyID string, access AccessToken) (*Session, error) {
	csrfToken, err := s.csrf.Issue(familyID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	return &Session{
		AccessToken:   access.Value,
		ExpiresIn:     access.ExpiresIn,
		RefreshToken:  rawRefresh,
		RefreshMaxAge: int(s.cfg.RefreshTTL.Seconds()),
		CSRFToken:     csrfToken,
		UserID:        userID,
	}, nil
}

// issuedRefresh is the persisted credential plus the material the caller needs:
// the raw value for the cookie, the family for the CSRF binding, and the row id
// that rotation points the predecessor's replaced_by at.
type issuedRefresh struct {
	raw      string
	id       string
	familyID string
}

// issueRefreshToken persists a refresh token for the given family, minting a
// new family when familyID is empty. It returns the raw token — only its hash
// is persisted — the family id and the new token's id.
func (s *ServiceImpl) issueRefreshToken(ctx context.Context, userID string, familyID string) (issuedRefresh, error) {
	if familyID == "" {
		generated, err := idgen.New()
		if err != nil {
			return issuedRefresh{}, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
		}
		familyID = generated
	}

	id, err := idgen.New()
	if err != nil {
		return issuedRefresh{}, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	raw, err := s.generator.Generate()
	if err != nil {
		return issuedRefresh{}, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	now := s.clock.Now()
	token := &RefreshToken{
		ID:        id,
		TokenHash: HashRefreshToken(raw),
		UserID:    userID,
		FamilyID:  familyID,
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
		CreatedAt: now,
	}
	if err := s.refreshTokens.Create(ctx, token); err != nil {
		return issuedRefresh{}, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	return issuedRefresh{raw: raw, id: id, familyID: familyID}, nil
}

func (s *ServiceImpl) invalidCredentials(ctx context.Context, reason string) error {
	s.observer.LoginFailure()
	s.log.Warn(ctx, "login failed", logger.Attribute{Key: "reason", Value: reason})
	return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidCredentials, nil)
}

func (s *ServiceImpl) reuseDetected(ctx context.Context, token *RefreshToken) error {
	s.observer.RefreshReuseDetected()
	if err := s.refreshTokens.RevokeFamily(ctx, token.FamilyID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	s.log.Warn(
		ctx,
		"refresh token reuse detected; family revoked",
		logger.Attribute{Key: attrUserID, Value: token.UserID},
	)
	return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, nil)
}
