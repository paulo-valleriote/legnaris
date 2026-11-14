// Package quota meters generations per account: a lifetime Free trial counter
// and a paid daily counter (ADR 0009). Every reservation is a single conditional
// update, so two concurrent requests at the limit cannot both succeed. Callers
// that also write a generation run Reserve and Refund inside the transaction
// helper (backend/shared/db), so a refund on failure is part of the same state
// change as the terminal status.
package quota

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

// TrialAllowance is the lifetime generation allowance granted to a verified,
// non-disposable account (ADR 0009). LGN-229 writes it into the trial row; this
// package only reads the row's own allowance, so an account without a row has
// none.
const TrialAllowance = 3

// PaidDailyLimit caps the generations a paid account may reserve per UTC day,
// across every kind of generation (ADR 0009).
const PaidDailyLimit = 30

// usageRetention keeps a day's counter for one more day after it closes, so a
// request that straddles midnight can still refund against its own day before
// the TTL removes the row.
const usageRetention = 48 * time.Hour

// dayLayout is the storage form of a UTC day, the second half of the daily
// counter's key.
const dayLayout = "2006-01-02"

// MsgInvalidUserID is returned when a caller passes a user id that is not a
// canonical UUID v7. It never reaches a storage query.
const MsgInvalidUserID = "user id is not a valid identifier"

// MsgTrialExhausted refuses a generation once the Free allowance is spent or
// was never granted.
const MsgTrialExhausted = "free generation trial is used up; upgrade to keep generating"

// MsgDailyQuotaExceeded refuses a generation past the paid daily limit.
const MsgDailyQuotaExceeded = "daily generation quota reached"

// MsgQuotaUnavailable hides a storage failure behind a generic internal error.
const MsgQuotaUnavailable = "generation quota is unavailable"

// MsgInvalidDay is returned when a refund names a day that is not a UTC day key.
const MsgInvalidDay = "quota day is not a UTC date"

// Store is the persistence port for the trial and daily counters. Every
// reservation method reports false instead of an error when the counter is at
// its limit, so the service decides the apperr code.
type Store interface {
	ReserveTrial(ctx context.Context, userID string) (bool, error)
	RefundTrial(ctx context.Context, userID string) error
	ReserveDaily(ctx context.Context, userID, dayUTC string, expiresAt time.Time) (bool, error)
	RefundDaily(ctx context.Context, userID, dayUTC string) error
	FetchTrial(ctx context.Context, userID string) (TrialRow, error)
	FetchDaily(ctx context.Context, userID, dayUTC string) (int, error)
}

// Service reserves, refunds and reports generation allowance.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService returns a Service that reads the day from now. A nil now uses the
// wall clock.
func NewService(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

// ReserveTrial takes one unit of the lifetime Free allowance. It returns
// PLAN_UPGRADE_REQUIRED (403) when the allowance is spent or missing. ctx must
// be the transaction context when the caller writes a generation in the same
// transaction.
func (s *Service) ReserveTrial(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	ok, err := s.store.ReserveTrial(ctx, userID)
	if err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("reserve trial: %w", err))
	}
	if !ok {
		return apperr.NewError(apperr.CodePlanUpgradeRequired, MsgTrialExhausted, nil)
	}
	return nil
}

// RefundTrial returns one unit of the trial allowance taken by a failed
// generation. A refund on an unused allowance is a no-op, so it never goes
// negative.
func (s *Service) RefundTrial(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := s.store.RefundTrial(ctx, userID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("refund trial: %w", err))
	}
	return nil
}

// ReserveDaily takes one unit of the paid daily quota for the current UTC day
// and returns that day's key. The caller passes the key to RefundDaily, so a
// refund that runs after midnight still lands on the day it was reserved on.
// It returns QUOTA_EXCEEDED (429) with a Retry-After up to the next UTC
// midnight when the day is full.
func (s *Service) ReserveDaily(ctx context.Context, userID string) (string, error) {
	if err := validateUserID(userID); err != nil {
		return "", err
	}
	now := s.now().UTC()
	day := dayStart(now)
	dayUTC := day.Format(dayLayout)

	ok, err := s.store.ReserveDaily(ctx, userID, dayUTC, day.Add(usageRetention))
	if err != nil {
		return "", apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("reserve daily: %w", err))
	}
	if !ok {
		retryAfter := day.Add(24 * time.Hour).Sub(now)
		return "", apperr.NewError(apperr.CodeQuotaExceeded, MsgDailyQuotaExceeded, nil).WithRetryAfter(retryAfter)
	}
	return dayUTC, nil
}

// RefundDaily returns one unit to the day that ReserveDaily reserved on. A
// refund on an empty day is a no-op.
func (s *Service) RefundDaily(ctx context.Context, userID, dayUTC string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if _, err := parseDay(dayUTC); err != nil {
		return err
	}
	if err := s.store.RefundDaily(ctx, userID, dayUTC); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("refund daily: %w", err))
	}
	return nil
}

// TrialStatus reports the lifetime allowance of an account.
type TrialStatus struct {
	Allowance int
	Used      int
	Remaining int
}

// DailyStatus reports the paid quota for the current UTC day.
type DailyStatus struct {
	Limit     int
	Used      int
	Remaining int
	ResetsAt  time.Time
}

// TrialStatus reads the trial counter. An account without a trial row reports
// zero allowance.
func (s *Service) TrialStatus(ctx context.Context, userID string) (TrialStatus, error) {
	if err := validateUserID(userID); err != nil {
		return TrialStatus{}, err
	}
	row, err := s.store.FetchTrial(ctx, userID)
	if err != nil {
		return TrialStatus{}, apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("fetch trial: %w", err))
	}
	return TrialStatus{
		Allowance: row.Allowance,
		Used:      row.Used,
		Remaining: max(row.Allowance-row.Used, 0),
	}, nil
}

// DailyStatus reads the paid counter for the current UTC day. ResetsAt is the
// next UTC midnight.
func (s *Service) DailyStatus(ctx context.Context, userID string) (DailyStatus, error) {
	if err := validateUserID(userID); err != nil {
		return DailyStatus{}, err
	}
	now := s.now().UTC()
	day := dayStart(now)
	used, err := s.store.FetchDaily(ctx, userID, day.Format(dayLayout))
	if err != nil {
		return DailyStatus{}, apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("fetch daily: %w", err))
	}
	return DailyStatus{
		Limit:     PaidDailyLimit,
		Used:      used,
		Remaining: max(PaidDailyLimit-used, 0),
		ResetsAt:  day.Add(24 * time.Hour),
	}, nil
}

// dayStart returns the UTC midnight that opens t's day.
func dayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// parseDay accepts only the storage form of a UTC day.
func parseDay(dayUTC string) (time.Time, error) {
	day, err := time.Parse(dayLayout, dayUTC)
	if err != nil || day.Format(dayLayout) != dayUTC {
		return time.Time{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidDay, err)
	}
	return day, nil
}

// validateUserID translates a malformed id into apperr at the service boundary,
// before any storage call.
func validateUserID(userID string) error {
	if err := idgen.Validate(userID); err != nil {
		return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidUserID, err)
	}
	return nil
}
