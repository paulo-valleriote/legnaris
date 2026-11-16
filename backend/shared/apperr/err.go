package apperr

import (
	"fmt"
	"time"
)

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
	// CodeNotReady reports a failed readiness probe: a dependency (the
	// database) is unreachable, so the instance must not receive traffic.
	CodeNotReady Code = "NOT_READY"
	// CodeDisposableEmail refuses a registration whose domain is a throwaway
	// provider or cannot receive mail, before any account is written (LGN-151).
	CodeDisposableEmail Code = "DISPOSABLE_EMAIL"
	// CodeValidation refuses a request whose value fails a versioned contract,
	// such as an LLM consent version other than the current one (LGN-194).
	CodeValidation Code = "VALIDATION"
	// CodeLLMConsentRequired refuses a generation until the player has given
	// consent to the current LLM disclosure (ADR 0008, LGN-194).
	CodeLLMConsentRequired Code = "LLM_CONSENT_REQUIRED"
	// CodePlanUpgradeRequired refuses a generation once the Free lifetime trial
	// is spent or was never granted; the fix is a paid Plan (ADR 0009).
	CodePlanUpgradeRequired Code = "PLAN_UPGRADE_REQUIRED"
	// CodeQuotaExceeded refuses a generation past the paid daily quota. It
	// carries a Retry-After hint up to the next UTC midnight (ADR 0009).
	CodeQuotaExceeded Code = "QUOTA_EXCEEDED"
	// CodeGenerationInProgress refuses a second generation while the player has
	// one queued or running (ADR 0007, LGN-186).
	CodeGenerationInProgress Code = "GENERATION_IN_PROGRESS"
	// CodeProviderUnavailable answers 503 while generation is switched off by
	// the kill-switch, so no request reaches the gateway (ADR 0007, LGN-186).
	CodeProviderUnavailable Code = "PROVIDER_UNAVAILABLE"
	// CodeGenerationCapReached answers 503 once the global daily job cap is spent,
	// with a Retry-After up to the next UTC midnight (LGN-186).
	CodeGenerationCapReached Code = "GENERATION_CAP_REACHED"
)

type Error struct {
	Code       Code
	Message    string
	Err        error
	httpStatus int
	// RetryAfter is the optional hint the HTTP boundary renders as the
	// Retry-After header; a non-positive value means no header.
	RetryAfter time.Duration
	// Details carries machine-readable context for the client, such as the
	// kind and limit of a refused creation. It is rendered as the body's
	// details object and is omitted when empty.
	Details map[string]any
}

// WithDetails attaches client-facing context to the error and returns the same
// error so a producer can set it inline. A nil map means no details.
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// WithRetryAfter attaches the wait hint a rate-limited or unavailable response
// advertises (OTP-13, OTP-14). It returns the same error so a producer can set
// the hint inline.
func (e *Error) WithRetryAfter(retryAfter time.Duration) *Error {
	e.RetryAfter = retryAfter
	return e
}

func NewError(code Code, message string, err error) *Error {
	var httpStatus int
	switch code {
	case CodeInvalidJSON, CodeUnknownField, CodeInvalidRequest,
		CodeCycleDetected, CodeInvalidDependency, CodeInvalidPath, CodeDisposableEmail,
		CodeValidation:
		httpStatus = 400
	case CodeUnauthorized, CodeOTPInvalid:
		httpStatus = 401
	case CodeForbidden, CodeLLMConsentRequired, CodePlanUpgradeRequired:
		httpStatus = 403
	case CodeNotFound:
		httpStatus = 404
	case CodeMethodNotAllowed:
		httpStatus = 405
	case CodeConflict, CodeSheetLimitReached, CodeGenerationInProgress:
		httpStatus = 409
	case CodeRateLimited, CodeQuotaExceeded:
		httpStatus = 429
	case CodeEmailUnavailable, CodeNotReady, CodeProviderUnavailable, CodeGenerationCapReached:
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
