package generation

import (
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusAccepted  Status = "accepted"
)

const KindCharacter = "character"

const (
	ReservationTrial = "trial"
	ReservationDaily = "daily"
)

const (
	ErrorCodePublishFailed       = "publish_failed"
	ErrorCodeProviderUnavailable = "provider_unavailable"
	ErrorCodeProviderFailed      = "provider_failed"
	ErrorCodeInvalidOutput       = "invalid_output"
	ErrorCodeTemplateUnavailable = "template_unavailable"
	ErrorCodeLeaseExpired        = "lease_expired"
)

const (
	Retention        = 24 * time.Hour
	MinConceptLength = 10
	MaxConceptLength = 1000
	RepublishAge     = 10 * time.Second
)

const (
	LeaseDuration    = 90 * time.Second
	MaxSchemaRetries = 2
	TransportRetries = 1
	TransportBackoff = 500 * time.Millisecond
	MaxOutputTokens  = 6000
	ReapInterval     = 10 * time.Second
)

type Reservation struct {
	Kind   string `bson:"kind"`
	DayUTC string `bson:"day_utc,omitempty"`
}

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
	ReapAttempts      int            `bson:"reap_attempts"`
	Values            map[string]any `bson:"values,omitempty"`
	ErrorCode         string         `bson:"error_code,omitempty"`
	SheetID           string         `bson:"sheet_id,omitempty"`
	Quota             Reservation    `bson:"quota"`
	CreatedAt         time.Time      `bson:"created_at"`
	ExpiresAt         time.Time      `bson:"expires_at"`
}

type View struct {
	ID            string
	Status        Status
	Kind          string
	TemplateID    string
	Values        map[string]any
	ErrorCode     string
	SuggestedName *string
	CreatedAt     time.Time
}

type Message struct {
	ID   string `json:"generation_id"`
	Kind string `json:"kind"`
}

const (
	MsgDisabled         = "character generation is currently unavailable"
	MsgUnavailable      = "generation is unavailable"
	MsgKindInvalid      = "generation kind must be character"
	MsgConceptLength    = "concept must be between 10 and 1000 characters"
	MsgTemplateKind     = "template does not accept characters"
	MsgIDInvalid        = "generation id must be a canonical UUID v7"
	MsgNotFound         = "generation not found"
	MsgNameRequired     = "sheet name is required"
	MsgValuesRequired   = "sheet values are required"
	MsgNotAcceptable    = "generation is not ready to accept"
	MsgInProgress       = "a generation is already in progress"
	MsgCapReached       = "the daily generation capacity is reached; try again tomorrow"
	MsgOwnerNotProvided = "authenticated user is required"
)
