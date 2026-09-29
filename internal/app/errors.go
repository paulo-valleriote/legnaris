package app

import (
	"errors"

	"legnaris/internal/sheet/template"
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

// templateError translates typed template-package errors into apperr codes at
// the controller boundary, mirroring decodeError: the template package
// declares typed errors instead of importing shared/apperr, so the HTTP mapping
// belongs here — this translates the outcome of the service's validation, it
// does not repeat it (LGN-66).
func templateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, template.ErrTemplateIDNotProvided) {
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	}
	return err
}
