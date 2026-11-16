package generation

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
)

const ReapQueueName = "generation_lease"

type ReapDeps struct {
	Store LeaseStore
	Quota Quota
	Tx    Transactor
}

type reapStore struct {
	deps ReapDeps
}

func (r reapStore) Claim(ctx context.Context, _ string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[Document], error) {
	docs, err := r.deps.Store.ClaimExpired(ctx, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[Document], 0, len(docs))
	for _, doc := range docs {
		jobs = append(jobs, sweeper.Job[Document]{ID: doc.ID, Attempts: doc.ReapAttempts, Payload: doc})
	}
	return jobs, nil
}

func (r reapStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, _ string) error {
	return r.deps.Store.RescheduleExpired(ctx, id, nextAt)
}

func (r reapStore) MarkDead(ctx context.Context, id string, now time.Time, _ string) error {
	return r.fail(ctx, id, now)
}

func (r reapStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return r.deps.Store.OldestRunning(ctx)
}

func (r reapStore) fail(ctx context.Context, id string, now time.Time) error {
	return r.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		doc, failed, err := r.deps.Store.FailExpired(txCtx, id, now, ErrorCodeLeaseExpired)
		if err != nil {
			return err
		}
		if !failed {
			return nil
		}
		return RefundQuota(txCtx, r.deps.Quota, doc)
	})
}

func NewReapQueue(deps ReapDeps, attemptCap, batch int) (sweeper.Queue, error) {
	store := reapStore{deps: deps}
	return sweeper.NewQueue(sweeper.QueueSpec[Document]{
		Name:       ReapQueueName,
		Interval:   ReapInterval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      store,
		Handle: func(ctx context.Context, job sweeper.Job[Document], now time.Time) error {
			return store.fail(ctx, job.ID, now)
		},
	})
}
