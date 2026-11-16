package apperr

import (
	"fmt"
	"time"
)

type Code string

const (
	CodeInvalidJSON          Code = "INVALID_JSON"
	CodeInvalidRequest       Code = "INVALID_REQUEST"
	CodeUnknownField         Code = "UNKNOWN_FIELD"
	CodeUnauthorized         Code = "UNAUTHORIZED"
	CodeForbidden            Code = "FORBIDDEN"
	CodeNotFound             Code = "NOT_FOUND"
	CodeMethodNotAllowed     Code = "METHOD_NOT_ALLOWED"
	CodeConflict             Code = "CONFLICT"
	CodeSheetLimitReached    Code = "SHEET_LIMIT_REACHED"
	CodeRateLimited          Code = "RATE_LIMITED"
	CodeInternal             Code = "INTERNAL_ERROR"
	CodeCycleDetected        Code = "CYCLE_DETECTED"
	CodeInvalidDependency    Code = "INVALID_DEPENDENCY"
	CodeInvalidPath          Code = "INVALID_PATH"
	CodeOTPInvalid           Code = "OTP_INVALID"
	CodeEmailUnavailable     Code = "EMAIL_UNAVAILABLE"
	CodeNotReady             Code = "NOT_READY"
	CodeDisposableEmail      Code = "DISPOSABLE_EMAIL"
	CodeValidation           Code = "VALIDATION"
	CodeLLMConsentRequired   Code = "LLM_CONSENT_REQUIRED"
	CodePlanUpgradeRequired  Code = "PLAN_UPGRADE_REQUIRED"
	CodeQuotaExceeded        Code = "QUOTA_EXCEEDED"
	CodeGenerationInProgress Code = "GENERATION_IN_PROGRESS"
	CodeProviderUnavailable  Code = "PROVIDER_UNAVAILABLE"
	CodeGenerationCapReached Code = "GENERATION_CAP_REACHED"
	CodeAlreadySubscribed    Code = "ALREADY_SUBSCRIBED"
	CodeBillingUnavailable   Code = "BILLING_UNAVAILABLE"
	CodeInvalidSignature     Code = "INVALID_SIGNATURE"
	CodeNoSubscription       Code = "NO_SUBSCRIPTION"
)

type Error struct {
	Code       Code
	Message    string
	Err        error
	httpStatus int
	RetryAfter time.Duration
	Details    map[string]any
}

func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

func (e *Error) WithRetryAfter(retryAfter time.Duration) *Error {
	e.RetryAfter = retryAfter
	return e
}

func NewError(code Code, message string, err error) *Error {
	var httpStatus int
	switch code {
	case CodeInvalidJSON, CodeUnknownField, CodeInvalidRequest, CodeInvalidSignature,
		CodeCycleDetected, CodeInvalidDependency, CodeInvalidPath, CodeDisposableEmail,
		CodeValidation:
		httpStatus = 400
	case CodeUnauthorized, CodeOTPInvalid:
		httpStatus = 401
	case CodeForbidden, CodeLLMConsentRequired, CodePlanUpgradeRequired:
		httpStatus = 403
	case CodeNotFound, CodeNoSubscription:
		httpStatus = 404
	case CodeMethodNotAllowed:
		httpStatus = 405
	case CodeConflict, CodeSheetLimitReached, CodeGenerationInProgress, CodeAlreadySubscribed:
		httpStatus = 409
	case CodeRateLimited, CodeQuotaExceeded:
		httpStatus = 429
	case CodeEmailUnavailable, CodeNotReady, CodeProviderUnavailable, CodeGenerationCapReached, CodeBillingUnavailable:
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
