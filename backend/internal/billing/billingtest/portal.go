package billingtest

import (
	"context"
	"fmt"

	"legnaris/backend/internal/billing"
)

func (p *Provider) CreatePortalSession(_ context.Context, req billing.PortalRequest) (billing.PortalSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.PortalSession{}, billing.ErrProviderUnavailable
	}
	p.portalsCreated++
	return billing.PortalSession{
		URL: fmt.Sprintf("https://billing.example/portal/%s/%d", req.CustomerID, p.portalsCreated),
	}, nil
}

func (p *Provider) CancelAtPeriodEnd(_ context.Context, req billing.CancelRequest) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Unavailable {
		return billing.ErrProviderUnavailable
	}
	p.cancelsScheduled = append(p.cancelsScheduled, req.ProviderSubscriptionID)
	return nil
}

func (p *Provider) PortalsCreated() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.portalsCreated
}

func (p *Provider) CancelsScheduled() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.cancelsScheduled...)
}
