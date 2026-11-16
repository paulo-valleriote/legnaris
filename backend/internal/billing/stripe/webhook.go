package stripe

import (
	"fmt"

	"legnaris/backend/internal/billing"
	sharedjson "legnaris/backend/shared/json"

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
	if err := sharedjson.DecodeBytes(event.Data.Raw, &session); err != nil {
		return billing.Notification{}, fmt.Errorf("%w: decode checkout session: %w", billing.ErrProviderRejected, err)
	}
	if string(session.Mode) != checkoutModeSubscription || session.Subscription == nil {
		return ignored, nil
	}
	return subscriptionNotification(event.ID, session.Metadata, session.Subscription.ID, session.Customer), nil
}

func parseSubscription(event stripesdk.Event, ignored billing.Notification) (billing.Notification, error) {
	var sub stripesdk.Subscription
	if err := sharedjson.DecodeBytes(event.Data.Raw, &sub); err != nil {
		return billing.Notification{}, fmt.Errorf("%w: decode subscription: %w", billing.ErrProviderRejected, err)
	}
	if sub.ID == "" {
		return ignored, nil
	}
	return subscriptionNotification(event.ID, sub.Metadata, sub.ID, sub.Customer), nil
}

func subscriptionNotification(eventID string, metadata map[string]string, subscriptionID string, customer *stripesdk.Customer) billing.Notification {
	n := billing.Notification{
		ID:                     eventID,
		Kind:                   billing.KindSubscriptionChanged,
		UserID:                 metadata[subscriptionMetadataUserKey],
		ProviderSubscriptionID: subscriptionID,
	}
	if customer != nil {
		n.ProviderCustomerID = customer.ID
	}
	return n
}
