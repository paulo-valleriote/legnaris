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

const (
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
	MaxSchemaRetries = 2
	TransportRetries = 1
	TransportBackoff = 500 * time.Millisecond
	MaxOutputTokens  = 8000
	ReapInterval     = 10 * time.Second

	leaseMargin = 20 * time.Second
)

func LeaseDurationFor(llmTimeout time.Duration) time.Duration {
	return jobBudgetFor(llmTimeout) + settleTimeout + leaseMargin
}

func jobBudgetFor(llmTimeout time.Duration) time.Duration {
	attempts := time.Duration(1+MaxSchemaRetries) * time.Duration(1+TransportRetries)
	backoffs := time.Duration(1+MaxSchemaRetries) * TransportBackoff
	return attempts*llmTimeout + backoffs
}

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
	PromptVersion     string         `bson:"prompt_version,omitempty"`
	Model             string         `bson:"model,omitempty"`
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

type Metadata struct {
	PromptVersion string
	Model         string
}

const (
	MsgDisabled            = "generation is currently unavailable"
	MsgUnavailable         = "generation is unavailable"
	MsgKindInvalid         = "generation kind must be character, npc or enemy"
	MsgConceptLength       = "concept must be between 10 and 1000 characters"
	MsgTemplateKind        = "template does not accept this sheet kind"
	MsgTemplateState       = "template is not available in this environment"
	MsgTemplateRetired     = "template is retired and cannot start new generations"
	MsgIDInvalid           = "generation id must be a canonical UUID v7"
	MsgNotFound            = "generation not found"
	MsgValuesRequired      = "sheet values are required"
	MsgNotAcceptable       = "generation is not ready to accept"
	MsgInProgress          = "a generation is already in progress"
	MsgCapReached          = "the daily generation capacity is reached; try again tomorrow"
	MsgOwnerNotProvided    = "authenticated user is required"
	MsgPlanUpgradeRequired = "AI generation requires a paid plan"
)
