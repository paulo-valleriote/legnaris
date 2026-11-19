package auth

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/idgen"

	"github.com/golang-jwt/jwt/v5"
)

type AccessToken struct {
	Value     string
	ExpiresIn int
}

type TokenManager interface {
	Issue(userID string) (AccessToken, error)
	Parse(token string) (userID string, err error)
}

type jwtTokenManager struct {
	secret []byte
	ttl    time.Duration
	clock  Clock
}

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
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.clock.Now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	if !parsed.Valid || claims.Subject == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}
