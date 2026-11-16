package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/events/outbox"
	"legnaris/backend/internal/plan"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/idgen"
)

const (
	PlanChangedEvent   = "plan.changed"
	userAggregate      = "user"
	planChangedVersion = 1
)

type ConsumerDeps struct {
	Provider Provider
	Ledger   Ledger
	Tx       Transactor
	Outbox   Outbox
	Now      func() time.Time
}

type Consumer struct {
	deps ConsumerDeps
}

func NewConsumer(deps ConsumerDeps) *Consumer {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Consumer{deps: deps}
}

func (c *Consumer) Handle(ctx context.Context, job sweeper.Job[Event], now time.Time) error {
	ev := job.Payload
	defer func() { _ = c.deps.Ledger.ReleaseUser(ctx, ev.UserID, ev.Claimer) }()

	account, err := c.deps.Ledger.AccountPlan(ctx, ev.UserID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	if !account.Exists || account.Deleting {
		return c.deps.Ledger.MarkEventProcessed(ctx, ev.ID, ReasonIgnored, now)
	}

	snap, err := c.deps.Provider.GetSubscription(ctx, ev.ProviderSubscriptionID)
	if err != nil {
		if errors.Is(err, ErrProviderRejected) {
			return fmt.Errorf("%w: %w", sweeper.ErrPermanent, err)
		}
		return fmt.Errorf("read subscription: %w", err)
	}
	readAt := c.deps.Now()

	return c.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := c.deps.Tx.Step(txCtx, db.StepAfterProviderRead); err != nil {
			return err
		}
		return c.apply(txCtx, ev, snap, readAt, now)
	})
}

func (c *Consumer) apply(ctx context.Context, ev Event, snap ProviderSubscription, readAt, now time.Time) error {
	account, err := c.deps.Ledger.AccountPlan(ctx, ev.UserID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	if !account.Exists || account.Deleting {
		return c.deps.Ledger.MarkEventProcessed(ctx, ev.ID, ReasonIgnored, now)
	}

	if err := c.upsertSubscription(ctx, ev.UserID, snap, readAt, now); err != nil {
		return err
	}
	if err := c.deps.Tx.Step(ctx, db.StepAfterEntityWrite); err != nil {
		return err
	}

	if err := c.syncPlan(ctx, ev.UserID, account, now); err != nil {
		return err
	}
	return c.deps.Ledger.MarkEventProcessed(ctx, ev.ID, "", now)
}

func (c *Consumer) upsertSubscription(ctx context.Context, userID string, snap ProviderSubscription, readAt, now time.Time) error {
	target, found, err := c.deps.Ledger.FindTarget(ctx, userID, snap.ID)
	if err != nil {
		return fmt.Errorf("find subscription: %w", err)
	}
	synced := readAt
	if !found {
		id, err := idgen.New()
		if err != nil {
			return fmt.Errorf("new subscription id: %w", err)
		}
		target = Subscription{
			ID:        id,
			UserID:    userID,
			PlanID:    plan.IDPaid,
			Provider:  ProviderStripe,
			CreatedAt: now,
		}
	}
	target.ProviderSubID = snap.ID
	target.ProviderCustomerID = snap.CustomerID
	target.Status = snap.Status
	period := snap.CurrentPeriodEnd
	target.CurrentPeriodEnd = &period
	target.CancelAtPeriodEnd = snap.CancelAtPeriodEnd
	target.ProviderSyncedAt = &synced
	target.UpdatedAt = now

	if !found {
		if err := c.deps.Ledger.InsertSubscription(ctx, target); err != nil {
			return fmt.Errorf("insert subscription: %w", err)
		}
		return nil
	}
	if _, err := c.deps.Ledger.ApplySubscription(ctx, target, readAt); err != nil {
		return fmt.Errorf("apply subscription: %w", err)
	}
	return nil
}

func (c *Consumer) syncPlan(ctx context.Context, userID string, account AccountPlan, now time.Time) error {
	subs, err := c.deps.Ledger.SubscriptionsOf(ctx, userID)
	if err != nil {
		return fmt.Errorf("load subscriptions: %w", err)
	}
	derived := DerivePlanID(subs)
	current := account.Plan
	if current == "" {
		current = plan.IDFree
	}
	if derived == current {
		return nil
	}

	version, err := c.deps.Ledger.SetPlan(ctx, userID, derived)
	if err != nil {
		return fmt.Errorf("set plan: %w", err)
	}
	entry, err := outbox.NewEntry(events.Event{
		Type:             PlanChangedEvent,
		AggregateType:    userAggregate,
		AggregateID:      userID,
		AggregateVersion: version,
		Data:             map[string]string{"from": current, "to": derived},
	}, now)
	if err != nil {
		return fmt.Errorf("plan changed entry: %w", err)
	}
	if err := c.deps.Outbox.Insert(ctx, entry); err != nil {
		return fmt.Errorf("insert plan changed: %w", err)
	}
	return c.deps.Tx.Step(ctx, db.StepAfterOutboxInsert)
}
