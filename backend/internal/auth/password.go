package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher hashes and verifies passwords. It is an interface so the auth
// service can be tested without paying bcrypt's cost on every case.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type bcryptHasher struct {
	cost int
}

// minBcryptCost is the lowest accepted work factor. docs/TDD.md mandates
// bcrypt cost >= 12, so a weaker value fails at construction (boot) instead
// of silently hashing with less work than the documented contract.
const minBcryptCost = 12

// NewPasswordHasher builds a bcrypt-backed hasher. The cost is validated
// against the accepted range [minBcryptCost, bcrypt.MaxCost]; the production
// default (cost 12) comes from AUTH_BCRYPT_COST.
func NewPasswordHasher(cost int) (PasswordHasher, error) {
	if cost < minBcryptCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("%w: %d", ErrInvalidBcryptCost, cost)
	}
	return &bcryptHasher{cost: cost}, nil
}

func (h *bcryptHasher) Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashed), nil
}

func (h *bcryptHasher) Compare(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordMismatch, err)
	}
	return nil
}
