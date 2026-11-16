package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/logger"
)

const (
	ReconcileInterval      = 24 * time.Hour
	stuckReceivedAfter     = time.Hour
	CorrectionSubscription = "subscription"
	CorrectionPlan         = "plan"
	CorrectionEvent        = "event"
)

type ReconcileObserver interface {
	Corrected(kind string, count int)
}

type ReconcileDeps struct {
	ConsumerDeps
	Claimer  string
	Observer ReconcileObserver
	Log      logger.StructuredLogger
}

type Reconciler struct {
	deps     ReconcileDeps
	consumer *Consumer
}

func NewReconciler(deps ReconcileDeps) *Reconciler {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Reconciler{deps: deps, consumer: NewConsumer(deps.ConsumerDeps)}
}

func (r *Reconciler) RunOnce(ctx context.Context, now time.Time) error {
	var errs []error

	requeued, err := r.deps.Ledger.RequeueEvents(ctx, now, now.Add(-stuckReceivedAfter))
	if err != nil {
		errs = append(errs, fmt.Errorf("requeue events: %w", err))
	}
	if requeued > 0 {
		r.deps.Observer.Corrected(CorrectionEvent, requeued)
	}

	subs, err := r.deps.Ledger.LiveSubscriptions(ctx)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("load live subscriptions: %w", err))...)
	}
	for _, sub := range subs {
		corr, err := r.reconcileOne(ctx, sub, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("reconcile subscription %s: %w", sub.ID, err))
			continue
		}
		if corr.Subscription {
			r.deps.Observer.Corrected(CorrectionSubscription, 1)
		}
		if corr.Plan {
			r.deps.Observer.Corrected(CorrectionPlan, 1)
		}
	}
	return errors.Join(errs...)
}

func (r *Reconciler) reconcileOne(ctx context.Context, sub Subscription, now time.Time) (Corrections, error) {
	account, err := r.deps.Ledger.AccountPlan(ctx, sub.UserID)
	if err != nil {
		return Corrections{}, fmt.Errorf("load account: %w", err)
	}
	if !account.Exists || account.Deleting {
		return Corrections{}, nil
	}

	acquired, err := r.deps.Ledger.AcquireUser(ctx, sub.UserID, r.deps.Claimer, now, sweeper.Lease)
	if err != nil {
		return Corrections{}, fmt.Errorf("acquire user lease: %w", err)
	}
	if !acquired {
		return Corrections{}, nil
	}
	defer func() { _ = r.deps.Ledger.ReleaseUser(ctx, sub.UserID, r.deps.Claimer) }()

	snap, err := r.deps.Provider.GetSubscription(ctx, sub.ProviderSubID)
	if err != nil {
		return Corrections{}, fmt.Errorf("read subscription: %w", err)
	}
	readAt := r.deps.Now()

	var corr Corrections
	err = r.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.deps.Tx.Step(txCtx, db.StepAfterProviderRead); err != nil {
			return err
		}
		_, c, err := r.consumer.writeSnapshot(txCtx, sub.UserID, snap, readAt, now)
		corr = c
		return err
	})
	if err != nil {
		return Corrections{}, err
	}
	return corr, nil
}

func (r *Reconciler) Run(ctx context.Context) error {
	for {
		if err := r.RunOnce(ctx, r.deps.Now()); err != nil {
			r.deps.Log.Error(ctx, "billing reconciliation pass failed", logger.Attribute{Key: "error", Value: err.Error()})
		}
		if ctx.Err() != nil {
			return nil
		}

		timer := time.NewTimer(ReconcileInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
