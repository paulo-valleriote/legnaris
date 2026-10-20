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
	// DummyHash returns a bcrypt digest carrying this hasher's own work
	// factor. It is built from a secret nobody holds, so no submitted
	// password matches it; the sign-in of an address that does not exist
	// compares against it, so both credential failures cost the same work
	// whatever the configured cost is (TFA-05).
	DummyHash() string
}

type bcryptHasher struct {
	cost  int
	dummy string
}

// minBcryptCost is the lowest accepted work factor. docs/TDD.md mandates
// bcrypt cost >= 12, so a weaker value fails at construction (boot) instead
// of silently hashing with less work than the documented contract.
const minBcryptCost = 12

// dummyHashTemplate is a bcrypt digest whose stored part belongs to no
// plaintext anyone holds. Only its work factor matters to the sign-in of an
// address that does not exist: the submitted password is compared against it
// to spend the same password work as the sign-in of a known one, and the
// result is discarded. The digest therefore never grants access, whatever the
// caller submits.
const dummyHashTemplate = "$2a$12$1oc7Q1ZE7F4BVs/eABCG9ezOuIyC2VSqFknSk3Ao0.rtVTfCnse7C"

// bcryptCostPrefixLen is the length of the "$2a$<cost>$" prefix of a bcrypt
// digest: the algorithm, the two-digit work factor and their separators.
const bcryptCostPrefixLen = len("$2a$12$")

// NewPasswordHasher builds a bcrypt-backed hasher. The cost is validated
// against the accepted range [minBcryptCost, bcrypt.MaxCost]; the production
// default (cost 12) comes from AUTH_BCRYPT_COST.
//
// The hasher also derives its dummy hash at the same cost, so the sign-in of
// an address that does not exist spends the same password work as the sign-in
// of a known one. A digest pinned to a fixed cost would re-open the
// account-existence oracle the comparison exists to close, because bcrypt
// spends the rounds encoded in the digest it is given, not the ones the
// hasher was configured with (TFA-05).
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

// dummyHashAtCost returns the digest the unknown-address sign-in compares
// against, carrying the given work factor.
//
// The digest is the template with its cost field rewritten, because bcrypt
// spends the rounds encoded in the digest it is handed: a comparison against
// this digest therefore costs exactly the configured number of rounds. The
// rewrite also keeps the construction free, where generating a digest at the
// configured cost would charge one bcrypt hash to every boot and to every
// suite that wires the application (TFA-05).
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

// DummyHash returns the digest the unknown-address sign-in compares against.
// It is derived at construction, so its cost always matches the hasher's.
func (h *bcryptHasher) DummyHash() string {
	return h.dummy
}
