package idgen

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var ErrInvalidID = errors.New("not a canonical UUID v7")

func New() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuid v7: %w", err)
	}
	return id.String(), nil
}

func Validate(id string) error {
	u, err := uuid.Parse(id)
	switch {
	case err != nil:
		return fmt.Errorf("%w: %w", ErrInvalidID, err)
	case u.Version() != 7 || u.Variant() != uuid.RFC4122:
		return fmt.Errorf("%w: version %d variant %d", ErrInvalidID, u.Version(), u.Variant())
	case id != u.String():
		return fmt.Errorf("%w: %q is not the canonical form", ErrInvalidID, id)
	}
	return nil
}
