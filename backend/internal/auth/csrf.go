package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type CSRFTokenManager interface {
	Issue(familyID string) (string, error)
	Verify(token, familyID string) error
}

const (
	csrfNonceBytes = 16
	csrfMACBytes   = sha256.Size
	csrfTokenBytes = csrfNonceBytes + csrfMACBytes

	csrfKeyDomain     = "legnaris/csrf/v1"
	csrfMessageDomain = "csrf/v1"
)

type csrfTokenManager struct {
	key []byte
}

func NewCSRFTokenManager(secret []byte) CSRFTokenManager {
	derived := hmac.New(sha256.New, secret)
	derived.Write([]byte(csrfKeyDomain))
	return &csrfTokenManager{key: derived.Sum(nil)}
}

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
