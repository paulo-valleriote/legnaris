package apperr

import (
	"errors"
	"testing"

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
