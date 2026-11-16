package stripe

import (
	"context"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
)

func (p *Provider) CancelNow(ctx context.Context, req billing.CancelRequest) error {
	params := &stripesdk.SubscriptionCancelParams{}
	params.SetIdempotencyKey(req.IdempotencyKey)

	if _, err := p.client.V1Subscriptions.Cancel(ctx, req.ProviderSubscriptionID, params); err != nil {
		return mapError(err)
	}
	return nil
}

func (p *Provider) ScrubCustomer(ctx context.Context, req billing.ScrubRequest) error {
	params := &stripesdk.CustomerUpdateParams{
		Name:        stripesdk.String(""),
		Email:       stripesdk.String(""),
		Phone:       stripesdk.String(""),
		Description: stripesdk.String(""),
		Metadata:    map[string]string{metadataUserID: ""},
	}
	params.SetIdempotencyKey(req.IdempotencyKey)

	if _, err := p.client.V1Customers.Update(ctx, req.CustomerID, params); err != nil {
		return mapError(err)
	}
	return nil
}
