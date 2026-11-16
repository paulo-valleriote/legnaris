package stripe

import (
	"context"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
)

func (p *Provider) CreatePortalSession(ctx context.Context, req billing.PortalRequest) (billing.PortalSession, error) {
	params := &stripesdk.BillingPortalSessionCreateParams{
		Customer: stripesdk.String(req.CustomerID),
	}

	session, err := p.client.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return billing.PortalSession{}, mapError(err)
	}
	return billing.PortalSession{URL: session.URL}, nil
}

func (p *Provider) CancelAtPeriodEnd(ctx context.Context, req billing.CancelRequest) error {
	params := &stripesdk.SubscriptionUpdateParams{
		CancelAtPeriodEnd: stripesdk.Bool(true),
	}

	if _, err := p.client.V1Subscriptions.Update(ctx, req.ProviderSubscriptionID, params); err != nil {
		return mapError(err)
	}
	return nil
}
