package accountdeletion

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
	"legnaris/backend/internal/user"
	"legnaris/backend/shared/apperr"
)

const QueueName = "users_deleting"

const Interval = time.Minute

const MsgDepsRequired = "account deletion dependencies are required"

type BillingSettler interface {
	Settle(ctx context.Context, userID string) error
}

type CampaignSettler interface {
	SettleAccount(ctx context.Context, userID string) error
}

type Deps struct {
	Store     user.DeletionStore
	Cascade   *Cascade
	Tx        Transactor
	Billing   BillingSettler
	Campaigns CampaignSettler
}

type store struct {
	deps Deps
}

func (s store) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[user.User], error) {
	accounts, err := s.deps.Store.ClaimDeleting(ctx, claimer, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[user.User], 0, len(accounts))
	for _, account := range accounts {
		jobs = append(jobs, sweeper.Job[user.User]{ID: account.ID, Attempts: account.DeletionAttempts, Payload: account})
	}
	return jobs, nil
}

func (s store) Reschedule(ctx context.Context, id string, _, nextAt time.Time, reason string) error {
	return s.deps.Store.RescheduleDeleting(ctx, id, nextAt, reason)
}

func (s store) MarkDead(ctx context.Context, id string, _ time.Time, reason string) error {
	return s.deps.Store.MarkDeletionDead(ctx, id, reason)
}

func (s store) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.deps.Store.OldestDeleting(ctx)
}

func NewQueue(deps Deps, attemptCap, batch int) (sweeper.Queue, error) {
	if deps.Billing == nil || deps.Campaigns == nil {
		return sweeper.Queue{}, apperr.NewError(apperr.CodeInternal, MsgDepsRequired, nil)
	}
	s := store{deps: deps}
	return sweeper.NewQueue(sweeper.QueueSpec[user.User]{
		Name:       QueueName,
		Interval:   Interval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      s,
		Handle: func(ctx context.Context, job sweeper.Job[user.User], _ time.Time) error {
			if err := deps.Billing.Settle(ctx, job.ID); err != nil {
				return err
			}
			if err := deps.Campaigns.SettleAccount(ctx, job.ID); err != nil {
				return err
			}
			return deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
				return deps.Cascade.Remove(txCtx, job.ID)
			})
		},
	})
}
