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

var echoedStatusCodes = map[int]apperr.Code{
	http.StatusBadRequest:       apperr.CodeInvalidRequest,
	http.StatusUnauthorized:     apperr.CodeUnauthorized,
	http.StatusForbidden:        apperr.CodeForbidden,
	http.StatusNotFound:         apperr.CodeNotFound,
	http.StatusMethodNotAllowed: apperr.CodeMethodNotAllowed,
}

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

func templateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, template.ErrTemplateIDNotProvided) || errors.Is(err, template.ErrTemplateIDInvalid) {
		return apperr.NewError(apperr.CodeInvalidRequest, err.Error(), nil)
	}
	if errors.Is(err, template.ErrTemplateNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "template not found", err)
	}
	return err
}
