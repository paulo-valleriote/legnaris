// Package idgen owns the repository-wide contract for server-generated entity
// identifiers: every ID the server creates is a canonical lowercase UUID v7
// string (RFC 9562). Generate with New; screen inbound IDs with Validate at
// the service boundary, before any repository call.
package idgen

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrInvalidID reports an identifier that is not a canonical lowercase UUID
// v7 — the only shape New ever produces. It stays a plain sentinel (this
// package never imports shared/apperr, per layering rules) so services can
// translate it into their typed domain error at the boundary.
var ErrInvalidID = errors.New("not a canonical UUID v7")

// New returns a canonical lowercase UUID v7 string. The 48-bit timestamp
// prefix makes lexicographic order equal creation order (the first 8 bytes are
// strictly increasing, so the random tail never decides a comparison), which
// keeps _id indexes append-mostly and lets future listings sort by _id.
// Monotonicity is per process (12-bit sequence); across instances it depends
// on clock sync. The only failure mode is crypto/rand exhaustion.
func New() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuid v7: %w", err)
	}
	return id.String(), nil
}

// Validate reports whether id is exactly what New produces: a 36-char
// hyphenated lowercase UUID v7 with the RFC 4122 variant. The final
// id != u.String() comparison rejects every alternate encoding — uppercase,
// urn:uuid:, braces, missing hyphens — in one step, because none of those
// spellings can ever match the document stored in the database.
func Validate(id string) error {
	u, err := uuid.Parse(id)
	switch {
	case err != nil:
		return fmt.Errorf("%w: %v", ErrInvalidID, err)
	case u.Version() != 7 || u.Variant() != uuid.RFC4122:
		return fmt.Errorf("%w: version %d variant %d", ErrInvalidID, u.Version(), u.Variant())
	case id != u.String():
		return fmt.Errorf("%w: %q is not the canonical form", ErrInvalidID, id)
	}
	return nil
}
