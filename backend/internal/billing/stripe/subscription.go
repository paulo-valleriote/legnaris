package stripe

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
)

func (p *Provider) GetSubscription(ctx context.Context, providerSubscriptionID string) (billing.ProviderSubscription, error) {
	sub, err := p.client.V1Subscriptions.Retrieve(ctx, providerSubscriptionID, nil)
	if err != nil {
		return billing.ProviderSubscription{}, mapError(err)
	}
	status, err := mapStatus(sub.Status)
	if err != nil {
		return billing.ProviderSubscription{}, err
	}
	out := billing.ProviderSubscription{
		ID:                sub.ID,
		Status:            status,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}
	if sub.Customer != nil {
		out.CustomerID = sub.Customer.ID
	}
	if sub.StartDate > 0 {
		out.StartedAt = time.Unix(sub.StartDate, 0).UTC()
	}
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0] != nil {
		out.CurrentPeriodEnd = time.Unix(sub.Items.Data[0].CurrentPeriodEnd, 0).UTC()
	}
	return out, nil
}

func mapStatus(status stripesdk.SubscriptionStatus) (billing.Status, error) {
	switch string(status) {
	case "incomplete":
		return billing.StatusPending, nil
	case "active":
		return billing.StatusActive, nil
	case "past_due", "paused":
		return billing.StatusPastDue, nil
	case "unpaid", "incomplete_expired", "canceled":
		return billing.StatusCanceled, nil
	}
	return "", fmt.Errorf("%w: unknown subscription status %q", billing.ErrProviderRejected, status)
}
