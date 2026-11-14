// Package generation is the API half of character generation (ADR 0007, LGN-186).
// POST /generations gates a request in a fixed order, reserves the global cap and
// the player's quota and inserts a queued generation document in one transaction,
// then publishes a job for the worker. The worker (LGN-230) owns the transitions
// after queued; this package owns queued creation, reads, accept and the republish
// sweep that recovers jobs whose publish failed.
//
// Concept and output live only in the generation document, for at most 24 h, and
// are never logged (ADR 0008).
package generation

import (
	"time"
)

// Status is the lifecycle state of a generation document (ADR 0007).
type Status string

const (
	// StatusQueued is a job waiting for a worker claim. It is the only state the
	// publish sweeper republishes.
	StatusQueued Status = "queued"
	// StatusRunning is a job a worker claimed under a lease.
	StatusRunning Status = "running"
	// StatusSucceeded is a job whose values passed validation and await accept.
	StatusSucceeded Status = "succeeded"
	// StatusFailed is a job that ended without values; the quota was refunded.
	StatusFailed Status = "failed"
	// StatusAccepted is a succeeded job whose values became a Sheet.
	StatusAccepted Status = "accepted"
)

// KindCharacter is the only generation kind the MVP accepts.
const KindCharacter = "character"

// Quota reservation kinds recorded on a document, so a refund returns the unit to
// the allowance that was taken.
const (
	ReservationTrial = "trial"
	ReservationDaily = "daily"
)

// ErrorCodePublishFailed is the error_code of a job the publish sweeper gave up
// on. The worker writes its own codes, such as provider_unavailable.
const ErrorCodePublishFailed = "publish_failed"

// Lifetime and length bounds of a generation (ADR 0007, LGN-186).
const (
	// Retention is how long a generation document lives before the TTL removes it.
	Retention = 24 * time.Hour
	// MinConceptLength and MaxConceptLength bound the concept in characters.
	MinConceptLength = 10
	MaxConceptLength = 1000
	// RepublishAge is how old a queued document must be before the sweeper
	// republishes it, so a publish that is still in flight is not duplicated.
	RepublishAge = 10 * time.Second
)

// Reservation records the quota unit a generation took: the trial lifetime
// allowance or the paid counter of one UTC day.
type Reservation struct {
	Kind   string `bson:"kind"`
	DayUTC string `bson:"day_utc,omitempty"`
}

// Document is the stored shape of a generation. The fields the worker claims with
// are status, lease_until and attempts; the publish sweeper uses its own pair,
// publish_lease_until and publish_attempts, so the two never fight over a counter.
//
// active_user_id is set only while the job is queued or running. Its sparse unique
// index is the race guard of "one in progress per player"; the terminal transition
// (worker or sweeper) must unset it.
type Document struct {
	ID                string         `bson:"_id"`
	UserID            string         `bson:"user_id"`
	TemplateID        string         `bson:"template_id"`
	Kind              string         `bson:"kind"`
	Concept           string         `bson:"concept"`
	Status            Status         `bson:"status"`
	ActiveUserID      string         `bson:"active_user_id,omitempty"`
	Attempts          int            `bson:"attempts"`
	LeaseUntil        *time.Time     `bson:"lease_until,omitempty"`
	PublishLeaseUntil *time.Time     `bson:"publish_lease_until,omitempty"`
	PublishAttempts   int            `bson:"publish_attempts"`
	Values            map[string]any `bson:"values,omitempty"`
	ErrorCode         string         `bson:"error_code,omitempty"`
	SheetID           string         `bson:"sheet_id,omitempty"`
	Quota             Reservation    `bson:"quota"`
	CreatedAt         time.Time      `bson:"created_at"`
	ExpiresAt         time.Time      `bson:"expires_at"`
}

// View is what GET /generations/{id} answers. Values is present only once the job
// has succeeded, and ErrorCode only once it has failed.
type View struct {
	ID         string
	Status     Status
	Kind       string
	TemplateID string
	Values     map[string]any
	ErrorCode  string
	CreatedAt  time.Time
}

// Message is the job published to the generation queue. It carries identifiers
// only: the worker loads the concept from the document it names.
type Message struct {
	ID   string `json:"generation_id"`
	Kind string `json:"kind"`
}

// Client-facing messages. They are plain strings, never error causes.
const (
	MsgDisabled         = "character generation is currently unavailable"
	MsgUnavailable      = "generation is unavailable"
	MsgKindInvalid      = "generation kind must be character"
	MsgConceptLength    = "concept must be between 10 and 1000 characters"
	MsgTemplateKind     = "template does not accept characters"
	MsgIDInvalid        = "generation id must be a canonical UUID v7"
	MsgNotFound         = "generation not found"
	MsgNameRequired     = "sheet name is required"
	MsgNotAcceptable    = "generation is not ready to accept"
	MsgInProgress       = "a generation is already in progress"
	MsgCapReached       = "the daily generation capacity is reached; try again tomorrow"
	MsgOwnerNotProvided = "authenticated user is required"
)
