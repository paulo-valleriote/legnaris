package billing

import (
	"context"
	"fmt"
	"time"
)

type SubscriptionLister interface {
	SubscriptionsOf(ctx context.Context, userID string) ([]Subscription, error)
}

type PlanReader struct {
	subscriptions SubscriptionLister
	now           func() time.Time
}

func NewPlanReader(subscriptions SubscriptionLister, now func() time.Time) *PlanReader {
	if now == nil {
		now = time.Now
	}
	return &PlanReader{subscriptions: subscriptions, now: now}
}

func (r *PlanReader) EffectivePlanID(ctx context.Context, userID string) (string, error) {
	subs, err := r.subscriptions.SubscriptionsOf(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("load subscriptions: %w", err)
	}
	return DerivePlanID(subs, r.now()), nil
}
