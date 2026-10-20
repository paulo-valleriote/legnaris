package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"legnaris/shared/apperr"
	"legnaris/shared/idgen"
	"legnaris/shared/logger"
)

// Challenge command and result shapes (contracts/endpoints.contract.md).
type IssueChallengeCommand struct {
	// UserID is resolved by the caller after its credentials were verified;
	// it is never derived from an address alone (OTP-09).
	UserID  string
	Email   string
	Purpose Purpose
}

// ChallengeIssued is the issuance result: the identity the client carries back,
// the claimed lifetime and the resend window (OTP-10).
type ChallengeIssued struct {
	ChallengeID string `json:"challenge_id"`
	ExpiresIn   int    `json:"expires_in"`
	ResendAfter int    `json:"resend_after"`
}

// VerifyChallengeCommand carries only the challenge identity and the submitted
// code; the purpose is resolved server-side (OTP-09).
type VerifyChallengeCommand struct{ ChallengeID, Code string }

// ResendChallengeCommand carries only the challenge identity (OTP-11).
type ResendChallengeCommand struct{ ChallengeID string }

// ResendResult is the uniform resend answer: the retry window the client needs
// to render its countdown (OTP-10).
type ResendResult struct {
	ResendAfter int `json:"resend_after"`
}

// Structured-log field keys shared by the challenge paths (OTP-15).
const (
	attrChallengeID = "challenge_id"
	attrPurpose     = "purpose"
)

// IssueChallenge mints a challenge for an account whose credentials were
// already verified: it enforces the daily send cap, generates the code and its
// keyed digest, closes any previous active challenge, inserts the new record
// and records the send intention last, so a crash between the two writes never
// loses the code (OTP-01, OTP-02, OTP-12, OTP-13).
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

	if err := s.challenge.Repository.CloseActive(ctx, cmd.UserID, cmd.Purpose, now); err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if err := s.challenge.Repository.Insert(ctx, challenge); err != nil {
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

// VerifyChallenge consumes a challenge exactly once and establishes a session
// exactly as sign-in does, recording the account's verification timestamp when
// it was absent. Every failure answers the one uniform typed outcome, so no
// case is distinguishable by status, body or timing class (OTP-04…OTP-08).
func (s *ServiceImpl) VerifyChallenge(ctx context.Context, cmd VerifyChallengeCommand) (*Session, error) {
	if err := idgen.Validate(cmd.ChallengeID); err != nil {
		return nil, s.invalidCode(ResultUnknown)
	}

	now := s.clock.Now()
	policy := s.challenge.Policy

	claimed, err := s.challenge.Repository.ClaimAttempt(ctx, cmd.ChallengeID, now, policy.MaxAttempts)
	if err != nil {
		if errors.Is(err, ErrChallengeUnusable) {
			return nil, s.classifyFailure(ctx, cmd.ChallengeID, now)
		}
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}

	if err := s.checkAccountBound(ctx, claimed.UserID, now); err != nil {
		return nil, err
	}

	digest := s.challenge.Digest.Digest(claimed.UserID, claimed.Purpose, cmd.Code)
	if !s.challenge.Digest.Equal(digest, claimed.CodeHash) {
		return nil, s.invalidCode(ResultInvalid)
	}

	consumed, err := s.challenge.Repository.Consume(ctx, cmd.ChallengeID, digest, now)
	if err != nil {
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
	}
	if !consumed {
		return nil, s.invalidCode(ResultConsumed)
	}

	// The timestamp is recorded before the session is minted, so a session is
	// only ever issued for a verified account (R12, OTP-07).
	if err := s.users.MarkEmailVerified(ctx, claimed.UserID, now); err != nil {
		s.log.Error(ctx, "mark email verified failed",
			logger.Attribute{Key: attrUserID, Value: claimed.UserID},
			logger.Attribute{Key: attrChallengeID, Value: claimed.ID},
			logger.Attribute{Key: attrPurpose, Value: string(claimed.Purpose)},
			logger.Attribute{Key: "error", Value: err.Error()},
		)
		return nil, apperr.NewError(apperr.CodeInternal, MsgInternal, err)
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

// ResendChallenge answers a repeat request for a challenge's code. It is
// idempotent for the client: it answers the accepted outcome whether or not it
// recorded a new send, never rotates the code, never changes the identity and
// never moves the deadline past the challenge's absolute ceiling. An unknown,
// malformed, consumed or expired identity answers identically and changes
// nothing (OTP-10, OTP-11).
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

// suppressedResend is the uniform resend answer for a case that recorded no
// send: the same status and the same body as an accepted resend, so no case is
// distinguishable (OTP-11). Nothing is logged per suppression, because the
// identity is attacker-controlled (R10).
func (s *ServiceImpl) suppressedResend(policy ChallengePolicy) *ResendResult {
	s.challenge.Observer.ResendRequested(ResendSuppressed)
	return &ResendResult{ResendAfter: int(policy.ResendCooldown.Seconds())}
}

// invalidCode is the single producer of the uniform verification failure: one
// code, one message, one status for every case, with the classification
// exported only to the observer (OTP-08, OTP-16).
func (s *ServiceImpl) invalidCode(result string) error {
	s.challenge.Observer.ChallengeVerified(result)
	return apperr.NewError(apperr.CodeOTPInvalid, MsgOTPInvalid, nil)
}

// classifyFailure reads the stored row to label why a verification could not
// proceed, for metrics and logs only; the response is the uniform failure
// whichever label is produced (R4, OTP-16).
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

// checkAccountBound refuses the request once the account's challenges recorded
// inside the configured window have reached the documented cap. The count is a
// store fact, so a restart or a second instance cannot reset the budget, and
// the refusal is the existing rate-limit outcome with a retry hint (F-07,
// OTP-14).
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

// checkSendBudget refuses a new send once the day's recorded intentions have
// reached the configured cap, before any intention is written (OTP-13).
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

// sendBudgetExhausted is the typed unavailable outcome with a retry hint that
// points at the next day's window; reaching the cap reveals no account state
// (OTP-13, R6).
func (s *ServiceImpl) sendBudgetExhausted(ctx context.Context, now time.Time) error {
	policy := s.challenge.Policy

	s.challenge.Observer.SendCapBlocked()
	s.log.Warn(ctx, "daily email send cap reached")

	return apperr.NewError(apperr.CodeEmailUnavailable, MsgEmailUnavailable, nil).
		WithRetryAfter(policy.DayStart(now).Add(24 * time.Hour).Sub(now))
}

// recordChallengeIssued records issuance and logs it by identity and purpose
// with the recipient masked; the code never appears (OTP-15, OTP-16).
func (s *ServiceImpl) recordChallengeIssued(ctx context.Context, purpose Purpose, challengeID, email string) {
	s.challenge.Observer.ChallengeIssued(purpose)
	s.log.Info(ctx, "challenge issued",
		logger.Attribute{Key: attrChallengeID, Value: challengeID},
		logger.Attribute{Key: attrPurpose, Value: string(purpose)},
		logger.Attribute{Key: "to", Value: maskEmail(email)},
	)
}

// maskEmail masks a recipient address for logging as the first local-part
// character plus the domain (for example a***@example.com); an address without
// a usable local part or domain becomes *** (OTP-15). It mirrors the email
// domain's rule for the same value.
func maskEmail(address string) string {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return "***"
	}
	return address[:1] + "***" + address[at:]
}
