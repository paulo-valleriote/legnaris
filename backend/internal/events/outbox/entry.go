// Package outbox is the transactional event outbox for guaranteed domain events
// (ADR 0010, ADR 0013). An entry is written in the same transaction as the
// entity change it reports, and the relay publishes it afterwards with a lease,
// retries and an attempt cap. The claim shape follows backend/internal/email/outbox.
package outbox

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/idgen"
)

// Status is the lifecycle state of an outbox entry.
type Status string

const (
	// StatusPending marks an entry waiting for a publish attempt.
	StatusPending Status = "pending"
	// StatusPublished marks an entry the broker confirmed. It is terminal.
	StatusPublished Status = "published"
	// StatusDead marks an entry that reached the attempt cap. It is terminal
	// and surfaces for operator attention.
	StatusDead Status = "dead"
)

// ErrInvalidEntry reports an entry that cannot be stored: an identity that is
// not a canonical UUID v7, or an empty body. It is a plain sentinel; check it
// with errors.Is.
var ErrInvalidEntry = errors.New("invalid event outbox entry")

// Entry is one guaranteed event waiting to be published. Body is the encoded
// envelope, fixed when the entry is created, so every republish after a crash is
// byte-identical and consumers deduplicate it by its envelope id. The bson tags
// are the stored field names of the event_outbox collection.
type Entry struct {
	ID            string     `bson:"_id"`
	EventType     string     `bson:"event_type"`
	Body          []byte     `bson:"body"`
	Status        Status     `bson:"status"`
	Attempts      int        `bson:"attempts"`
	NextAttemptAt time.Time  `bson:"next_attempt_at"`
	LockedUntil   *time.Time `bson:"locked_until"`
	LockedBy      string     `bson:"locked_by"`
	CreatedAt     time.Time  `bson:"created_at"`
	PublishedAt   *time.Time `bson:"published_at"`
	LastError     string     `bson:"last_error"`
}

// NewEntry builds a pending entry for ev. It generates the envelope identity
// with idgen, encodes the envelope once and sets next_attempt_at and created_at
// to now. The caller passes now so the clock stays under its control.
func NewEntry(ev events.Event, now time.Time) (Entry, error) {
	id, err := idgen.New()
	if err != nil {
		return Entry{}, fmt.Errorf("generate outbox identity: %w", err)
	}

	body, err := events.Encode(ev, id, now)
	if err != nil {
		return Entry{}, fmt.Errorf("encode outbox entry: %w", err)
	}

	return Entry{
		ID:            id,
		EventType:     ev.Type,
		Body:          body,
		Status:        StatusPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}

// validate rejects an entry the store must never hold: a non-canonical identity
// or an empty body.
func (e Entry) validate() error {
	if err := idgen.Validate(e.ID); err != nil {
		return fmt.Errorf("%w: identity: %v", ErrInvalidEntry, err)
	}
	if len(e.Body) == 0 {
		return fmt.Errorf("%w: empty body", ErrInvalidEntry)
	}
	return nil
}
