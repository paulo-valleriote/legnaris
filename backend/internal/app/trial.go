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

type trialDomainChecker struct {
	checker auth.EmailChecker
}

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

type trialVerifier struct {
	gate *trial.Gate
	log  logger.StructuredLogger
}

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

func newTrialVerifier(granter trial.Granter, checker auth.EmailChecker, ips ratelimit.Allower, log logger.StructuredLogger) auth.EmailVerifier {
	gate := trial.NewGate(granter, trialDomainChecker{checker: checker}, ips)
	return trialVerifier{gate: gate, log: log}
}
