package stripe

import (
	"encoding/json"
	"fmt"

	"legnaris/backend/internal/billing"

	stripesdk "github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

const (
	eventCheckoutCompleted      = "checkout.session.completed"
	eventSubscriptionCreated    = "customer.subscription.created"
	eventSubscriptionUpdated    = "customer.subscription.updated"
	eventSubscriptionDeleted    = "customer.subscription.deleted"
	checkoutModeSubscription    = "subscription"
	subscriptionMetadataUserKey = "user_id"
)

func (p *Provider) ParseWebhook(payload []byte, signature string) (billing.Notification, error) {
	if p.webhookSecret == "" {
		return billing.Notification{}, billing.ErrProviderUnavailable
	}
	event, err := webhook.ConstructEvent(payload, signature, p.webhookSecret)
	if err != nil {
		return billing.Notification{}, fmt.Errorf("%w: %w", billing.ErrInvalidSignature, err)
	}

	ignored := billing.Notification{ID: event.ID, Kind: billing.KindIgnored}
	switch event.Type {
	case eventCheckoutCompleted:
		return parseCheckout(event, ignored)
	case eventSubscriptionCreated, eventSubscriptionUpdated, eventSubscriptionDeleted:
		return parseSubscription(event, ignored)
	}
	return ignored, nil
}

func parseCheckout(event stripesdk.Event, ignored billing.Notification) (billing.Notification, error) {
	var session stripesdk.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return billing.Notification{}, fmt.Errorf("%w: decode checkout session: %w", billing.ErrProviderRejected, err)
	}
	if string(session.Mode) != checkoutModeSubscription || session.Subscription == nil {
		return ignored, nil
	}
	n := billing.Notification{
		ID:                     event.ID,
		Kind:                   billing.KindSubscriptionChanged,
		UserID:                 session.Metadata[subscriptionMetadataUserKey],
		ProviderSubscriptionID: session.Subscription.ID,
	}
	if session.Customer != nil {
		n.ProviderCustomerID = session.Customer.ID
	}
	return n, nil
}

func parseSubscription(event stripesdk.Event, ignored billing.Notification) (billing.Notification, error) {
	var sub stripesdk.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return billing.Notification{}, fmt.Errorf("%w: decode subscription: %w", billing.ErrProviderRejected, err)
	}
	if sub.ID == "" {
		return ignored, nil
	}
	n := billing.Notification{
		ID:                     event.ID,
		Kind:                   billing.KindSubscriptionChanged,
		UserID:                 sub.Metadata[subscriptionMetadataUserKey],
		ProviderSubscriptionID: sub.ID,
	}
	if sub.Customer != nil {
		n.ProviderCustomerID = sub.Customer.ID
	}
	return n, nil
}
