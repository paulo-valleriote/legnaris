package billing

import (
	"context"
	"errors"
	"time"
)

const ProviderStripe = "stripe"

var (
	ErrProviderUnavailable = errors.New("billing provider unavailable")
	ErrProviderRejected    = errors.New("billing provider rejected the request")
	ErrLiveExists          = errors.New("live subscription already exists")
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

type PortalRequest struct {
	CustomerID string
}

type PortalSession struct {
	URL string
}

type CancelRequest struct {
	ProviderSubscriptionID string
}

type Provider interface {
	CreateCustomer(ctx context.Context, req CustomerRequest) (Customer, error)
	CreateCheckoutSession(ctx context.Context, req CheckoutRequest) (CheckoutSession, error)
	CreatePortalSession(ctx context.Context, req PortalRequest) (PortalSession, error)
	CancelAtPeriodEnd(ctx context.Context, req CancelRequest) error
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) CreateCustomer(context.Context, CustomerRequest) (Customer, error) {
	return Customer{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CreateCheckoutSession(context.Context, CheckoutRequest) (CheckoutSession, error) {
	return CheckoutSession{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CreatePortalSession(context.Context, PortalRequest) (PortalSession, error) {
	return PortalSession{}, ErrProviderUnavailable
}

func (UnconfiguredProvider) CancelAtPeriodEnd(context.Context, CancelRequest) error {
	return ErrProviderUnavailable
}
