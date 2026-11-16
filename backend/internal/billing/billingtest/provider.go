package billingtest

import (
	"context"
	"encoding/json"
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
	portalsCreated   int
	cancelsScheduled []string
	Unavailable      bool
	subscriptions    map[string]billing.ProviderSubscription
	reads            int
	cancels          []billing.CancelRequest
	scrubs           []billing.ScrubRequest
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

const ValidSignature = "valid-signature"

func (p *Provider) SetSubscription(sub billing.ProviderSubscription) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.subscriptions == nil {
		p.subscriptions = make(map[string]billing.ProviderSubscription)
	}
	p.subscriptions[sub.ID] = sub
}

func (p *Provider) GetSubscription(_ context.Context, id string) (billing.ProviderSubscription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.ProviderSubscription{}, billing.ErrProviderUnavailable
	}
	p.reads++
	sub, ok := p.subscriptions[id]
	if !ok {
		return billing.ProviderSubscription{}, fmt.Errorf("subscription %s: %w", id, billing.ErrProviderRejected)
	}
	return sub, nil
}

func (p *Provider) Reads() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reads
}

func (p *Provider) ParseWebhook(payload []byte, signature string) (billing.Notification, error) {
	if signature != ValidSignature {
		return billing.Notification{}, billing.ErrInvalidSignature
	}
	var n billing.Notification
	if err := json.Unmarshal(payload, &n); err != nil {
		return billing.Notification{}, fmt.Errorf("%w: %w", billing.ErrProviderRejected, err)
	}
	return n, nil
}

func (p *Provider) CancelNow(_ context.Context, req billing.CancelRequest) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.ErrProviderUnavailable
	}
	sub, ok := p.subscriptions[req.ProviderSubscriptionID]
	if !ok {
		return fmt.Errorf("subscription %s: %w", req.ProviderSubscriptionID, billing.ErrProviderRejected)
	}
	sub.Status = billing.StatusCanceled
	p.subscriptions[req.ProviderSubscriptionID] = sub
	p.cancels = append(p.cancels, req)
	return nil
}

func (p *Provider) ScrubCustomer(_ context.Context, req billing.ScrubRequest) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.ErrProviderUnavailable
	}
	p.scrubs = append(p.scrubs, req)
	return nil
}

func (p *Provider) Cancels() []billing.CancelRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]billing.CancelRequest(nil), p.cancels...)
}

func (p *Provider) Scrubs() []billing.ScrubRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]billing.ScrubRequest(nil), p.scrubs...)
}
