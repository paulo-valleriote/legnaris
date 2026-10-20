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

// minCodeSecretLength is the shortest accepted digest secret. A shorter or blank
// key would silently weaken every code, so it fails fast at boot (OTP-18).
const minCodeSecretLength = 32

// codeLength is the fixed number of decimal digits a code has (OTP-02).
const codeLength = 6

// codeVersion tags the digest algorithm so a future change is distinguishable,
// and NUL separators make the account/purpose binding unambiguous even if a
// field ever contains a separator character (OTP-03).
const codeVersion = "legnaris-otp-v1"

// codeDigitSpace is the number of uniform six-digit values, 000000…999999.
const codeDigitSpace = 1_000_000

// ErrCodeSecretWeak reports an absent, blank, or sub-32-byte digest secret. It is
// a plain sentinel: the auth domain maps it to a typed boot error at its
// boundary, exactly like the other lower-layer sentinels in this package.
var ErrCodeSecretWeak = errors.New("email code secret must be at least 32 bytes")

// CodeDigest turns a code into the value the challenge stores. It keys every
// digest with the required EMAIL_CODE_SECRET, which is distinct from the
// session-signing secret (F-09, OTP-03, OTP-21).
type CodeDigest struct {
	key []byte
}

// NewCodeDigest validates the secret once at boot and returns the digest keyed
// by it. An absent, blank (whitespace-only), or sub-32-byte value is
// ErrCodeSecretWeak (OTP-18).
func NewCodeDigest(secret string) (*CodeDigest, error) {
	if strings.TrimSpace(secret) == "" || len(secret) < minCodeSecretLength {
		return nil, ErrCodeSecretWeak
	}
	return &CodeDigest{key: []byte(secret)}, nil
}

// Digest returns the lowercase-hex HMAC-SHA256 of the version tag, the account,
// the purpose and the code, NUL-separated. The binding makes a code issued for
// one account or purpose unusable for another (OTP-03).
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

// Equal compares two digests in constant time, so a wrong code costs the same
// time as a right one regardless of the shared prefix. It is never called with
// the raw code (OTP-03).
func (d *CodeDigest) Equal(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// CodeGenerator produces one six-digit code from a cryptographically secure
// source (OTP-02).
type CodeGenerator interface {
	Generate() (string, error)
}

type cryptoCodeGenerator struct{}

// NewCodeGenerator returns the crypto/rand-backed generator. math/rand is never
// used (the plan's §3.4 invariant).
func NewCodeGenerator() CodeGenerator {
	return &cryptoCodeGenerator{}
}

// Generate returns exactly six decimal characters uniformly over 000000…999999,
// preserving leading zeros as part of the string value.
func (g *cryptoCodeGenerator) Generate() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeDigitSpace))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%0*d", codeLength, n.Int64()), nil
}
