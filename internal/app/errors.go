package app

import (
	"errors"

	"legnaris/shared/apperr"
	"legnaris/shared/json"
)

// decodeError translates typed shared/json decoding errors into apperr codes
// at the controller boundary (controllers are allowed to use apperr; the
// decoding helper itself is not).
func decodeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, json.ErrInvalidJSON):
		return apperr.NewError(apperr.CodeInvalidJSON, "request body contains invalid JSON", err)
	case errors.Is(err, json.ErrUnknownField):
		return apperr.NewError(apperr.CodeUnknownField, "request body contains an unknown field", err)
	}
	return err
}
