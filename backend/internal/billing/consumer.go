package billing

import (
	"context"
	"fmt"
	"time"

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
	Provider   Provider
	Ledger     Ledger
	Tx         Transactor
	Outbox     Outbox
	CompExpiry CompExpiryRunner
	Now        func() time.Time
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

	readAt := c.deps.Now()
	snap, err := c.deps.Provider.GetSubscription(ctx, ev.ProviderSubscriptionID)
	if err != nil {
		return fmt.Errorf("read subscription: %w", providerFailure(err))
	}

	return c.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := c.deps.Tx.Step(txCtx, db.StepAfterProviderRead); err != nil {
			return err
		}
		written, _, err := c.writeSnapshot(txCtx, ev.UserID, snap, readAt, now)
		if err != nil {
			return err
		}
		if !written {
			return c.deps.Ledger.MarkEventProcessed(txCtx, ev.ID, ReasonIgnored, now)
		}
		return c.deps.Ledger.MarkEventProcessed(txCtx, ev.ID, "", now)
	})
}

type Corrections struct {
	Subscription bool
	Plan         bool
}

func (c *Consumer) writeSnapshot(ctx context.Context, userID string, snap ProviderSubscription, readAt, now time.Time) (bool, Corrections, error) {
	account, err := c.deps.Ledger.AccountPlan(ctx, userID)
	if err != nil {
		return false, Corrections{}, fmt.Errorf("load account: %w", err)
	}
	if !account.Exists || account.Deleting {
		return false, Corrections{}, nil
	}

	subChanged, err := c.upsertSubscription(ctx, userID, snap, readAt, now)
	if err != nil {
		return false, Corrections{}, err
	}
	if err := c.deps.Tx.Step(ctx, db.StepAfterEntityWrite); err != nil {
		return false, Corrections{}, err
	}

	planChanged, err := c.syncPlan(ctx, userID, account, now)
	if err != nil {
		return false, Corrections{}, err
	}
	return true, Corrections{Subscription: subChanged, Plan: planChanged}, nil
}

func (c *Consumer) upsertSubscription(ctx context.Context, userID string, snap ProviderSubscription, readAt, now time.Time) (bool, error) {
	target, found, err := c.deps.Ledger.FindTarget(ctx, userID, snap.ID)
	if err != nil {
		return false, fmt.Errorf("find subscription: %w", err)
	}
	if !found {
		id, err := idgen.New()
		if err != nil {
			return false, fmt.Errorf("new subscription id: %w", err)
		}
		target = Subscription{
			ID:        id,
			UserID:    userID,
			PlanID:    plan.IDPaid,
			Provider:  ProviderStripe,
			CreatedAt: now,
		}
	}
	before := target
	synced := readAt
	period := snap.CurrentPeriodEnd
	target.ProviderSubID = snap.ID
	target.ProviderCustomerID = snap.CustomerID
	target.Status = snap.Status
	target.CurrentPeriodEnd = &period
	target.CancelAtPeriodEnd = snap.CancelAtPeriodEnd
	target.ProviderSyncedAt = &synced
	if target.FirstPaidAt == nil && snap.Status == StatusActive {
		paidAt := firstPaymentAt(snap, readAt)
		target.FirstPaidAt = &paidAt
	}
	target.UpdatedAt = now

	if !found {
		if err := c.deps.Ledger.InsertSubscription(ctx, target); err != nil {
			return false, fmt.Errorf("insert subscription: %w", err)
		}
		return true, nil
	}
	applied, err := c.deps.Ledger.ApplySubscription(ctx, target, readAt)
	if err != nil {
		return false, fmt.Errorf("apply subscription: %w", err)
	}
	return applied && subscriptionDiffers(before, target), nil
}

func firstPaymentAt(snap ProviderSubscription, readAt time.Time) time.Time {
	if snap.StartedAt.IsZero() {
		return readAt
	}
	return snap.StartedAt
}

func subscriptionDiffers(before, after Subscription) bool {
	return before.ProviderSubID != after.ProviderSubID ||
		before.ProviderCustomerID != after.ProviderCustomerID ||
		before.Status != after.Status ||
		before.CancelAtPeriodEnd != after.CancelAtPeriodEnd ||
		!sameInstant(before.CurrentPeriodEnd, after.CurrentPeriodEnd)
}

func sameInstant(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func (c *Consumer) syncPlan(ctx context.Context, userID string, account AccountPlan, now time.Time) (bool, error) {
	return SyncPlan(ctx, PlanSyncDeps{
		Ledger: c.deps.Ledger,
		Outbox: c.deps.Outbox,
		Tx:     c.deps.Tx,
	}, userID, account, now)
}
