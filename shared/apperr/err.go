package apperr

import "fmt"

type Code string

const (
	CodeInvalidJSON       Code = "INVALID_JSON"
	CodeInvalidRequest    Code = "INVALID_REQUEST"
	CodeUnknownField      Code = "UNKNOWN_FIELD"
	CodeUnauthorized      Code = "UNAUTHORIZED"
	CodeForbidden         Code = "FORBIDDEN"
	CodeNotFound          Code = "NOT_FOUND"
	CodeConflict          Code = "CONFLICT"
	CodeInternal          Code = "INTERNAL_ERROR"
	CodeCycleDetected     Code = "CYCLE_DETECTED"
	CodeInvalidDependency Code = "INVALID_DEPENDENCY"
	CodeInvalidPath       Code = "INVALID_PATH"
)

type Error struct {
	Code       Code
	Message    string
	Err        error
	httpStatus int
}

func NewError(code Code, message string, err error) *Error {
	var httpStatus int
	switch code {
	case CodeInvalidJSON, CodeUnknownField, CodeInvalidRequest,
		CodeCycleDetected, CodeInvalidDependency, CodeInvalidPath:
		httpStatus = 400
	case CodeUnauthorized:
		httpStatus = 401
	case CodeForbidden:
		httpStatus = 403
	case CodeNotFound:
		httpStatus = 404
	case CodeConflict:
		httpStatus = 409
	case CodeInternal:
		httpStatus = 500
	}

	return &Error{
		Code:       code,
		Message:    message,
		Err:        err,
		httpStatus: httpStatus,
	}
}

const LowestErrorState int = 400

func (e *Error) HttpStatus() int {
	if e.httpStatus < LowestErrorState {
		return 500
	}
	return e.httpStatus
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}

	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}
