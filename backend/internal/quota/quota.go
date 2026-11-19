package quota

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/shared/apperr"
	"legnaris/backend/shared/idgen"
)

const TrialAllowance = 3

const PaidDailyLimit = 30

const usageRetention = 48 * time.Hour

const dayLayout = "2006-01-02"

const MsgInvalidUserID = "user id is not a valid identifier"

const MsgTrialExhausted = "free generation trial is used up; upgrade to keep generating"

const MsgDailyQuotaExceeded = "daily generation quota reached"

const MsgQuotaUnavailable = "generation quota is unavailable"

const MsgInvalidDay = "quota day is not a UTC date"

type Store interface {
	GrantTrial(ctx context.Context, userID string, allowance int) error
	ReserveTrial(ctx context.Context, userID string) (bool, error)
	RefundTrial(ctx context.Context, userID string) error
	ReserveDaily(ctx context.Context, userID, dayUTC string, expiresAt time.Time) (bool, error)
	RefundDaily(ctx context.Context, userID, dayUTC string) error
	DeleteDaily(ctx context.Context, userID, dayUTC string) error
	FetchTrial(ctx context.Context, userID string) (TrialRow, error)
	FetchDaily(ctx context.Context, userID, dayUTC string) (int, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

type TrialGranter struct {
	store Store
}

func NewTrialGranter(store Store) *TrialGranter {
	return &TrialGranter{store: store}
}

func (g *TrialGranter) Grant(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := g.store.GrantTrial(ctx, userID, TrialAllowance); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("grant trial: %w", err))
	}
	return nil
}

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

func (s *Service) RefundTrial(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if err := s.store.RefundTrial(ctx, userID); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("refund trial: %w", err))
	}
	return nil
}

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

func (s *Service) ResetDaily(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	dayUTC := dayStart(s.now().UTC()).Format(dayLayout)
	if err := s.store.DeleteDaily(ctx, userID, dayUTC); err != nil {
		return apperr.NewError(apperr.CodeInternal, MsgQuotaUnavailable, fmt.Errorf("reset daily: %w", err))
	}
	return nil
}

type TrialStatus struct {
	Allowance int
	Used      int
	Remaining int
}

type DailyStatus struct {
	Limit     int
	Used      int
	Remaining int
	ResetsAt  time.Time
}

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

func dayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func parseDay(dayUTC string) (time.Time, error) {
	day, err := time.Parse(dayLayout, dayUTC)
	if err != nil || day.Format(dayLayout) != dayUTC {
		return time.Time{}, apperr.NewError(apperr.CodeInvalidRequest, MsgInvalidDay, err)
	}
	return day, nil
}

func validateUserID(userID string) error {
	if err := idgen.Validate(userID); err != nil {
		return apperr.NewError(apperr.CodeUnauthorized, MsgInvalidUserID, err)
	}
	return nil
}
