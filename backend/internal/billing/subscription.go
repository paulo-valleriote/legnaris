package billing

import (
	"context"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusActive   Status = "active"
	StatusCanceled Status = "canceled"
)

type Subscription struct {
	ID                 string    `bson:"_id"`
	UserID             string    `bson:"user_id"`
	PlanID             string    `bson:"plan_id"`
	Provider           string    `bson:"provider"`
	ProviderCustomerID string    `bson:"provider_customer_id,omitempty"`
	Status             Status    `bson:"status"`
	CheckoutURL        string    `bson:"checkout_url,omitempty"`
	CheckoutExpiresAt  time.Time `bson:"checkout_expires_at"`
	CreatedAt          time.Time `bson:"created_at"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

type Store interface {
	FindLive(ctx context.Context, userID string) (Subscription, bool, error)
	InsertPending(ctx context.Context, sub Subscription) error
	Cancel(ctx context.Context, id string) error
	AttachCheckout(ctx context.Context, id, customerID, url string, expiresAt time.Time) (bool, error)
}
