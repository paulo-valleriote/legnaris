package billing

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/db"
	"legnaris/backend/shared/logger"
)

const (
	CompExpiryQueueName = "billing_comp_expiry"
	CompExpiryInterval  = time.Hour
	CompExpiryBatchSize = 100
	compExpiryJobID     = "comp_expiry"
)

type CompExpiryLedger interface {
	PlanSyncLedger
	ExpiredComps(ctx context.Context, now time.Time, limit int) ([]Subscription, error)
	CancelComp(ctx context.Context, id string, now time.Time) (bool, error)
}

type CompExpiryRunner interface {
	Expire(ctx context.Context, now time.Time) (int, error)
}

type CompExpiryDeps struct {
	Ledger CompExpiryLedger
	Outbox Outbox
	Tx     Transactor
	Log    logger.StructuredLogger
}

type CompExpiry struct {
	deps CompExpiryDeps
}

func NewCompExpiry(deps CompExpiryDeps) *CompExpiry {
	return &CompExpiry{deps: deps}
}

func (c *CompExpiry) Expire(ctx context.Context, now time.Time) (int, error) {
	total := 0
	for {
		rows, err := c.deps.Ledger.ExpiredComps(ctx, now, CompExpiryBatchSize)
		if err != nil {
			return total, fmt.Errorf("load expired comps: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		progressed := false
		for _, row := range rows {
			canceled, err := c.expireOne(ctx, row, now)
			if err != nil {
				return total, fmt.Errorf("expire comp %s: %w", row.ID, err)
			}
			if canceled {
				total++
				progressed = true
			}
		}
		if len(rows) < CompExpiryBatchSize || !progressed {
			break
		}
	}
	if c.deps.Log != nil {
		c.deps.Log.Info(ctx, "comp subscriptions expired", logger.Attribute{Key: "count", Value: strconv.Itoa(total)})
	}
	return total, nil
}

func (c *CompExpiry) expireOne(ctx context.Context, row Subscription, now time.Time) (bool, error) {
	canceled := false
	err := c.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		applied, err := c.deps.Ledger.CancelComp(txCtx, row.ID, now)
		if err != nil {
			return err
		}
		canceled = applied
		if applied {
			if err := c.deps.Tx.Step(txCtx, db.StepAfterEntityWrite); err != nil {
				return err
			}
		}
		account, err := c.deps.Ledger.AccountPlan(txCtx, row.UserID)
		if err != nil {
			return fmt.Errorf("load account: %w", err)
		}
		_, err = SyncPlan(txCtx, PlanSyncDeps{
			Ledger: c.deps.Ledger,
			Outbox: c.deps.Outbox,
			Tx:     c.deps.Tx,
		}, row.UserID, account, now)
		return err
	})
	if err != nil {
		return false, err
	}
	return canceled, nil
}

type compExpiryStore struct {
	runner CompExpiryRunner
}

func (s compExpiryStore) Claim(context.Context, string, time.Time, time.Duration, int) ([]sweeper.Job[struct{}], error) {
	return []sweeper.Job[struct{}]{{ID: compExpiryJobID}}, nil
}

func (s compExpiryStore) Reschedule(context.Context, string, time.Time, time.Time, string) error {
	return nil
}

func (s compExpiryStore) MarkDead(context.Context, string, time.Time, string) error {
	return nil
}

func (s compExpiryStore) OldestPending(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func NewCompExpiryQueue(runner CompExpiryRunner) (sweeper.Queue, error) {
	return sweeper.NewQueue(sweeper.QueueSpec[struct{}]{
		Name:       CompExpiryQueueName,
		Interval:   CompExpiryInterval,
		AttemptCap: 1,
		Batch:      1,
		Store:      compExpiryStore{runner: runner},
		Handle: func(ctx context.Context, _ sweeper.Job[struct{}], now time.Time) error {
			_, err := runner.Expire(ctx, now)
			return err
		},
	})
}
