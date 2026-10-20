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
	CodeMethodNotAllowed  Code = "METHOD_NOT_ALLOWED"
	CodeConflict          Code = "CONFLICT"
	CodeSheetLimitReached Code = "SHEET_LIMIT_REACHED"
	CodeRateLimited       Code = "RATE_LIMITED"
	CodeInternal          Code = "INTERNAL_ERROR"
	CodeCycleDetected     Code = "CYCLE_DETECTED"
	CodeInvalidDependency Code = "INVALID_DEPENDENCY"
	CodeInvalidPath       Code = "INVALID_PATH"
	// CodeOTPInvalid is the one uniform outcome of every verification failure
	// (unknown, expired, consumed, wrong or exhausted), so no case is
	// distinguishable from another (OTP-08).
	CodeOTPInvalid Code = "OTP_INVALID"
	// CodeEmailUnavailable is the daily send cap being reached: no intention is
	// recorded and the caller is told to retry later (OTP-13).
	CodeEmailUnavailable Code = "EMAIL_UNAVAILABLE"
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
	case CodeUnauthorized, CodeOTPInvalid:
		httpStatus = 401
	case CodeForbidden:
		httpStatus = 403
	case CodeNotFound:
		httpStatus = 404
	case CodeMethodNotAllowed:
		httpStatus = 405
	case CodeConflict, CodeSheetLimitReached:
		httpStatus = 409
	case CodeRateLimited:
		httpStatus = 429
	case CodeEmailUnavailable:
		httpStatus = 503
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

const httpStatusFloor int = 400

func (e *Error) HttpStatus() int {
	if e.httpStatus < httpStatusFloor {
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
