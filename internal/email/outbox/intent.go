package outbox

import (
	"errors"
	"fmt"
	"time"

	"legnaris/shared/idgen"
)

// Status is the lifecycle state of a send intention (EML-04).
type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

// Payload is the code, its lifetime and the purpose carried to the worker for
// rendering. The code is a short-lived credential and must never reach a log
// line or an error message (EML-13).
type Payload struct {
	Code             string `bson:"code" json:"code"`
	ExpiresInMinutes int    `bson:"expires_in_minutes" json:"expires_in_minutes"`
	Purpose          string `bson:"purpose" json:"purpose"`
}

// Intent is the durable record that an email must be sent, written before the
// request is reported accepted so a crash never loses the code (EML-01). The
// collection is email_outbox; the bson tags match data-model.md §1.
type Intent struct {
	ID            string     `bson:"_id"`
	To            string     `bson:"to"`
	Template      string     `bson:"template"`
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

// ErrInvalidIntentID reports an identity that is not a canonical lowercase UUID
// v7 — what shared/idgen.New produces. A read identity that is not canonical is
// rejected rather than published with a malformed idempotency key (EML-18).
//
// It is a plain errors.New sentinel, never apperr: this package is a non-HTTP
// layer with no status to map. The precedent is shared/idgen.ErrInvalidID and
// shared/db's own typed errors; the constitution's Principle I is scoped to
// boundaries and there is none here.
var ErrInvalidIntentID = errors.New("intent identity is not a canonical UUID v7")

// NewIntent records an intention: it generates a canonical UUID v7 identity
// when id is empty, or validates the supplied one, and starts the row pending
// with next_attempt_at = now, attempts = 0 and no lease (EML-01, EML-18).
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
		Payload:       payload,
		Status:        StatusPending,
		Attempts:      0,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}

// validateIntentID screens an identity against the repository-wide UUID v7
// contract and maps the lower-layer sentinel to the one the relay acts on
// (EML-18, EML-19).
func validateIntentID(id string) error {
	if err := idgen.Validate(id); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidIntentID, err)
	}
	return nil
}
