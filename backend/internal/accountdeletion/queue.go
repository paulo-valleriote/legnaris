package accountdeletion

import (
	"context"
	"time"

	"legnaris/backend/internal/sweeper"
	"legnaris/backend/internal/user"
)

// QueueName is the name of the deleting-accounts queue, the label of its metrics.
const QueueName = "users_deleting"

// Interval is how often the sweeper claims deleting accounts. A deletion is not
// user-facing latency, so a minute is enough.
const Interval = time.Minute

// Deps are the collaborators of the deleting-accounts queue.
type Deps struct {
	Store   user.DeletionStore
	Cascade *Cascade
	Tx      Transactor
}

// store adapts the user deletion store to the sweeper's persistence port.
type store struct {
	deps Deps
}

// Claim leases the deleting accounts for claimer.
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

// Reschedule makes a deleting account eligible again at nextAt and counts the attempt.
func (s store) Reschedule(ctx context.Context, id string, _, nextAt time.Time, reason string) error {
	return s.deps.Store.RescheduleDeleting(ctx, id, nextAt, reason)
}

// MarkDead stops the sweeper from claiming an account whose deletion keeps failing.
func (s store) MarkDead(ctx context.Context, id string, _ time.Time, reason string) error {
	return s.deps.Store.MarkDeletionDead(ctx, id, reason)
}

// OldestPending returns the time the oldest deleting account was marked.
func (s store) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.deps.Store.OldestDeleting(ctx)
}

// NewQueue builds the sweeper queue that removes deleting accounts. Each attempt
// runs the cascade in one transaction, so a failed attempt leaves the account
// intact and the next attempt starts from the same state.
func NewQueue(deps Deps, attemptCap, batch int) (sweeper.Queue, error) {
	s := store{deps: deps}
	return sweeper.NewQueue(sweeper.QueueSpec[user.User]{
		Name:       QueueName,
		Interval:   Interval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      s,
		Handle: func(ctx context.Context, job sweeper.Job[user.User], _ time.Time) error {
			return deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
				return deps.Cascade.Remove(txCtx, job.ID)
			})
		},
	})
}
