package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
	DummyHash() string
}

type bcryptHasher struct {
	cost  int
	dummy string
}

const minBcryptCost = 12

const dummyHashTemplate = "$2a$12$1oc7Q1ZE7F4BVs/eABCG9ezOuIyC2VSqFknSk3Ao0.rtVTfCnse7C"

const bcryptCostPrefixLen = len("$2a$12$")

func NewPasswordHasher(cost int) (PasswordHasher, error) {
	if cost < minBcryptCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("%w: %d", ErrInvalidBcryptCost, cost)
	}

	dummy, err := dummyHashAtCost(cost)
	if err != nil {
		return nil, err
	}

	return &bcryptHasher{cost: cost, dummy: dummy}, nil
}

func dummyHashAtCost(cost int) (string, error) {
	if cost < minBcryptCost || cost > bcrypt.MaxCost {
		return "", fmt.Errorf("%w: %d", ErrInvalidBcryptCost, cost)
	}
	return fmt.Sprintf("$2a$%02d$", cost) + dummyHashTemplate[bcryptCostPrefixLen:], nil
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

func (h *bcryptHasher) DummyHash() string {
	return h.dummy
}
