package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
	"legnaris/backend/shared/logger"
)

type IssueChallengeCommand struct {
	UserID  string
	Email   string
	Purpose Purpose
}

type ChallengeIssued struct {
	ChallengeID string `json:"challenge_id"`
	ExpiresIn   int    `json:"expires_in"`
	ResendAfter int    `json:"resend_after"`
	Delivery    string `json:"-"`
}

const DeliveryDeferred = "deferred"

type VerifyChallengeCommand struct {
	ChallengeID, Code string
	ClientIP          string
}

type ResendChallengeCommand struct{ ChallengeID string }

type ResendResult struct {
	ResendAfter int `json:"resend_after"`
}

const (
	attrChallengeID = "challenge_id"
	attrPurpose     = "purpose"
)

func (s *ServiceImpl) IssueChallenge(ctx context.Context, cmd IssueChallengeCommand) (*ChallengeIssued, error) {
	if err := idgen.Validate(cmd.UserID); err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	now := s.clock.Now()
	policy := s.challenge.Policy

	if err := s.checkAccountBound(ctx, cmd.UserID, now); err != nil {
		return nil, err
	}

	if err := s.checkSendBudget(ctx, now); err != nil {
		return nil, err
	}

	code, err := s.challenge.Generator.Generate()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	challengeID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	intentID, err := idgen.New()
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	challenge := Challenge{
		ID:           challengeID,
		UserID:       cmd.UserID,
		Purpose:      cmd.Purpose,
		CodeHash:     s.challenge.Digest.Digest(cmd.UserID, cmd.Purpose, code),
		ExpiresAt:    policy.Deadline(now, now),
		CeilingAt:    now.Add(policy.Ceiling),
		CreatedAt:    now,
		LastSentAt:   now,
		LastIntentID: intentID,
	}

	if err := s.replaceActiveChallenge(ctx, challenge, now); err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.challenge.Mailer.Record(ctx, intentID, cmd.Email, cmd.Purpose, code, int(policy.Lifetime.Minutes()), now); err != nil {
		if errors.Is(err, ErrSendBudgetExhausted) {
			return nil, s.sendBudgetExhausted(ctx, now)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	s.recordChallengeIssued(ctx, cmd.Purpose, challenge.ID, cmd.Email)
	return &ChallengeIssued{
		ChallengeID: challenge.ID,
		ExpiresIn:   int(policy.Lifetime.Seconds()),
		ResendAfter: int(policy.ResendCooldown.Seconds()),
	}, nil
}

const maxReplaceAttempts = 3

func (s *ServiceImpl) replaceActiveChallenge(ctx context.Context, challenge Challenge, now time.Time) error {
	var err error
	for attempt := 0; attempt < maxReplaceAttempts; attempt++ {
		if err = s.challenge.Repository.CloseActive(ctx, challenge.UserID, challenge.Purpose, now); err != nil {
			return err
		}
		err = s.challenge.Repository.Insert(ctx, challenge)
		if !errors.Is(err, ErrChallengeActive) {
			return err
		}
	}
	return err
}

func (s *ServiceImpl) VerifyChallenge(ctx context.Context, cmd VerifyChallengeCommand) (*Session, error) {
	claimed, err := s.consumeChallenge(ctx, cmd.ChallengeID, cmd.Code, func(row *Challenge) bool {
		return row.Purpose != PurposeDeleteAccount
	})
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	pending := s.unverifiedAccount(ctx, claimed.UserID)

	if err := s.users.MarkEmailVerified(ctx, claimed.UserID, now); err != nil {
		s.log.Error(ctx, "mark email verified failed",
			logger.Attribute{Key: attrUserID, Value: claimed.UserID},
			logger.Attribute{Key: attrChallengeID, Value: claimed.ID},
			logger.Attribute{Key: attrPurpose, Value: string(claimed.Purpose)},
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if pending != nil {
		s.challenge.EmailVerifier.EmailVerified(ctx, pending.ID, pending.Email, cmd.ClientIP)
	}

	session, err := s.signIn(ctx, claimed.UserID)
	if err != nil {
		return nil, err
	}

	s.challenge.Observer.ChallengeVerified(ResultVerified)
	s.log.Info(ctx, "challenge verified",
		logger.Attribute{Key: attrChallengeID, Value: claimed.ID},
		logger.Attribute{Key: attrPurpose, Value: string(claimed.Purpose)},
	)
	return session, nil
}

func (s *ServiceImpl) consumeChallenge(ctx context.Context, challengeID, code string, accepts func(*Challenge) bool) (*Challenge, error) {
	if err := idgen.Validate(challengeID); err != nil {
		return nil, s.invalidCode(ResultUnknown)
	}

	now := s.clock.Now()
	policy := s.challenge.Policy

	claimed, err := s.challenge.Repository.ClaimAttempt(ctx, challengeID, now, policy.MaxAttempts)
	if err != nil {
		if errors.Is(err, ErrChallengeUnusable) {
			return nil, s.classifyFailure(ctx, challengeID, now)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.checkAccountBound(ctx, claimed.UserID, now); err != nil {
		return nil, err
	}

	if !accepts(claimed) {
		return nil, s.invalidCode(ResultInvalid)
	}

	digest := s.challenge.Digest.Digest(claimed.UserID, claimed.Purpose, code)
	if !s.challenge.Digest.Equal(digest, claimed.CodeHash) {
		return nil, s.invalidCode(ResultInvalid)
	}

	consumed, err := s.challenge.Repository.Consume(ctx, challengeID, digest, now)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !consumed {
		return nil, s.invalidCode(ResultConsumed)
	}
	return claimed, nil
}

func (s *ServiceImpl) unverifiedAccount(ctx context.Context, userID string) *user.User {
	if s.challenge.EmailVerifier == nil {
		return nil
	}
	account, err := s.users.FetchByID(ctx, userID)
	if err != nil {
		s.log.Error(ctx, "load account for email verification hook failed",
			logger.Attribute{Key: attrUserID, Value: userID},
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return nil
	}
	if account == nil || account.EmailVerifiedAt != nil {
		return nil
	}
	return account
}

func (s *ServiceImpl) ResendChallenge(ctx context.Context, cmd ResendChallengeCommand) (*ResendResult, error) {
	policy := s.challenge.Policy
	now := s.clock.Now()

	if err := idgen.Validate(cmd.ChallengeID); err != nil {
		return s.suppressedResend(policy), nil
	}

	row, err := s.challenge.Repository.Fetch(ctx, cmd.ChallengeID)
	if err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			return s.suppressedResend(policy), nil
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !row.Usable(now, policy.MaxAttempts) {
		return s.suppressedResend(policy), nil
	}

	if err := s.checkAccountBound(ctx, row.UserID, now); err != nil {
		return nil, err
	}

	if err := s.checkSendBudget(ctx, now); err != nil {
		var appErr *apperr.Error
		if errors.As(err, &appErr) && appErr.Code == apperr.CodeEmailUnavailable {
			s.challenge.Observer.ResendRequested(ResendCapped)
		}
		return nil, err
	}
	if err := s.checkResendBudget(ctx, now); err != nil {
		return nil, err
	}

	extended, err := s.challenge.Repository.Extend(ctx, cmd.ChallengeID, now, policy.ResendCooldown, policy.DeadlineWithin(row.CeilingAt, now))
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !extended {
		return s.suppressedResend(policy), nil
	}

	newIntentID, err := s.challenge.Mailer.Requeue(ctx, row.LastIntentID, now)
	if err != nil {
		if errors.Is(err, ErrSendBudgetExhausted) {
			return nil, s.sendBudgetExhausted(ctx, now)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	s.challenge.Observer.ResendRequested(ResendSent)
	s.log.Info(ctx, "challenge resent",
		logger.Attribute{Key: attrChallengeID, Value: cmd.ChallengeID},
		logger.Attribute{Key: attrPurpose, Value: string(row.Purpose)},
		logger.Attribute{Key: "intent_id", Value: newIntentID},
	)
	return &ResendResult{ResendAfter: int(policy.ResendCooldown.Seconds())}, nil
}

func (s *ServiceImpl) suppressedResend(policy ChallengePolicy) *ResendResult {
	s.challenge.Observer.ResendRequested(ResendSuppressed)
	return &ResendResult{ResendAfter: int(policy.ResendCooldown.Seconds())}
}

func (s *ServiceImpl) invalidCode(result string) error {
	s.challenge.Observer.ChallengeVerified(result)
	return apperr.NewError(apperr.CodeOTPInvalid, MsgOTPInvalid, nil)
}

func (s *ServiceImpl) classifyFailure(ctx context.Context, id string, now time.Time) error {
	row, err := s.challenge.Repository.Fetch(ctx, id)
	if err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			return s.invalidCode(ResultUnknown)
		}
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	return s.invalidCode(row.FailureResult(now, s.challenge.Policy.MaxAttempts))
}

func (s *ServiceImpl) checkAccountBound(ctx context.Context, userID string, now time.Time) error {
	policy := s.challenge.Policy

	count, err := s.challenge.Repository.CountForUserSince(ctx, userID, now.Add(-policy.UserWindow))
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if count >= policy.MaxPerUserWindow {
		return apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil).WithRetryAfter(policy.UserWindow)
	}
	return nil
}

func (s *ServiceImpl) checkSendBudget(ctx context.Context, now time.Time) error {
	policy := s.challenge.Policy

	sent, err := s.challenge.Mailer.SentSince(ctx, policy.DayStart(now))
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if sent >= policy.DailySendCap {
		return s.sendBudgetExhausted(ctx, now)
	}
	return nil
}

func (s *ServiceImpl) checkResendBudget(ctx context.Context, now time.Time) error {
	policy := s.challenge.Policy

	resends, err := s.challenge.Mailer.ResendsSince(ctx, policy.DayStart(now))
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if resends < policy.ResendDayCap() {
		return nil
	}
	s.challenge.Observer.ResendRequested(ResendReserved)
	return s.emailUnavailable(now)
}

func (s *ServiceImpl) sendBudgetExhausted(ctx context.Context, now time.Time) error {
	s.challenge.Observer.SendCapBlocked()
	s.log.Warn(ctx, "daily email send cap reached")
	return s.emailUnavailable(now)
}

func (s *ServiceImpl) emailUnavailable(now time.Time) error {
	policy := s.challenge.Policy

	return apperr.NewError(apperr.CodeEmailUnavailable, MsgEmailUnavailable, nil).
		WithRetryAfter(policy.DayStart(now).Add(24 * time.Hour).Sub(now))
}

func (s *ServiceImpl) recordChallengeIssued(ctx context.Context, purpose Purpose, challengeID, email string) {
	s.challenge.Observer.ChallengeIssued(purpose)
	s.log.Info(ctx, "challenge issued",
		logger.Attribute{Key: attrChallengeID, Value: challengeID},
		logger.Attribute{Key: attrPurpose, Value: string(purpose)},
		logger.Attribute{Key: "to", Value: maskEmail(email)},
	)
}

func maskEmail(address string) string {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return "***"
	}
	return address[:1] + "***" + address[at:]
}
