package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const minCodeSecretLength = 32

const codeLength = 6

const codeVersion = "legnaris-otp-v1"

const codeDigitSpace = 1_000_000

var ErrCodeSecretWeak = errors.New("email code secret must be at least 32 bytes")

type CodeDigest struct {
	key []byte
}

func NewCodeDigest(secret string) (*CodeDigest, error) {
	if strings.TrimSpace(secret) == "" || len(secret) < minCodeSecretLength {
		return nil, ErrCodeSecretWeak
	}
	return &CodeDigest{key: []byte(secret)}, nil
}

func (d *CodeDigest) Digest(userID string, purpose Purpose, code string) string {
	mac := hmac.New(sha256.New, d.key)
	mac.Write([]byte(codeVersion))
	mac.Write([]byte{0})
	mac.Write([]byte(userID))
	mac.Write([]byte{0})
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *CodeDigest) Equal(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

type CodeGenerator interface {
	Generate() (string, error)
}

type cryptoCodeGenerator struct{}

func NewCodeGenerator() CodeGenerator {
	return &cryptoCodeGenerator{}
}

func (g *cryptoCodeGenerator) Generate() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeDigitSpace))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%0*d", codeLength, n.Int64()), nil
}
