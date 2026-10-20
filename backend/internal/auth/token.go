package auth

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/idgen"

	"github.com/golang-jwt/jwt/v5"
)

// AccessToken is an issued JWT and its remaining lifetime in seconds, ready to
// be serialized as the access_token/expires_in pair.
type AccessToken struct {
	Value     string
	ExpiresIn int
}

// TokenManager issues and validates access tokens. It is an interface so the
// auth service and middleware can be tested with a deterministic fake.
type TokenManager interface {
	Issue(userID string) (AccessToken, error)
	Parse(token string) (userID string, err error)
}

type jwtTokenManager struct {
	secret []byte
	ttl    time.Duration
	clock  Clock
}

// NewTokenManager builds the HS256 token manager. An empty secret is a startup
// error: a service that cannot sign tokens must not boot. A nil clock defaults
// to the system clock (PRQ-16).
func NewTokenManager(secret string, ttl time.Duration, clock Clock) (TokenManager, error) {
	if secret == "" {
		return nil, ErrEmptyJWTSecret
	}
	return &jwtTokenManager{secret: []byte(secret), ttl: ttl, clock: clock}, nil
}

func (m *jwtTokenManager) Issue(userID string) (AccessToken, error) {
	now := m.clock.Now()

	jti, err := idgen.New()
	if err != nil {
		return AccessToken{}, fmt.Errorf("generate token id: %w", err)
	}

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return AccessToken{}, fmt.Errorf("sign access token: %w", err)
	}

	return AccessToken{Value: signed, ExpiresIn: int(m.ttl.Seconds())}, nil
}

func (m *jwtTokenManager) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Pin the algorithm: never trust the alg header, which rules out the
		// "alg: none" and HS/RS confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		// Validate expiry against the injected clock so issuance and parsing
		// agree in tests and production (PRQ-16).
		jwt.WithTimeFunc(m.clock.Now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !parsed.Valid || claims.Subject == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}
