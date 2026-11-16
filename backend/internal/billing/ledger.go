package billing

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/shared/db"
)

const (
	EventsCollection       = "billing_events"
	UserLocksCollection    = "billing_user_locks"
	ReasonIgnored          = "ignored"
	inboxRetentionDuration = 90 * 24 * time.Hour
)

var ErrDuplicateEvent = errors.New("billing event already received")

type EventKind string

const (
	KindSubscriptionChanged EventKind = "subscription.changed"
	KindIgnored             EventKind = "ignored"
)

type EventStatus string

const (
	EventReceived  EventStatus = "received"
	EventProcessed EventStatus = "processed"
	EventFailed    EventStatus = "failed"
)

type Event struct {
	ID                     string      `bson:"_id"`
	Provider               string      `bson:"provider"`
	ProviderEventID        string      `bson:"provider_event_id"`
	Kind                   EventKind   `bson:"type"`
	UserID                 string      `bson:"user_id,omitempty"`
	ProviderSubscriptionID string      `bson:"provider_subscription_id,omitempty"`
	ProviderCustomerID     string      `bson:"provider_customer_id,omitempty"`
	Status                 EventStatus `bson:"status"`
	Reason                 string      `bson:"reason,omitempty"`
	Attempts               int         `bson:"attempts"`
	NextAttemptAt          time.Time   `bson:"next_attempt_at"`
	LastError              string      `bson:"last_error,omitempty"`
	ReceivedAt             time.Time   `bson:"received_at"`
	ProcessedAt            *time.Time  `bson:"processed_at,omitempty"`
	Payload                []byte      `bson:"payload"`
	ExpiresAt              time.Time   `bson:"expires_at"`
	Claimer                string      `bson:"-"`
}

type AccountPlan struct {
	Exists      bool
	Deleting    bool
	Plan        string
	PlanVersion int64
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Step(ctx context.Context, step db.Step) error
}

type Outbox interface {
	Insert(ctx context.Context, entry outbox.Entry) error
}

type Ledger interface {
	RefundLedger
	InsertEvent(ctx context.Context, ev Event) error
	ResolveUser(ctx context.Context, providerSubscriptionID, providerCustomerID string) (string, bool, error)
	ClaimEvents(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Event, error)
	ReleaseUser(ctx context.Context, userID, claimer string) error
	RescheduleEvent(ctx context.Context, id string, nextAt time.Time, reason string) error
	FailEvent(ctx context.Context, id string, reason string) error
	MarkEventProcessed(ctx context.Context, id, reason string, at time.Time) error
	OldestReceived(ctx context.Context) (time.Time, bool, error)
	AccountPlan(ctx context.Context, userID string) (AccountPlan, error)
	SetPlan(ctx context.Context, userID, plan string) (int64, error)
	SubscriptionsOf(ctx context.Context, userID string) ([]Subscription, error)
	FindTarget(ctx context.Context, userID, providerSubscriptionID string) (Subscription, bool, error)
	ApplySubscription(ctx context.Context, sub Subscription, readAt time.Time) (bool, error)
	InsertSubscription(ctx context.Context, sub Subscription) error
}
