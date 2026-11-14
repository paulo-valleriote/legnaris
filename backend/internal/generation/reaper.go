package generation

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
)

// ReapQueueName is the name of the lease reaper queue, the label of its metrics.
const ReapQueueName = "generation_lease"

// ReapDeps are the collaborators of the lease reaper queue.
type ReapDeps struct {
	Store LeaseStore
	Quota Quota
	Tx    Transactor
}

// reapStore is the sweeper's view of the running documents whose worker lease ended.
// A give-up fails the job and refunds its quota in one transaction.
type reapStore struct {
	deps ReapDeps
}

// Claim leases the expired running documents for claimer.
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

// Reschedule makes a document due for the reaper again at nextAt.
func (r reapStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, _ string) error {
	return r.deps.Store.RescheduleExpired(ctx, id, nextAt)
}

// MarkDead gives up on a document the reaper cannot settle: it fails and refunds.
func (r reapStore) MarkDead(ctx context.Context, id string, now time.Time, _ string) error {
	return r.fail(ctx, id, now)
}

// OldestPending returns the creation time of the oldest running document.
func (r reapStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return r.deps.Store.OldestRunning(ctx)
}

// fail moves an expired running document to failed and refunds its quota. A document
// that is no longer expired-and-running is left alone, so a repeat is a no-op.
func (r reapStore) fail(ctx context.Context, id string, now time.Time) error {
	return r.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		doc, failed, err := r.deps.Store.FailExpired(txCtx, id, now, ErrorCodeLeaseExpired)
		if err != nil {
			return err
		}
		if !failed {
			return nil
		}
		return refund(txCtx, r.deps.Quota, doc)
	})
}

// NewReapQueue builds the sweeper queue that fails running generations whose worker
// lease ended, and refunds them. It is registered in the relay's sweeper table next
// to the republish queue (ADR 0013, LGN-230).
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
