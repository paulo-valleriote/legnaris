package outbox

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/shared/idgen"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

type Kind string

const (
	KindInitial Kind = "initial"
	KindResend  Kind = "resend"
)

type Payload struct {
	Code             string `bson:"code" json:"code"`
	ExpiresInMinutes int    `bson:"expires_in_minutes" json:"expires_in_minutes"`
	Purpose          string `bson:"purpose" json:"purpose"`
}

type Intent struct {
	ID            string     `bson:"_id"`
	To            string     `bson:"to"`
	Template      string     `bson:"template"`
	Kind          Kind       `bson:"kind"`
	Payload       Payload    `bson:"payload"`
	Status        Status     `bson:"status"`
	Attempts      int        `bson:"attempts"`
	NextAttemptAt time.Time  `bson:"next_attempt_at"`
	LockedUntil   *time.Time `bson:"locked_until"`
	LockedBy      string     `bson:"locked_by"`
	CreatedAt     time.Time  `bson:"created_at"`
	PublishedAt   *time.Time `bson:"published_at"`
	LastError     string     `bson:"last_error"`
}

var ErrInvalidIntentID = errors.New("intent identity is not a canonical UUID v7")

func NewIntent(id, to, template string, payload Payload, now time.Time) (Intent, error) {
	if id == "" {
		generated, err := idgen.New()
		if err != nil {
			return Intent{}, fmt.Errorf("generate intent identity: %w", err)
		}
		id = generated
	} else if err := validateIntentID(id); err != nil {
		return Intent{}, err
	}

	return Intent{
		ID:            id,
		To:            to,
		Template:      template,
		Kind:          KindInitial,
		Payload:       payload,
		Status:        StatusPending,
		Attempts:      0,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}

func validateIntentID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidIntentID, err)
	}
	return nil
}
