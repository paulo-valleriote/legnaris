// Package llmconsent records and gates the player's consent to send concept
// text to the LLM gateway (ADR 0008, LGN-194). The Version constant must match
// the disclosure in llm-consent.json, which the tests compare.
package llmconsent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/apperr"
)

// Version is the consent disclosure version the backend accepts. Bump it
// whenever the disclosure changes (provider, model, region, retention or the
// revocation path), and update llm-consent.json in the same change.
const Version = "1"

const (
	// MsgConsentRequired is the client-facing reason for LLM_CONSENT_REQUIRED.
	MsgConsentRequired = "llm consent is required"
	// MsgVersionMismatch is the client-facing reason for a consent version
	// other than the current one.
	MsgVersionMismatch = "llm consent version is not the current one"
)

// ErrUserNotFound reports a consent operation on a user document that does
// not exist.
var ErrUserNotFound = errors.New("user not found")

// Record is the consent state stored on the user. At is nil when the user has
// never consented or has revoked consent.
type Record struct {
	At      *time.Time
	Version string
}

// ConsentStore reads and writes the consent fields of the user document.
type ConsentStore interface {
	Fetch(ctx context.Context, userID string) (Record, error)
	Grant(ctx context.Context, userID, version string, at time.Time) error
	Clear(ctx context.Context, userID string) error
}

// Quota reservation kinds carried by an OpenGeneration. They match the kinds the
// generation package records on a document.
const (
	ReservationTrial = "trial"
	ReservationDaily = "daily"
)

// OpenGeneration is the part of a queued or running generation that a revoke
// needs: the document to refund and the allowance it reserved.
type OpenGeneration struct {
	ID              string
	UserID          string
	ReservationKind string
	DayUTC          string
}

// GenerationStore reads and deletes the generation documents owned by a user.
type GenerationStore interface {
	// ListOpenByUserID returns the user's queued and running generations, the
	// only ones whose quota is still reserved.
	ListOpenByUserID(ctx context.Context, userID string) ([]OpenGeneration, error)
	DeleteByUserID(ctx context.Context, userID string) (int64, error)
}

// QuotaRefunder returns the quota unit an open generation reserved. The
// generation package implements it, so this package does not import it.
type QuotaRefunder interface {
	RefundOpen(ctx context.Context, open OpenGeneration) error
}

// Transactor runs fn in one transaction. *db.Transactor satisfies it.
type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Service is the consent use case behind the /me/llm-consent routes and the
// generation gate.
type Service interface {
	Grant(ctx context.Context, userID, version string) error
	Revoke(ctx context.Context, userID string) error
	Gate(ctx context.Context, userID string) error
	Status(ctx context.Context, userID string) (Record, error)
}

// CheckConsent returns LLM_CONSENT_REQUIRED unless the record holds a consent
// at the current version. A missing and an outdated consent are the same
// refusal: the client shows the disclosure again in both cases.
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

// NewService builds the consent service. now supplies the consent timestamp.
func NewService(consents ConsentStore, generations GenerationStore, refunder QuotaRefunder, tx Transactor, now func() time.Time) Service {
	return &service{consents: consents, generations: generations, refunder: refunder, tx: tx, now: now}
}

// Grant records consent at the current version. A repeated grant at the same
// version keeps the original timestamp.
func (s *service) Grant(ctx context.Context, userID, version string) error {
	if version != Version {
		return apperr.NewError(apperr.CodeValidation, MsgVersionMismatch, nil)
	}

	if err := s.consents.Grant(ctx, userID, version, s.now()); err != nil {
		return translateUserErr(err)
	}
	return nil
}

// Revoke clears the consent, refunds the quota of the user's queued and running
// generations and deletes their generation documents in one transaction, so a
// crash never leaves a cleared consent with data still held or allowance lost.
// Revoking without a consent is a no-op.
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

// refundOpen returns the quota of every open generation of userID, so deleting
// the document does not take the allowance with it.
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

// Gate refuses with LLM_CONSENT_REQUIRED unless the user holds a current
// consent. It is the first check of POST /generations (ADR 0008).
func (s *service) Gate(ctx context.Context, userID string) error {
	record, err := s.consents.Fetch(ctx, userID)
	if err != nil {
		return translateUserErr(err)
	}
	return CheckConsent(record)
}

// Status returns the stored consent record as it is, whether or not it is
// current, so the account screen can show when and at which version the player
// consented. The generation gate stays the authority on whether consent counts.
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
