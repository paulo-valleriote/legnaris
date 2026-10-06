package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RefreshTokenGenerator produces opaque, high-entropy refresh tokens. The raw
// value is only ever returned to the caller (and sent to the client); what the
// server persists is its SHA-256 hash.
type RefreshTokenGenerator interface {
	Generate() (string, error)
}

type cryptoRefreshTokenGenerator struct{}

// refreshTokenBytes is the entropy per token. 256 bits matches the security
// margin of the bcrypt-hashed password while staying a short URL-safe string.
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

// HashRefreshToken returns the stable hex SHA-256 of a raw refresh token. Hash
// and lookup both go through this function so a stored token can never be
// compared against anything but its own hash.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
