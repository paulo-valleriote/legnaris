package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type RefreshTokenGenerator interface {
	Generate() (string, error)
}

type cryptoRefreshTokenGenerator struct{}

const refreshTokenBytes = 32

func NewRefreshTokenGenerator() RefreshTokenGenerator {
	return &cryptoRefreshTokenGenerator{}
}

func (g *cryptoRefreshTokenGenerator) Generate() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
