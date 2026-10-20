package app

import (
	"errors"
	"net/http"
	"strings"

	"legnaris/backend/internal/sheet/template"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/json"

	"github.com/labstack/echo/v5"
)

// echoedStatusCodes maps the client-status errors Echo raises before a handler
// runs onto the apperr vocabulary. An unmatched route is a 404 and a known path
// reached with the wrong method is a 405, and neither is an apperr, so without
// this translation they reached the 500 fallback and answered INTERNAL_ERROR.
// Statuses without an apperr counterpart stay on that fallback.
var echoedStatusCodes = map[int]apperr.Code{
	http.StatusBadRequest:       apperr.CodeInvalidRequest,
	http.StatusUnauthorized:     apperr.CodeUnauthorized,
	http.StatusForbidden:        apperr.CodeForbidden,
	http.StatusNotFound:         apperr.CodeNotFound,
	http.StatusMethodNotAllowed: apperr.CodeMethodNotAllowed,
}

// asAppErr normalises a handler error into the apperr vocabulary: an apperr
// passes through untouched, an error that only carries an HTTP status is
// translated to its matching code, and anything else is the generic 500.
func asAppErr(err error) *apperr.Error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}

	status := echo.StatusCode(err)
	code, ok := echoedStatusCodes[status]
	if !ok {
		return apperr.NewError(apperr.CodeInternal, "internal server error", err)
	}

	return apperr.NewError(code, strings.ToLower(http.StatusText(status)), err)
}

// decodeError translates typed backend/shared/json decoding errors into apperr codes
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
// declares typed errors instead of importing backend/shared/apperr, so the HTTP mapping
// belongs here — this translates the outcome of the service's validation, it
// does not repeat it (LGN-66, LGN-86).
func templateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, template.ErrTemplateIDNotProvided) || errors.Is(err, template.ErrTemplateIDInvalid) {
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	}
	return err
}
