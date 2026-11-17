package campaign

import (
	"context"
	"errors"
	"time"

	"legnaris/backend/internal/events"
	"legnaris/backend/internal/sweeper"
	"legnaris/backend/shared/apperr"
)

const (
	DeletionQueueName   = "campaigns_deleting"
	DeletionInterval    = 5 * time.Minute
	DeletionUnlinkBatch = 100
)

const MsgOwnerOnlyDelete = "only the campaign owner can delete the campaign"

type DeleteCommand struct {
	CampaignID string `json:"-"`
	ActorID    string `json:"-"`
}

type OwnershipReleaser interface {
	ReleaseOwnership(ctx context.Context, userID string) error
}

type DeletionDeps struct {
	Store     DeletionStore
	Links     SheetLinks
	Tx        TransactionRunner
	Publisher events.Publisher
	Now       func() time.Time
}

type batchOutcome int

const (
	outcomeUnlinked batchOutcome = iota + 1
	outcomeRemoved
	outcomeGone
)

func (s *ServiceImpl) Delete(ctx context.Context, cmd DeleteCommand) error {
	if err := requireUser(cmd.ActorID); err != nil {
		return err
	}
	if err := requireCampaignID(cmd.CampaignID); err != nil {
		return err
	}
	now := s.now()
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		stored, err := s.repository.FetchByID(txCtx, cmd.CampaignID)
		if err != nil {
			return readError(err)
		}
		if !stored.Active() {
			return notFound()
		}
		member, ok := stored.MemberOf(cmd.ActorID)
		if !ok {
			return notFound()
		}
		if !member.Owner {
			return forbidden(MsgOwnerOnlyDelete)
		}
		return markDeleting(txCtx, s.repository, s.releaser, stored, now)
	})
	if err != nil {
		return unwrapTxError(err)
	}
	return nil
}

// RequestDeletion moves a Campaign to the deleting state without an actor check. It is the
// entry point for account deletion (LGN-217) when the deleted Owner has no other GM. Already
// deleting or dead Campaigns are returned unchanged. A missing Campaign is CAMPAIGN_NOT_FOUND.
func RequestDeletion(ctx context.Context, tx TransactionRunner, repo Repository, releaser OwnershipReleaser, campaignID string, now time.Time) (Campaign, error) {
	var out Campaign
	err := tx.WithTransaction(ctx, func(txCtx context.Context) error {
		stored, err := repo.FetchByID(txCtx, campaignID)
		if err != nil {
			return readError(err)
		}
		if !stored.Active() {
			out = *stored
			return nil
		}
		if err := markDeleting(txCtx, repo, releaser, stored, now); err != nil {
			return err
		}
		out = *stored
		return nil
	})
	if err != nil {
		return Campaign{}, unwrapTxError(err)
	}
	return out, nil
}

func markDeleting(ctx context.Context, repo Repository, releaser OwnershipReleaser, c *Campaign, now time.Time) error {
	expected := c.Version
	deletingAt := now
	c.Status = StatusDeleting
	c.DeletingAt = &deletingAt
	c.DeletionAttempts = 0
	c.DeletionNextAttempt = nil
	c.DeletionLockedUntil = nil
	c.DeletionLastError = ""
	c.Version = expected + 1
	c.UpdatedAt = now
	if err := repo.MarkDeleting(ctx, c, expected); err != nil {
		return saveError(err)
	}
	if releaser != nil {
		if err := releaser.ReleaseOwnership(ctx, ownerOf(c)); err != nil {
			return internalError(MsgCampaignWrite, err)
		}
	}
	return nil
}

func ownerOf(c *Campaign) string {
	for _, m := range c.Members {
		if m.Owner {
			return m.UserID
		}
	}
	return ""
}

func (d DeletionDeps) Handle(ctx context.Context, c Campaign) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		outcome, version, err := d.runBatch(ctx, c.ID)
		if err != nil {
			return err
		}
		switch outcome {
		case outcomeUnlinked:
			continue
		case outcomeRemoved:
			d.publishDeleted(c, version)
			return nil
		default:
			return nil
		}
	}
}

func (d DeletionDeps) runBatch(ctx context.Context, campaignID string) (batchOutcome, int64, error) {
	var outcome batchOutcome
	var version int64
	err := d.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		outcome, version = outcomeGone, 0
		touched, err := d.Store.TouchDeleting(txCtx, campaignID, d.Now())
		if errors.Is(err, ErrNotDeleting) {
			return nil
		}
		if err != nil {
			return err
		}
		version = touched
		ids, err := d.Links.LinkedSheetIDs(txCtx, campaignID, DeletionUnlinkBatch)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			outcome = outcomeUnlinked
			return d.Links.UnlinkSheets(txCtx, campaignID, ids)
		}
		removed, err := d.Store.RemoveDeleting(txCtx, campaignID)
		if err != nil {
			return err
		}
		if removed {
			outcome = outcomeRemoved
		}
		return nil
	})
	if err != nil {
		return 0, 0, unwrapTxError(err)
	}
	return outcome, version, nil
}

func (d DeletionDeps) publishDeleted(c Campaign, version int64) {
	d.Publisher.Publish(events.Event{
		Type:             EventDeleted,
		AggregateType:    aggregateType,
		AggregateID:      c.ID,
		AggregateVersion: version,
		Data:             map[string]any{"template_id": c.TemplateID},
	})
}

type deletionQueueStore struct {
	store DeletionStore
}

func (s deletionQueueStore) Claim(ctx context.Context, _ string, now time.Time, lease time.Duration, limit int) ([]sweeper.Job[Campaign], error) {
	found, err := s.store.ClaimDeleting(ctx, now, lease, limit)
	if err != nil {
		return nil, err
	}
	jobs := make([]sweeper.Job[Campaign], 0, len(found))
	for _, c := range found {
		jobs = append(jobs, sweeper.Job[Campaign]{ID: c.ID, Attempts: c.DeletionAttempts, Payload: c})
	}
	return jobs, nil
}

func (s deletionQueueStore) Reschedule(ctx context.Context, id string, _, nextAt time.Time, reason string) error {
	return s.store.RescheduleDeleting(ctx, id, nextAt, reason)
}

func (s deletionQueueStore) MarkDead(ctx context.Context, id string, _ time.Time, reason string) error {
	return s.store.MarkDeletionDead(ctx, id, reason)
}

func (s deletionQueueStore) OldestPending(ctx context.Context) (time.Time, bool, error) {
	return s.store.OldestDeleting(ctx)
}

func NewDeletionQueue(deps DeletionDeps, attemptCap, batch int) (sweeper.Queue, error) {
	if deps.Store == nil || deps.Links == nil || deps.Tx == nil {
		return sweeper.Queue{}, apperr.NewError(apperr.CodeInternal, "campaign deletion dependencies are required", nil)
	}
	if deps.Publisher == nil {
		deps.Publisher = events.NoOp{}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return sweeper.NewQueue(sweeper.QueueSpec[Campaign]{
		Name:       DeletionQueueName,
		Interval:   DeletionInterval,
		AttemptCap: attemptCap,
		Batch:      batch,
		Store:      deletionQueueStore{store: deps.Store},
		Handle: func(ctx context.Context, job sweeper.Job[Campaign], _ time.Time) error {
			return deps.Handle(ctx, job.Payload)
		},
	})
}
