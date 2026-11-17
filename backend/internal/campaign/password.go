package campaign

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime      uint32 = 2
	argonMemory    uint32 = 19456
	argonThreads   uint8  = 1
	argonKeyLen    uint32 = 32
	argonSaltLen          = 16
	argonPartCount        = 6
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encoded, password string) bool
	DummyHash() string
}

type Argon2Hasher struct {
	dummy string
}

func NewArgon2Hasher() (*Argon2Hasher, error) {
	h := &Argon2Hasher{}
	dummy, err := h.Hash("dummy-password-for-timing-parity")
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	return h, nil
}

func (h *Argon2Hasher) DummyHash() string {
	return h.dummy
}

func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (h *Argon2Hasher) Verify(encoded, password string) bool {
	params, salt, want, ok := parseArgon2(encoded)
	if !ok {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

type argon2Params struct {
	time    uint32
	memory  uint32
	threads uint8
}

func parseArgon2(encoded string) (argon2Params, []byte, []byte, bool) {
	parts := strings.Split(encoded, "$")
	if len(parts) != argonPartCount || parts[0] != "" || parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argon2Params{}, nil, nil, false
	}
	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return argon2Params{}, nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return argon2Params{}, nil, nil, false
	}
	return p, salt, key, true
}
