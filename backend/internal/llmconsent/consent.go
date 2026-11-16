package llmconsent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/apperr"
)

const Version = "3"

const (
	MsgConsentRequired = "llm consent is required"
	MsgVersionMismatch = "llm consent version is not the current one"
)

var ErrUserNotFound = errors.New("user not found")

type Record struct {
	At      *time.Time
	Version string
}

type ConsentStore interface {
	Fetch(ctx context.Context, userID string) (Record, error)
	Grant(ctx context.Context, userID, version string, at time.Time) error
	Clear(ctx context.Context, userID string) error
}

const (
	ReservationTrial = "trial"
	ReservationDaily = "daily"
)

type OpenGeneration struct {
	ID              string
	UserID          string
	ReservationKind string
	DayUTC          string
}

type GenerationStore interface {
	ListOpenByUserID(ctx context.Context, userID string) ([]OpenGeneration, error)
	DeleteByUserID(ctx context.Context, userID string) (int64, error)
}

type QuotaRefunder interface {
	RefundOpen(ctx context.Context, open OpenGeneration) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service interface {
	Grant(ctx context.Context, userID, version string) error
	Revoke(ctx context.Context, userID string) error
	Gate(ctx context.Context, userID string) error
	Status(ctx context.Context, userID string) (Record, error)
}

func CheckConsent(record Record) error {
	if record.At == nil || record.Version != Version {
		return apperr.NewError(apperr.CodeLLMConsentRequired, MsgConsentRequired, nil)
	}
	return nil
}

type service struct {
	consents    ConsentStore
	generations GenerationStore
	refunder    QuotaRefunder
	tx          Transactor
	now         func() time.Time
}

func NewService(consents ConsentStore, generations GenerationStore, refunder QuotaRefunder, tx Transactor, now func() time.Time) Service {
	return &service{consents: consents, generations: generations, refunder: refunder, tx: tx, now: now}
}

func (s *service) Grant(ctx context.Context, userID, version string) error {
	if version != Version {
		return apperr.NewError(apperr.CodeValidation, MsgVersionMismatch, nil)
	}

	if err := s.consents.Grant(ctx, userID, version, s.now()); err != nil {
		return translateUserErr(err)
	}
	return nil
}

func (s *service) Revoke(ctx context.Context, userID string) error {
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.consents.Clear(txCtx, userID); err != nil {
			return err
		}
		if err := s.refundOpen(txCtx, userID); err != nil {
			return err
		}
		_, err := s.generations.DeleteByUserID(txCtx, userID)
		return err
	})
	if err != nil {
		return fmt.Errorf("revoke llm consent: %w", err)
	}
	return nil
}

func (s *service) refundOpen(ctx context.Context, userID string) error {
	open, err := s.generations.ListOpenByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, generation := range open {
		if err := s.refunder.RefundOpen(ctx, generation); err != nil {
			return fmt.Errorf("refund generation %s: %w", generation.ID, err)
		}
	}
	return nil
}

func (s *service) Gate(ctx context.Context, userID string) error {
	record, err := s.consents.Fetch(ctx, userID)
	if err != nil {
		return translateUserErr(err)
	}
	return CheckConsent(record)
}

func (s *service) Status(ctx context.Context, userID string) (Record, error) {
	record, err := s.consents.Fetch(ctx, userID)
	if err != nil {
		return Record{}, translateUserErr(err)
	}
	return record, nil
}

func translateUserErr(err error) error {
	if errors.Is(err, ErrUserNotFound) {
		return apperr.NewError(apperr.CodeNotFound, "user not found", err)
	}
	return fmt.Errorf("llm consent: %w", err)
}
