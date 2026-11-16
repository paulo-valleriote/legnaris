package auth_test

import (
	"errors"
	"fmt"
	"testing"

	"legnaris/backend/internal/auth"
)

func TestNopChallengeObserver(t *testing.T) {
	observer := auth.NewNopChallengeObserver()

	observer.ChallengeIssued(otherPurpose)
	observer.ChallengeVerified(auth.ResultVerified)
	observer.ResendRequested(auth.ResendSent)
	observer.SendCapBlocked()
}

func TestErrSendBudgetExhausted(t *testing.T) {
	wrapped := fmt.Errorf("record intention: %w", auth.ErrSendBudgetExhausted)

	if !errors.Is(wrapped, auth.ErrSendBudgetExhausted) {
		t.Fatal("the cap sentinel must survive wrapping so the service can map it")
	}
}
