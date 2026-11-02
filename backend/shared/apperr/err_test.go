package apperr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewError_StatusMapping covers the HTTP status mapping of every apperr
// code — new codes MUST be added here (and to NewError's switch) so they never
// silently fall back to 500.
func TestNewError_StatusMapping(t *testing.T) {
	tests := []struct {
		code   Code
		status int
	}{
		{CodeInvalidJSON, 400},
		{CodeInvalidRequest, 400},
		{CodeUnknownField, 400},
		{CodeCycleDetected, 400},
		{CodeInvalidDependency, 400},
		{CodeInvalidPath, 400},
		{CodeUnauthorized, 401},
		{CodeForbidden, 403},
		{CodeNotFound, 404},
		{CodeMethodNotAllowed, 405},
		{CodeConflict, 409},
		{CodeSheetLimitReached, 409},
		{CodeRateLimited, 429},
		{CodeEmailUnavailable, 503},
		{CodeNotReady, 503},
		{CodeOTPInvalid, 401},
		{CodeInternal, 500},
	}

	for _, tc := range tests {
		t.Run(string(tc.code), func(t *testing.T) {
			err := NewError(tc.code, "some error", errors.New("detail"))
			assert.Equal(t, tc.status, err.HttpStatus())
		})
	}
}

// TestHttpStatus_UnmappedCodeFallsBackTo500 documents the safety net for
// codes without an explicit mapping in NewError.
func TestHttpStatus_UnmappedCodeFallsBackTo500(t *testing.T) {
	err := &Error{Code: "SOMETHING_NEW", Message: "unmapped"}
	assert.Equal(t, 500, err.HttpStatus())
}

// TestWithRetryAfter pins the wait hint the HTTP boundary renders as
// Retry-After: a fresh error carries none, and the setter returns the same
// error so a producer can chain it (OTP-13, OTP-14).
func TestWithRetryAfter(t *testing.T) {
	plain := NewError(CodeRateLimited, "slow down", nil)
	assert.Zero(t, plain.RetryAfter, "an error without a hint advertises none")

	hinted := NewError(CodeEmailUnavailable, "unavailable", nil).WithRetryAfter(90 * time.Second)
	assert.Equal(t, 90*time.Second, hinted.RetryAfter)
	assert.Equal(t, 503, hinted.HttpStatus())
	assert.Equal(t, CodeEmailUnavailable, hinted.Code)
}
