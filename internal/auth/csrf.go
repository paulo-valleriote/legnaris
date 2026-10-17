package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// CSRFTokenManager mints and verifies the stateless, session-bound CSRF token
// the browser echoes in the X-CSRF-Token header. It keeps no storage: the token
// is an HMAC over a fresh nonce and the session's family id (AUTH-09…AUTH-12).
type CSRFTokenManager interface {
	Issue(familyID string) (string, error)
	// Verify returns nil only when the token was minted by this manager for the
	// given family; otherwise it returns ErrCSRFTokenInvalid.
	Verify(token, familyID string) error
}

const (
	csrfNonceBytes = 16
	csrfMACBytes   = sha256.Size
	csrfTokenBytes = csrfNonceBytes + csrfMACBytes

	// csrfKeyDomain separates the CSRF signing key from the JWT signing key.
	csrfKeyDomain = "legnaris/csrf/v1"
	// csrfMessageDomain separates the CSRF MAC input from any other HMAC use.
	csrfMessageDomain = "csrf/v1"
)

type csrfTokenManager struct {
	key []byte
}

// NewCSRFTokenManager derives the CSRF signing key from the JWT secret with
// domain separation, so no new required secret is introduced and a CSRF key
// compromise cannot forge access tokens (AUTH-11; research R2).
func NewCSRFTokenManager(secret []byte) CSRFTokenManager {
	derived := hmac.New(sha256.New, secret)
	derived.Write([]byte(csrfKeyDomain))
	return &csrfTokenManager{key: derived.Sum(nil)}
}

// Issue mints base64url(nonce ‖ HMAC-SHA256(key, "csrf/v1" ‖ 0x00 ‖ familyID ‖
// 0x00 ‖ nonce)) with a fresh 16-byte nonce, so every issue yields a new value
// and a rotation changes the token (AUTH-12).
func (m *csrfTokenManager) Issue(familyID string) (string, error) {
	nonce := make([]byte, csrfNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate csrf nonce: %w", err)
	}

	token := make([]byte, 0, csrfTokenBytes)
	token = append(token, nonce...)
	token = append(token, m.sign(familyID, nonce)...)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// Verify recomputes the MAC over the presented family id and compares it in
// constant time, so a token issued for one session is refused for another
// (AUTH-10, AUTH-11).
func (m *csrfTokenManager) Verify(token, familyID string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != csrfTokenBytes {
		return ErrCSRFTokenInvalid
	}

	nonce := decoded[:csrfNonceBytes]
	mac := decoded[csrfNonceBytes:]
	if !hmac.Equal(mac, m.sign(familyID, nonce)) {
		return ErrCSRFTokenInvalid
	}
	return nil
}

func (m *csrfTokenManager) sign(familyID string, nonce []byte) []byte {
	mac := hmac.New(sha256.New, m.key)
	mac.Write([]byte(csrfMessageDomain))
	mac.Write([]byte{0x00})
	mac.Write([]byte(familyID))
	mac.Write([]byte{0x00})
	mac.Write(nonce)
	return mac.Sum(nil)
}
