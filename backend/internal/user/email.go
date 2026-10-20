package user

import (
	"errors"
	"net/mail"
	"strings"
)

// MaxEmailLength is the longest accepted canonical address in bytes, matching
// RFC 5321's maximum path length.
const MaxEmailLength = 254

// ErrInvalidEmail reports an address that cannot be parsed or whose canonical
// form exceeds MaxEmailLength. Services map it to their invalid-request outcome.
var ErrInvalidEmail = errors.New("email is invalid")

// CanonicalEmail returns the bare canonical address for a supplied input:
// display name and surrounding whitespace removed, lowercase, and capped at
// MaxEmailLength bytes. Lowercasing and trimming are the only transformations,
// so dots and +tags in the local part are preserved. The same rule serves
// creation, sign-in, the account rate-limit key and the legacy migration
// (PRQ-08, PRQ-09, PRQ-10, PRQ-11).
func CanonicalEmail(raw string) (string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", ErrInvalidEmail
	}

	canonical := strings.ToLower(strings.TrimSpace(parsed.Address))
	if len(canonical) > MaxEmailLength {
		return "", ErrInvalidEmail
	}
	return canonical, nil
}
