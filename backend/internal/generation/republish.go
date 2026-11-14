package generation

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/sweeper"
)

// RepublishQueueName is the name of the sweeper queue, the label of its metrics.
const RepublishQueueName = "generation"

// RepublishInterval is how often the sweeper looks for queued documents whose
// publish did not reach the broker. A user waits on the screen, so this is far
// shorter than the email relay's cadence (ADR 0013).
const RepublishInterval = 10 * time.Second

// RepublishDeps are the collaborators of the republish queue.
type RepublishDeps struct {
	Store     PublishStore
	Publisher Publisher
	Quota     Quota
	Tx        Transactor
	Now       func() time.Time
}

// republishStore is the sweeper's view of the queued documents. A give-up refunds
// the quota in the same transaction that fails the job, so a dead job never keeps
// its allowance.
type republishStore struct {
	deps RepublishDeps
}

// Claim leases due documents for claimer. Each Job carries the document, so the
// handler publishes without a second read.
func (r republishStore) Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[Document], error) {
	docs, err := r.deps.Store.ClaimUnpublished(ctx, claimer, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[Document], 0, len(docs))
	for _, doc := range docs {
		jobs = append(jobs, sweeper.Job[Document]{ID: doc.ID, Attempts: doc.PublishAttempts, Payload: doc})
	}
	return jobs, nil
}

// Reschedule counts one failed publish and makes the document due again at nextAt.
func (r republishStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, _ string) error {
	return r.deps.Store.ReschedulePublish(ctx, id, nextAt)
}

// MarkDead gives up on a document that never reached the broker: the job fails,
// its in-progress slot is released and its quota unit goes back, in one transaction.
func (r republishStore) MarkDead(ctx context.Context, id string, _ time.Time, _ string) error {
	return r.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		doc, failed, err := r.deps.Store.FailQueued(txCtx, id, ErrorCodePublishFailed)
		if err != nil {
			return err
		}
		if !failed {
			return nil
		}
		return refund(txCtx, r.deps.Quota, doc)
	})
}

// OldestPending returns the creation time of the oldest queued document, so the
// sweeper can report the backlog age.
func (r republishStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return r.deps.Store.OldestQueued(ctx)
}

// refund returns the quota unit a generation took, to the allowance it came from.
func refund(ctx context.Context, quota Quota, doc Document) error {
	switch doc.Quota.Kind {
	case ReservationDaily:
		return quota.RefundDaily(ctx, doc.UserID, doc.Quota.DayUTC)
	case ReservationTrial:
		return quota.RefundTrial(ctx, doc.UserID)
	default:
		return fmt.Errorf("refund generation %s: unknown reservation kind %q", doc.ID, doc.Quota.Kind)
	}
}

// NewRepublishQueue builds the sweeper queue that republishes queued generations.
// The handler publishes and returns; the queue reschedules a failed publish and
// gives up after attemptCap tries, which refunds the job.
func NewRepublishQueue(deps RepublishDeps, attemptCap, batch int) (sweeper.Queue, error) {
	store := republishStore{deps: deps}
	return sweeper.NewQueue(sweeper.QueueSpec[Document]{
		Name:       RepublishQueueName,
		Interval:   RepublishInterval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      store,
		Handle: func(ctx context.Context, job sweeper.Job[Document], _ time.Time) error {
			return deps.Publisher.Publish(ctx, Message{ID: job.Payload.ID, Kind: job.Payload.Kind})
		},
	})
}
