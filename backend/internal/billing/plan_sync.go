package billing

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/db"
)

type PlanSyncLedger interface {
	AccountPlan(ctx context.Context, userID string) (AccountPlan, error)
	SubscriptionsOf(ctx context.Context, userID string) ([]Subscription, error)
	SetPlan(ctx context.Context, userID, plan string) (int64, error)
}

type PlanSyncDeps struct {
	Ledger PlanSyncLedger
	Outbox Outbox
	Tx     Transactor
}

func (d PlanSyncDeps) SyncPlan(ctx context.Context, userID string, now time.Time) error {
	account, err := d.Ledger.AccountPlan(ctx, userID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	_, err = SyncPlan(ctx, d, userID, account, now)
	return err
}

func SyncPlan(ctx context.Context, deps PlanSyncDeps, userID string, account AccountPlan, now time.Time) (bool, error) {
	if !account.Exists || account.Deleting {
		return false, nil
	}

	subs, err := deps.Ledger.SubscriptionsOf(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("load subscriptions: %w", err)
	}
	derived := DerivePlanID(subs, now)
	current := account.Plan
	if current == "" {
		current = plan.IDFree
	}
	if derived == current {
		return false, nil
	}

	version, err := deps.Ledger.SetPlan(ctx, userID, derived)
	if err != nil {
		return false, fmt.Errorf("set plan: %w", err)
	}
	entry, err := outbox.NewEntry(events.Event{
		Type:             PlanChangedEvent,
		AggregateType:    userAggregate,
		AggregateID:      userID,
		AggregateVersion: version,
		Data:             map[string]string{"from": current, "to": derived},
	}, now)
	if err != nil {
		return false, fmt.Errorf("plan changed entry: %w", err)
	}
	if err := deps.Outbox.Insert(ctx, entry); err != nil {
		return false, fmt.Errorf("insert plan changed: %w", err)
	}
	if err := deps.Tx.Step(ctx, db.StepAfterOutboxInsert); err != nil {
		return false, err
	}
	return true, nil
}
