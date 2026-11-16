package user

import (
	"errors"
	"net/mail"
	"strings"
)

const MaxEmailLength = 254

var ErrInvalidEmail = errors.New("email is invalid")

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
