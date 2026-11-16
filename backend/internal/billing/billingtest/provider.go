package billingtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"legnaris/backend/internal/billing"
)

const sessionLifetime = 24 * time.Hour

type Provider struct {
	mu               sync.Mutex
	customers        map[string]billing.Customer
	sessions         map[string]billing.CheckoutSession
	customersCreated int
	sessionsCreated  int
	Unavailable      bool
}

func NewProvider() *Provider {
	return &Provider{
		customers: make(map[string]billing.Customer),
		sessions:  make(map[string]billing.CheckoutSession),
	}
}

func (p *Provider) CreateCustomer(_ context.Context, req billing.CustomerRequest) (billing.Customer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.Customer{}, billing.ErrProviderUnavailable
	}
	if c, ok := p.customers[req.IdempotencyKey]; ok {
		return c, nil
	}
	p.customersCreated++
	c := billing.Customer{ID: fmt.Sprintf("cus_fake_%d", p.customersCreated)}
	p.customers[req.IdempotencyKey] = c
	return c, nil
}

func (p *Provider) CreateCheckoutSession(_ context.Context, req billing.CheckoutRequest) (billing.CheckoutSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.CheckoutSession{}, billing.ErrProviderUnavailable
	}
	if s, ok := p.sessions[req.IdempotencyKey]; ok {
		return s, nil
	}
	p.sessionsCreated++
	s := billing.CheckoutSession{
		ID:        fmt.Sprintf("cs_fake_%d", p.sessionsCreated),
		URL:       fmt.Sprintf("https://checkout.example/session/%d", p.sessionsCreated),
		ExpiresAt: time.Now().Add(sessionLifetime),
	}
	p.sessions[req.IdempotencyKey] = s
	return s, nil
}

func (p *Provider) CustomersCreated() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.customersCreated
}

func (p *Provider) SessionsCreated() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sessionsCreated
}
