package outbox

import (
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/shared/idgen"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusDead      Status = "dead"
)

var ErrInvalidEntry = errors.New("invalid event outbox entry")

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

func (e Entry) validate() error {
	if err := idgen.Validate(e.ID); err != nil {
		return fmt.Errorf("%w: identity: %w", ErrInvalidEntry, err)
	}
	if len(e.Body) == 0 {
		return fmt.Errorf("%w: empty body", ErrInvalidEntry)
	}
	return nil
}
