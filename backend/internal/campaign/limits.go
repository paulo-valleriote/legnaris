package campaign

import (
	"context"
	"errors"

	"legnaris/backend/internal/plan"
	"legnaris/backend/shared/apperr"
)

var ErrOwnershipLimitReached = errors.New("owned campaign limit reached")

const (
	MsgOwnershipLimitReached = "your plan allows no more owned campaigns"
	MsgOwnershipPlanRead     = "failed to resolve plan for owned campaigns"
	MsgOwnershipRead         = "failed to read owned campaigns"
	MsgOwnershipWrite        = "failed to update owned campaigns"
)

type PlanSource interface {
	PlanOf(ctx context.Context, ownerID string) (plan.Plan, error)
}

type OwnershipCounter interface {
	Reserve(ctx context.Context, ownerID string, limit int) error
	Release(ctx context.Context, ownerID string) error
	Count(ctx context.Context, ownerID string) (int, error)
}

type OwnershipUsage struct {
	Used  int
	Limit int
}

type OwnershipLimiter struct {
	plans    PlanSource
	counters OwnershipCounter
}

func NewOwnershipLimiter(plans PlanSource, counters OwnershipCounter) *OwnershipLimiter {
	return &OwnershipLimiter{plans: plans, counters: counters}
}

func (l *OwnershipLimiter) BeforeCreate(ctx context.Context, ownerID string) error {
	return l.ReserveOwnership(ctx, ownerID)
}

func (l *OwnershipLimiter) ReserveOwnership(ctx context.Context, userID string) error {
	if err := requireUser(userID); err != nil {
		return err
	}
	current, err := l.plans.PlanOf(ctx, userID)
	if err != nil {
		return internalError(MsgOwnershipPlanRead, err)
	}
	err = l.counters.Reserve(ctx, userID, current.OwnedCampaignLimit())
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrOwnershipLimitReached):
		return planLimitError(err)
	default:
		return internalError(MsgOwnershipWrite, err)
	}
}

func (l *OwnershipLimiter) ReleaseOwnership(ctx context.Context, userID string) error {
	if err := requireUser(userID); err != nil {
		return err
	}
	if err := l.counters.Release(ctx, userID); err != nil {
		return internalError(MsgOwnershipWrite, err)
	}
	return nil
}

func (l *OwnershipLimiter) Usage(ctx context.Context, userID string) (OwnershipUsage, error) {
	if err := requireUser(userID); err != nil {
		return OwnershipUsage{}, err
	}
	current, err := l.plans.PlanOf(ctx, userID)
	if err != nil {
		return OwnershipUsage{}, internalError(MsgOwnershipPlanRead, err)
	}
	used, err := l.counters.Count(ctx, userID)
	if err != nil {
		return OwnershipUsage{}, internalError(MsgOwnershipRead, err)
	}
	return OwnershipUsage{Used: used, Limit: current.OwnedCampaignLimit()}, nil
}

func planLimitError(cause error) *apperr.Error {
	return apperr.NewError(apperr.CodePlanLimitReached, MsgOwnershipLimitReached, cause)
}
