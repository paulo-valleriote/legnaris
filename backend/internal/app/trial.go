package app

import (
	"context"
	"errors"
	"fmt"

	"legnaris/backend/internal/auth"
	"legnaris/backend/internal/email/disposable"
	"legnaris/backend/internal/platform/ratelimit"
	"legnaris/backend/internal/trial"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/logger"
)

// trialDomainChecker adapts the email checker to the trial gate's domain port.
// A refusal means the domain is disposable for the gate; any other failure is
// returned, so the gate fails closed and grants nothing.
type trialDomainChecker struct {
	checker auth.EmailChecker
}

// IsDisposable reports whether the checker refuses email. The auth service wraps
// a refusal as CodeDisposableEmail, while the disposable checker returns its own
// sentinel, so both count as a disposable verdict.
func (c trialDomainChecker) IsDisposable(ctx context.Context, email string) (bool, error) {
	err := c.checker.Check(ctx, email)
	if err == nil {
		return false, nil
	}
	if isDisposableRefusal(err) {
		return true, nil
	}
	return false, fmt.Errorf("check email domain: %w", err)
}

func isDisposableRefusal(err error) bool {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Code == apperr.CodeDisposableEmail {
		return true
	}
	return errors.Is(err, disposable.ErrRefused)
}

// trialVerifier runs the trial gate on each account's first email verification.
// Gate errors are logged here, so a failing port never fails the verification.
type trialVerifier struct {
	gate *trial.Gate
	log  logger.StructuredLogger
}

// EmailVerified evaluates the allowance for the verified account. Every outcome
// other than a granted allowance leaves the account with none, which is not an
// error.
func (v trialVerifier) EmailVerified(ctx context.Context, userID, email, clientIP string) {
	decision, err := v.gate.Evaluate(ctx, trial.Candidate{
		UserID:        userID,
		Email:         email,
		EmailVerified: true,
		ClientIP:      clientIP,
	})
	if err != nil {
		v.log.Error(ctx, "trial allowance evaluation failed",
			logger.Attribute{Key: "user_id", Value: userID},
			logger.Attribute{Key: logErrorKey, Value: err.Error()},
		)
		return
	}
	v.log.Info(ctx, "trial allowance evaluated",
		logger.Attribute{Key: "user_id", Value: userID},
		logger.Attribute{Key: "decision", Value: string(decision)},
	)
}

// newTrialVerifier wires the trial gate for the verification hook. The domain
// check always runs, whether or not registration checks the domain, so a
// disposable account never receives the allowance.
func newTrialVerifier(granter trial.Granter, checker auth.EmailChecker, ips ratelimit.Allower, log logger.StructuredLogger) auth.EmailVerifier {
	gate := trial.NewGate(granter, trialDomainChecker{checker: checker}, ips)
	return trialVerifier{gate: gate, log: log}
}
