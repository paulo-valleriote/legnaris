package apperr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
		{CodePlanUpgradeRequired, 403},
		{CodeQuotaExceeded, 429},
		{CodeNotFound, 404},
		{CodeNoSubscription, 404},
		{CodeRefundNotEligible, 409},
		{CodeMethodNotAllowed, 405},
		{CodeConflict, 409},
		{CodeDisposableEmail, 400},
		{CodeSheetLimitReached, 409},
		{CodeRateLimited, 429},
		{CodeEmailUnavailable, 503},
		{CodeNotReady, 503},
		{CodeOTPInvalid, 401},
		{CodeInternal, 500},
		{CodeValidation, 400},
		{CodeLLMConsentRequired, 403},
		{CodeGenerationInProgress, 409},
		{CodeProviderUnavailable, 503},
		{CodeGenerationCapReached, 503},
		{CodeAlreadySubscribed, 409},
		{CodeBillingUnavailable, 503},
		{CodeInvalidSignature, 400},
		{CodeCampaignNotFound, 404},
		{CodeCampaignFull, 409},
		{CodeCampaignForbidden, 403},
		{CodeInvalidInvite, 404},
		{CodeTemplateMismatch, 422},
		{CodeSheetAlreadyLinked, 409},
		{CodePlanLimitReached, 403},
	}

	for _, tc := range tests {
		t.Run(string(tc.code), func(t *testing.T) {
			err := NewError(tc.code, "some error", errors.New("detail"))
			assert.Equal(t, tc.status, err.HttpStatus())
		})
	}
}

func TestHttpStatus_UnmappedCodeFallsBackTo500(t *testing.T) {
	err := &Error{Code: "SOMETHING_NEW", Message: "unmapped"}
	assert.Equal(t, 500, err.HttpStatus())
}

func TestWithRetryAfter(t *testing.T) {
	plain := NewError(CodeRateLimited, "slow down", nil)
	assert.Zero(t, plain.RetryAfter, "an error without a hint advertises none")

	hinted := NewError(CodeEmailUnavailable, "unavailable", nil).WithRetryAfter(90 * time.Second)
	assert.Equal(t, 90*time.Second, hinted.RetryAfter)
	assert.Equal(t, 503, hinted.HttpStatus())
	assert.Equal(t, CodeEmailUnavailable, hinted.Code)
}

func TestWithDetails(t *testing.T) {
	tests := []struct {
		name    string
		details map[string]any
		want    map[string]any
	}{
		{name: "Happy path: details are attached", details: map[string]any{"kind": "npc", "limit": 2}, want: map[string]any{"kind": "npc", "limit": 2}},
		{name: "Edge: nil map leaves no details", details: nil, want: nil},
		{name: "Edge: empty map is kept as given", details: map[string]any{}, want: map[string]any{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appErr := NewError(CodeSheetLimitReached, "limit", nil)
			assert.Nil(t, appErr.Details, "a fresh error carries no details")
			got := appErr.WithDetails(tc.details)
			assert.Same(t, appErr, got, "the setter returns the same error")
			assert.Equal(t, tc.want, got.Details)
		})
	}
}
