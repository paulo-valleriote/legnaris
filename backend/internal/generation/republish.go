package generation

import (
	"context"
	"fmt"
	"time"

	"legnaris/backend/internal/sweeper"
)

const RepublishQueueName = "generation"

const RepublishInterval = 10 * time.Second

type RepublishDeps struct {
	Store     PublishStore
	Publisher Publisher
	Quota     Quota
	Tx        Transactor
	Now       func() time.Time
}

type republishStore struct {
	deps RepublishDeps
}

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

func (r republishStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, _ string) error {
	return r.deps.Store.ReschedulePublish(ctx, id, nextAt)
}

func (r republishStore) MarkDead(ctx context.Context, id string, _ time.Time, _ string) error {
	return r.deps.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		doc, failed, err := r.deps.Store.FailQueued(txCtx, id, ErrorCodePublishFailed)
		if err != nil {
			return err
		}
		if !failed {
			return nil
		}
		return RefundQuota(txCtx, r.deps.Quota, doc)
	})
}

func (r republishStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return r.deps.Store.OldestQueued(ctx)
}

func RefundQuota(ctx context.Context, quota Quota, doc Document) error {
	switch doc.Quota.Kind {
	case ReservationDaily:
		return quota.RefundDaily(ctx, doc.UserID, doc.Quota.DayUTC)
	case ReservationTrial:
		return quota.RefundTrial(ctx, doc.UserID)
	default:
		return fmt.Errorf("refund generation %s: unknown reservation kind %q", doc.ID, doc.Quota.Kind)
	}
}

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
