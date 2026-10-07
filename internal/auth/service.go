package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"legnaris/internal/user"
	"legnaris/shared/apperr"
	"legnaris/shared/idgen"
	"legnaris/shared/logger"
)

// minPasswordLength is the shortest accepted password. Deeper policy (breach
// lists, character classes) is a hardening concern, not Phase 1.
const minPasswordLength = 8

// maxPasswordLength is the longest accepted password, in bytes:
// bcrypt.GenerateFromPassword rejects anything over 72 bytes
// (ErrPasswordTooLong), so longer input must fail validation as a client
// error instead of surfacing as a 500 from the hasher.
const maxPasswordLength = 72

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

// RefreshCommand and LogoutCommand carry the refresh token read from the
// X-Refresh-Token header. They have no JSON body by design.
type RefreshCommand struct {
	RefreshToken string
}

type LogoutCommand struct {
	RefreshToken string
}

// TokenPair is the login/refresh response body. The refresh token is an opaque
// credential the client stores in the OS secure storage.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Config carries the refresh-token lifetime resolved from the environment.
// The access-token TTL is a TokenManager concern, not a service one.
type Config struct {
	RefreshTTL time.Duration
}

type Service interface {
	Register(ctx context.Context, cmd RegisterCommand) (*user.User, error)
	Login(ctx context.Context, cmd LoginCommand) (*TokenPair, error)
	Refresh(ctx context.Context, cmd RefreshCommand) (*TokenPair, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
}

type ServiceImpl struct {
	users         user.Repository
	refreshTokens RefreshTokenRepository
	hasher        PasswordHasher
	tokens        TokenManager
	generator     RefreshTokenGenerator
	observer      Observer
	log           logger.StructuredLogger
	cfg           Config
}

func NewService(
	users user.Repository,
	refreshTokens RefreshTokenRepository,
	hasher PasswordHasher,
	tokens TokenManager,
	generator RefreshTokenGenerator,
	observer Observer,
	cfg Config,
	log logger.StructuredLogger,
) Service {
	return &ServiceImpl{
		users:         users,
		refreshTokens: refreshTokens,
		hasher:        hasher,
		tokens:        tokens,
		generator:     generator,
		observer:      observer,
		log:           log,
		cfg:           cfg,
	}
}

func (s *ServiceImpl) Register(ctx context.Context, cmd RegisterCommand) (*user.User, error) {
	email := normalizeEmail(cmd.Email)
	username := strings.TrimSpace(cmd.Username)

	if _, err := mail.ParseAddress(email); err != nil {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidEmail, err)
	}
	if username == "" {
		return nil, apperr.NewError(apperr.CodeInvalidRequest, MsgUsernameRequired, nil)
	}
	if utf8.RuneCountInString(cmd.Password) < minPasswordLength {
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

	now := time.Now()
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

	s.log.Info(ctx, "user registered", logger.Attribute{Key: attrUserID, Value: newUser.ID})
	return newUser, nil
}

func (s *ServiceImpl) Login(ctx context.Context, cmd LoginCommand) (*TokenPair, error) {
	email := normalizeEmail(cmd.Email)

	stored, err := s.users.FetchByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, s.invalidCredentials(ctx, "unknown email")
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.hasher.Compare(stored.PasswordHash, cmd.Password); err != nil {
		return nil, s.invalidCredentials(ctx, "password mismatch")
	}

	access, err := s.tokens.Issue(stored.ID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	raw, _, err := s.issueRefreshToken(ctx, stored.ID, "")
	if err != nil {
		return nil, err
	}

	s.log.Info(ctx, "login succeeded", logger.Attribute{Key: attrUserID, Value: stored.ID})
	return s.tokenPair(access, raw), nil
}

func (s *ServiceImpl) Refresh(ctx context.Context, cmd RefreshCommand) (*TokenPair, error) {
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

	if stored.Revoked {
		return nil, s.reuseDetected(ctx, stored)
	}
	if time.Now().After(stored.ExpiresAt) {
		return nil, apperr.NewError(apperr.CodeUnauthorized, MsgInvalidRefreshToken, nil)
	}

	access, err := s.tokens.Issue(stored.UserID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	rawNew, newID, err := s.issueRefreshToken(ctx, stored.UserID, stored.FamilyID)
	if err != nil {
		return nil, err
	}

	// Atomically claim the rotation, only after the successor is persisted:
	// any failure before this point leaves the old token intact and
	// retryable, and a false result (another request rotated first —
	// indistinguishable from theft) revokes a family that already contains
	// the successor.
	claimed, err := s.refreshTokens.RevokeByID(ctx, stored.ID, newID)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !claimed {
		return nil, s.reuseDetected(ctx, stored)
	}

	s.log.Info(ctx, "refresh rotated", logger.Attribute{Key: attrUserID, Value: stored.UserID})
	return s.tokenPair(access, rawNew), nil
}

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

	if err := s.refreshTokens.RevokeFamily(ctx, stored.FamilyID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	s.log.Info(ctx, "logout succeeded")
	return nil
}

// issueRefreshToken persists a refresh token for the given family, minting a
// new family when familyID is empty. It returns the raw token — only its hash
// is persisted — and the new token's id, which rotation needs to point the
// predecessor's replaced_by at its heir.
func (s *ServiceImpl) issueRefreshToken(ctx context.Context, userID string, familyID string) (string, string, error) {
	if familyID == "" {
		generated, err := idgen.New()
		if err != nil {
			return "", "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
		}
		familyID = generated
	}

	id, err := idgen.New()
	if err != nil {
		return "", "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	raw, err := s.generator.Generate()
	if err != nil {
		return "", "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	now := time.Now()
	token := &RefreshToken{
		ID:        id,
		TokenHash: HashRefreshToken(raw),
		UserID:    userID,
		FamilyID:  familyID,
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
		CreatedAt: now,
	}
	if err := s.refreshTokens.Create(ctx, token); err != nil {
		return "", "", apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	return raw, id, nil
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

func (s *ServiceImpl) tokenPair(access AccessToken, refresh string) *TokenPair {
	return &TokenPair{
		AccessToken:  access.Value,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    access.ExpiresIn,
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
