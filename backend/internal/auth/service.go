package auth

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

const minPasswordLength = 8

const maxPasswordLength = 72

const maxUsernameLength = 50

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

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

type RefreshCommand struct {
	RefreshToken string
	CSRFToken    string
}

type LogoutCommand struct {
	RefreshToken string
	CSRFToken    string
}

type Session struct {
	AccessToken   string
	ExpiresIn     int
	RefreshToken  string
	RefreshMaxAge int
	CSRFToken     string
	UserID        string
}

type LoginResult struct {
	Session   *Session
	Challenge *ChallengeIssued
}

type SessionResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type Config struct {
	RefreshTTL          time.Duration
	Now                 Clock
	EmailChecker        EmailChecker
	RegisterMinDuration time.Duration
}

type EmailChecker interface {
	Check(ctx context.Context, email string) error
}

type ChallengeDeps struct {
	Repository    ChallengeRepository
	Generator     CodeGenerator
	Digest        *CodeDigest
	Mailer        CodeMailer
	Notices       NoticeRepository
	Observer      ChallengeObserver
	Policy        ChallengePolicy
	EmailVerifier EmailVerifier
}

type EmailVerifier interface {
	EmailVerified(ctx context.Context, userID, email, clientIP string)
}

type Service interface {
	Register(ctx context.Context, cmd RegisterCommand) (*ChallengeIssued, error)
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	Refresh(ctx context.Context, cmd RefreshCommand) (*Session, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
	IssueCSRF(ctx context.Context, refreshToken string) (string, error)
	IssueChallenge(ctx context.Context, cmd IssueChallengeCommand) (*ChallengeIssued, error)
	VerifyChallenge(ctx context.Context, cmd VerifyChallengeCommand) (*Session, error)
	ResendChallenge(ctx context.Context, cmd ResendChallengeCommand) (*ResendResult, error)
	Profile(ctx context.Context, userID string) (*user.UserDTO, error)
	RequestAccountDeletion(ctx context.Context, userID string) (*ChallengeIssued, error)
	DeleteAccount(ctx context.Context, cmd DeleteAccountCommand) error
	EnsureActive(ctx context.Context, userID string) error
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
	publisher     events.Publisher
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
	publisher events.Publisher,
) Service {
	if challenge.Observer == nil {
		challenge.Observer = NewNopChallengeObserver()
	}
	if publisher == nil {
		publisher = events.NoOp{}
	}
	return &ServiceImpl{
		publisher:     publisher,
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

func (s *ServiceImpl) Register(ctx context.Context, cmd RegisterCommand) (*ChallengeIssued, error) {
	started := time.Now()
	issued, err := s.register(ctx, cmd)
	s.padRegistration(ctx, started)
	return issued, err
}

func (s *ServiceImpl) padRegistration(ctx context.Context, started time.Time) {
	remaining := s.cfg.RegisterMinDuration - time.Since(started)
	if remaining <= 0 {
		return
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func (s *ServiceImpl) register(ctx context.Context, cmd RegisterCommand) (*ChallengeIssued, error) {
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
		return s.registrationCreateFailed(ctx, err, email, username, hashed)
	}
	s.publisher.Publish(userRegisteredEvent(newUser.ID))

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

func (s *ServiceImpl) registrationCreateFailed(ctx context.Context, err error, email, username, hashed string) (*ChallengeIssued, error) {
	if errors.Is(err, user.ErrUsernameAlreadyExists) || errors.Is(err, user.ErrEmailAlreadyExists) {
		return s.reregisterUnverified(ctx, email, username, hashed, err)
	}
	return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
}

func (s *ServiceImpl) reregisterUnverified(ctx context.Context, email, username, hashed string, createErr error) (*ChallengeIssued, error) {
	pending, err := s.users.FetchByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return s.freeAddressCollision(createErr)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	usernameOwner := ""
	if pending.EmailVerifiedAt == nil {
		usernameOwner = pending.ID
	}
	free, err := s.usernameFree(ctx, username, usernameOwner)
	if err != nil {
		return nil, err
	}
	if !free {
		return nil, usernameConflict(user.ErrUsernameAlreadyExists)
	}
	if pending.EmailVerifiedAt != nil {
		return s.noticeAttempt(ctx, PurposeAccountExists, email, pending.ID)
	}

	replaced, err := s.users.ReplaceUnverifiedCredentials(ctx, pending.ID, username, hashed, s.clock.Now())
	if errors.Is(err, user.ErrUsernameAlreadyExists) {
		return nil, usernameConflict(err)
	}
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !replaced {
		return s.noticeAttempt(ctx, PurposeAccountExists, email, pending.ID)
	}

	if err := s.refreshTokens.RevokeAllForUser(ctx, pending.ID); err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	issued, err := s.startChallenge(ctx, pending.ID, email, PurposeVerifyEmail)
	if err != nil {
		if isDailyAllowanceRefusal(err) {
			return s.deferredRegistration(ctx, pending.ID), nil
		}
		return nil, err
	}

	s.log.Info(ctx, "unverified registration replaced", logger.Attribute{Key: attrUserID, Value: pending.ID})
	return issued, nil
}

func (s *ServiceImpl) usernameFree(ctx context.Context, username, ownerID string) (bool, error) {
	holder, err := s.users.FetchByUsername(ctx, username)
	if errors.Is(err, user.ErrUserNotFound) {
		return true, nil
	}
	if err != nil {
		return false, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return holder.ID == ownerID, nil
}

func (s *ServiceImpl) freeAddressCollision(createErr error) (*ChallengeIssued, error) {
	if errors.Is(createErr, user.ErrUsernameAlreadyExists) {
		return nil, usernameConflict(createErr)
	}
	return nil, registrationConflict(createErr)
}

func usernameConflict(cause error) error {
	return apperr.NewError(apperr.CodeConflict, MsgUsernameTaken, cause)
}

func registrationConflict(cause error) error {
	return apperr.NewError(apperr.CodeConflict, MsgRegistrationConflict, cause)
}

func (s *ServiceImpl) noticeAttempt(ctx context.Context, purpose Purpose, to, ownerID string) (*ChallengeIssued, error) {
	if s.challenge.Mailer == nil || s.challenge.Notices == nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, nil)
	}
	decoyID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	notified, err := s.sendNotice(ctx, purpose, to, s.clock.Now())
	if err != nil {
		if isDailyAllowanceRefusal(err) {
			return s.deferredRegistration(ctx, ownerID), nil
		}
		return nil, err
	}
	s.log.Info(ctx, "registration answered uniformly",
		logger.Attribute{Key: attrUserID, Value: ownerID},
		logger.Attribute{Key: attrPurpose, Value: string(purpose)},
		logger.Attribute{Key: "notified", Value: strconv.FormatBool(notified)},
	)

	policy := s.challenge.Policy
	return &ChallengeIssued{
		ChallengeID: decoyID,
		ExpiresIn:   int(policy.Lifetime.Seconds()),
		ResendAfter: int(policy.ResendCooldown.Seconds()),
	}, nil
}

func (s *ServiceImpl) sendNotice(ctx context.Context, purpose Purpose, to string, now time.Time) (bool, error) {
	policy := s.challenge.Policy
	if err := s.checkSendBudget(ctx, now); err != nil {
		return false, err
	}

	claimed, err := s.challenge.Notices.Claim(ctx, purpose, to, now.Add(-policy.UserWindow), now)
	if err != nil {
		return false, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !claimed {
		return false, nil
	}

	intentID, err := idgen.New()
	if err != nil {
		return false, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	err = s.challenge.Mailer.Record(ctx, intentID, to, purpose, "", int(policy.Lifetime.Minutes()), now)
	if errors.Is(err, ErrSendBudgetExhausted) {
		return false, s.sendBudgetExhausted(ctx, now)
	}
	if err != nil {
		return false, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return true, nil
}

func isDailyAllowanceRefusal(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.CodeEmailUnavailable
}

func (s *ServiceImpl) deferredRegistration(ctx context.Context, userID string) *ChallengeIssued {
	s.log.Info(ctx, "registration deferred by daily allowance", logger.Attribute{Key: attrUserID, Value: userID})
	return &ChallengeIssued{Delivery: DeliveryDeferred}
}

func (s *ServiceImpl) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	email, err := user.CanonicalEmail(cmd.Email)
	if err != nil {
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

	if stored.IsDeleting() {
		return nil, s.invalidCredentials(ctx, "account deleting")
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

func (s *ServiceImpl) loginChallenge(ctx context.Context, account *user.User) (*LoginResult, error) {
	issued, err := s.startChallenge(ctx, account.ID, account.Email, PurposeVerifyEmail)
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "login challenge started", logger.Attribute{Key: attrUserID, Value: account.ID})
	return &LoginResult{Challenge: issued}, nil
}

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

func (s *ServiceImpl) spendPasswordWork(password string) {
	_ = s.hasher.Compare(s.hasher.DummyHash(), password)
}

func (s *ServiceImpl) Refresh(ctx context.Context, cmd RefreshCommand) (*Session, error) {
	stored, err := s.resolveCredential(ctx, cmd)
	if err != nil {
		return nil, err
	}

	account, err := s.users.FetchByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, s.revokeRenewal(ctx, stored.FamilyID, err)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if account.EmailVerifiedAt == nil {
		return nil, s.revokeRenewal(ctx, stored.FamilyID, nil)
	}

	if account.IsDeleting() {
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

	claimed, err := s.refreshTokens.RevokeByID(ctx, stored.ID, issued.id)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !claimed {
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

func (s *ServiceImpl) revokeRenewal(ctx context.Context, familyID string, cause error) error {
	if err := s.refreshTokens.RevokeFamily(ctx, familyID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, cause)
}

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

func (s *ServiceImpl) Logout(ctx context.Context, cmd LogoutCommand) error {
	if cmd.RefreshToken == "" {
		return nil
	}

	stored, err := s.refreshTokens.FetchByHash(ctx, HashRefreshToken(cmd.RefreshToken))
	if err != nil {
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

func (s *ServiceImpl) IssueCSRF(ctx context.Context, refreshToken string) (string, error) {
	if refreshToken == "" {
		return "", apperr.NewError(apperr.CodeUnauthorized, MsgRefreshTokenRequired, nil)
	}

	stored, err := s.refreshTokens.FetchByHash(ctx, HashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, ErrRefreshTokenNotFound) {
			return "", apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, err)
		}
		return "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if stored.Revoked || !s.clock.Now().Before(stored.ExpiresAt) {
		return "", apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, nil)
	}

	token, err := s.csrf.Issue(stored.FamilyID)
	if err != nil {
		return "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return token, nil
}

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

type issuedRefresh struct {
	raw      string
	id       string
	familyID string
}

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
