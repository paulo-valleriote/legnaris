package billing

import (
	"context"
	"errors"
	"time"
)

const ProviderStripe = "stripe"

const ProviderComp = "comp"

var (
	ErrProviderUnavailable = errors.New("billing provider unavailable")
	ErrProviderRejected    = errors.New("billing provider rejected the request")
	ErrLiveExists          = errors.New("live subscription already exists")
	ErrInvalidSignature    = errors.New("invalid webhook signature")
)

type Customer struct {
	ID string
}

type CustomerRequest struct {
	IdempotencyKey string
	UserID         string
}

type CheckoutRequest struct {
	IdempotencyKey string
	UserID         string
	SubscriptionID string
	CustomerID     string
	PriceID        string
	SuccessURL     string
	CancelURL      string
}

type CheckoutSession struct {
	ID        string
	URL       string
	ExpiresAt time.Time
}

type ProviderSubscription struct {
	ID                string
	CustomerID        string
	Status            Status
	StartedAt         time.Time
	CurrentPeriodEnd  time.Time
	CancelAtPeriodEnd bool
}

type Notification struct {
	ID                     string
	Kind                   EventKind
	UserID                 string
	ProviderSubscriptionID string
	ProviderCustomerID     string
}

type PortalRequest struct {
	CustomerID string
}

type PortalSession struct {
	URL string
}

type CancelRequest struct {
	ProviderSubscriptionID string
	IdempotencyKey         string
}

type RefundRequest struct {
	ProviderSubscriptionID string
	IdempotencyKey         string
}

type ScrubRequest struct {
	IdempotencyKey string
	CustomerID     string
}

type Provider interface {
	CreateCustomer(ctx context.Context, req CustomerRequest) (Customer, error)
	CreateCheckoutSession(ctx context.Context, req CheckoutRequest) (CheckoutSession, error)
	GetSubscription(ctx context.Context, providerSubscriptionID string) (ProviderSubscription, error)
	ParseWebhook(payload []byte, signature string) (Notification, error)
	CreatePortalSession(ctx context.Context, req PortalRequest) (PortalSession, error)
	CancelAtPeriodEnd(ctx context.Context, req CancelRequest) error
	CancelNow(ctx context.Context, req CancelRequest) error
	RefundLatest(ctx context.Context, req RefundRequest) error
	ScrubCustomer(ctx context.Context, req ScrubRequest) error
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) CreateCustomer(context.Context, CustomerRequest) (Customer, error) {
	return Customer{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CreateCheckoutSession(context.Context, CheckoutRequest) (CheckoutSession, error) {
	return CheckoutSession{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) GetSubscription(context.Context, string) (ProviderSubscription, error) {
	return ProviderSubscription{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) ParseWebhook([]byte, string) (Notification, error) {
	return Notification{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CreatePortalSession(context.Context, PortalRequest) (PortalSession, error) {
	return PortalSession{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CancelAtPeriodEnd(context.Context, CancelRequest) error {
	return ErrProviderUnavailable
}

func (UnconfiguredProvider) CancelNow(context.Context, CancelRequest) error {
	return ErrProviderUnavailable
}

func (UnconfiguredProvider) RefundLatest(context.Context, RefundRequest) error {
	return ErrProviderUnavailable
}

func (UnconfiguredProvider) ScrubCustomer(context.Context, ScrubRequest) error {
	return ErrProviderUnavailable
}
