package auth_test

import (
	"errors"
	"fmt"
	"testing"

	"legnaris/backend/internal/auth"
)

// TestNopChallengeObserver proves the default observer is inert, so an unwired
// caller stays correct (OTP-16).
func TestNopChallengeObserver(t *testing.T) {
	observer := auth.NewNopChallengeObserver()

	observer.ChallengeIssued(otherPurpose)
	observer.ChallengeVerified(auth.ResultVerified)
	observer.ResendRequested(auth.ResendSent)
	observer.SendCapBlocked()
}

// TestErrSendBudgetExhausted proves the cap sentinel is matchable across the
// layer boundary with errors.Is (OTP-13).
func TestErrSendBudgetExhausted(t *testing.T) {
	wrapped := fmt.Errorf("record intention: %w", auth.ErrSendBudgetExhausted)

	if !errors.Is(wrapped, auth.ErrSendBudgetExhausted) {
		t.Fatal("the cap sentinel must survive wrapping so the service can map it")
	}
}
