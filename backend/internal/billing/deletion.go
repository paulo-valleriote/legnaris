package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/sweeper"
)

const (
	FiscalRetentionYears = 5
	cancelKeyPrefix      = "cancel:"
	scrubKeyPrefix       = "scrub:"
)

type DeletionDeps struct {
	Provider Provider
	Ledger   Ledger
}

type Deletion struct {
	deps DeletionDeps
}

func NewDeletion(deps DeletionDeps) *Deletion {
	return &Deletion{deps: deps}
}

func RetentionUntil(at time.Time) time.Time {
	return at.AddDate(FiscalRetentionYears, 0, 0)
}

func (d *Deletion) Settle(ctx context.Context, userID string) error {
	subs, err := d.deps.Ledger.SubscriptionsOf(ctx, userID)
	if err != nil {
		return fmt.Errorf("load subscriptions for deletion: %w", err)
	}

	customers := make([]string, 0, len(subs))
	seen := make(map[string]bool, len(subs))
	for _, sub := range subs {
		if sub.ProviderCustomerID != "" && !seen[sub.ProviderCustomerID] {
			seen[sub.ProviderCustomerID] = true
			customers = append(customers, sub.ProviderCustomerID)
		}
		if sub.ProviderSubID == "" || sub.Status == StatusCanceled {
			continue
		}
		if err := d.cancelNow(ctx, sub); err != nil {
			return err
		}
	}

	for _, customerID := range customers {
		req := ScrubRequest{IdempotencyKey: scrubKeyPrefix + customerID, CustomerID: customerID}
		if err := d.deps.Provider.ScrubCustomer(ctx, req); err != nil {
			return providerFailure(err)
		}
	}
	return nil
}

func (d *Deletion) cancelNow(ctx context.Context, sub Subscription) error {
	remote, err := d.deps.Provider.GetSubscription(ctx, sub.ProviderSubID)
	if err != nil {
		return providerFailure(err)
	}
	if remote.Status == StatusCanceled {
		return nil
	}
	req := CancelRequest{IdempotencyKey: cancelKeyPrefix + sub.ID, ProviderSubscriptionID: sub.ProviderSubID}
	if err := d.deps.Provider.CancelNow(ctx, req); err != nil {
		return providerFailure(err)
	}
	return nil
}

func providerFailure(err error) error {
	if errors.Is(err, ErrProviderRejected) {
		return fmt.Errorf("%w: %w", sweeper.ErrPermanent, err)
	}
	return err
}
